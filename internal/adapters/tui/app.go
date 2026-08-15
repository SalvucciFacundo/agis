// Package tui provides the Bubbletea terminal UI for AGIS.
//
// The TUI renders the conversation in a scrollable viewport, a text input for
// the user's next message, and a spinner while the assistant streams. On
// startup it restores the latest conversation from the repository; Enter
// submits the input to Brain.Step and paints streamed tokens into the viewport
// as they arrive.
package tui

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// Viewport line prefixes for the message kinds the TUI renders.
const (
	userPrefix      = "you: "
	assistantPrefix = "assistant: "
	errorPrefix     = "error: "
)

// Default terminal dimensions used until the first WindowSizeMsg arrives.
const (
	defaultWidth  = 80
	defaultHeight = 24
	// reserveHeight is the number of rows below the viewport: one for the
	// status/spinner line and one for the input.
	reserveHeight = 2
)

// Model is the Bubbletea TUI. It owns a Brain and a Repository; the stream
// channel carries assistant tokens from the Brain's sink into the viewport.
type Model struct {
	brain *core.Brain
	repo  core.Repository
	ctx   context.Context

	// stream receives assistant tokens from the Brain's sink. The submit
	// goroutine closes it when Step returns, which is the update loop's
	// "streaming finished" signal.
	stream chan string
	// errCh carries Step's result from the submit goroutine to the update
	// loop. Buffered so the goroutine never blocks after Step returns.
	errCh chan error

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	// history holds completed conversation lines; current accumulates the
	// in-flight assistant reply as it streams.
	history   strings.Builder
	current   strings.Builder
	streaming bool

	width  int
	height int
}

// tokenMsg carries one streamed assistant token into the update loop.
type tokenMsg string

// streamDoneMsg signals that Brain.Step finished, successfully or not.
type streamDoneMsg struct {
	err error
}

// historyMsg carries the restored conversation (or a load error) from Init.
type historyMsg struct {
	msgs []core.Message
	err  error
}

var _ tea.Model = (*Model)(nil)

// New returns a Bubbletea Model wired to brain and repo. stream is the channel
// the Brain's sink writes tokens into (see core.WithSink); the model drains it
// to paint tokens in real time and closes it when a step finishes.
func New(brain *core.Brain, repo core.Repository, stream chan string) *Model {
	input := textinput.New()
	input.Placeholder = "Type a message and press Enter"
	input.Width = defaultWidth
	// Focus and drop the blink command: a static, always-visible cursor is
	// enough in M1 and keeps the cursor timer out of tests.
	input.Focus()

	sp := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("5"))),
	)

	return &Model{
		brain:    brain,
		repo:     repo,
		ctx:      context.Background(),
		stream:   stream,
		errCh:    make(chan error, 1),
		viewport: viewport.New(defaultWidth, defaultHeight-reserveHeight),
		input:    input,
		spinner:  sp,
		width:    defaultWidth,
		height:   defaultHeight,
	}
}

// Init restores the latest conversation and starts the spinner animation.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadHistory(), m.spinner.Tick)
}

// Update handles window resizes, keys, and the stream/restore messages. It
// delegates everything else to the input and spinner widgets.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - reserveHeight
		m.input.Width = msg.Width
		return m, nil

	case historyMsg:
		if msg.err != nil {
			m.appendError(msg.err)
			return m, nil
		}
		m.restoreHistory(msg.msgs)
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.streaming {
				return m, nil
			}
			return m.submit()
		}

	case tokenMsg:
		m.current.WriteString(string(msg))
		m.refresh()
		return m, m.waitToken()

	case streamDoneMsg:
		return m.finishStream(msg)
	}

	// Delegate everything else to the input and spinner widgets.
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.spinner, cmd = m.spinner.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// View renders the conversation viewport, the spinner/status line, and the
// text input.
func (m Model) View() string {
	status := ""
	if m.streaming {
		status = m.spinner.View() + " thinking..."
	}
	return strings.Join([]string{m.viewport.View(), status, m.input.View()}, "\n")
}

// submit echoes the user's message, marks the model as streaming, and starts a
// goroutine that runs Brain.Step and closes the stream channel on completion.
// It returns the token-reader command that the update loop uses to paint
// streamed tokens.
func (m *Model) submit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.input.Value())
	m.input.SetValue("")
	if input == "" {
		return m, nil
	}

	m.streaming = true
	m.current.Reset()
	m.current.WriteString(assistantPrefix)
	m.history.WriteString(userPrefix)
	m.history.WriteString(input)
	m.history.WriteString("\n")
	m.refresh()

	go func() {
		m.errCh <- m.brain.Step(m.ctx, input)
		close(m.stream)
	}()

	return m, m.waitToken()
}

// waitToken reads the next token from the stream channel. A closed channel
// means Step finished, so it returns the step result.
func (m *Model) waitToken() tea.Cmd {
	return func() tea.Msg {
		text, ok := <-m.stream
		if !ok {
			return streamDoneMsg{err: <-m.errCh}
		}
		return tokenMsg(text)
	}
}

// finishStream commits the in-flight assistant reply (full or partial) and
// reports a step error, if any.
func (m *Model) finishStream(msg streamDoneMsg) (tea.Model, tea.Cmd) {
	m.streaming = false
	if m.current.Len() > len(assistantPrefix) {
		m.history.WriteString(m.current.String())
		m.history.WriteString("\n")
	}
	m.current.Reset()
	if msg.err != nil {
		m.appendError(msg.err)
		return m, nil
	}
	m.refresh()
	return m, nil
}

// loadHistory fetches the latest conversation and returns it as a historyMsg.
func (m *Model) loadHistory() tea.Cmd {
	return func() tea.Msg {
		conv, err := m.repo.LatestConversation(m.ctx)
		if errors.Is(err, core.ErrNotFound) {
			return historyMsg{}
		}
		if err != nil {
			return historyMsg{err: err}
		}
		msgs, err := m.repo.Messages(m.ctx, conv.ID, 0)
		if err != nil {
			return historyMsg{err: err}
		}
		return historyMsg{msgs: msgs}
	}
}

// restoreHistory replaces the viewport content with the given messages.
func (m *Model) restoreHistory(msgs []core.Message) {
	m.history.Reset()
	for _, msg := range msgs {
		m.history.WriteString(formatMessage(msg))
		m.history.WriteString("\n")
	}
	m.refresh()
}

// appendError writes an error line to the viewport.
func (m *Model) appendError(err error) {
	m.history.WriteString(errorPrefix)
	m.history.WriteString(err.Error())
	m.history.WriteString("\n")
	m.refresh()
}

// refresh pushes the current history plus in-flight reply into the viewport
// and scrolls to the bottom.
func (m *Model) refresh() {
	m.viewport.SetContent(m.history.String() + m.current.String())
	m.viewport.GotoBottom()
}

// formatMessage renders one persisted message as a viewport line.
func formatMessage(msg core.Message) string {
	switch msg.Role {
	case core.RoleUser:
		return userPrefix + msg.Content
	case core.RoleAssistant:
		return assistantPrefix + msg.Content
	default:
		return string(msg.Role) + ": " + msg.Content
	}
}
