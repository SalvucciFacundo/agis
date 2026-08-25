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
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/persona"
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

	// defaultCloseTimeout bounds the synchronous CloseSession call on quit.
	defaultCloseTimeout = 30 * time.Second

	// Status-line texts for the quit paths.
	cancellingStatus = "cancelling..."
	closingStatus    = "closing..."
)

// Option configures a Model.
type Option func(*Model)

// WithCloseTimeout sets how long CtrlC waits for CloseSession before giving
// up. A non-positive value keeps the default of 30s.
func WithCloseTimeout(d time.Duration) Option {
	return func(m *Model) {
		if d > 0 {
			m.closeTimeout = d
		}
	}
}

// WithOverlays wires the /personality resolver.
func WithOverlays(o *persona.Overlays) Option {
	return func(m *Model) { m.overlays = o }
}

// WithEvolution wires the /persona command target.
func WithEvolution(e *persona.Evolution) Option {
	return func(m *Model) { m.evolution = e }
}

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

	// cancel aborts the in-flight Step; it is set by submit and nil between
	// turns. cancelled records that a streaming quit was requested, so the
	// next quit press force-quits instead of waiting for the drain.
	cancel    context.CancelFunc
	cancelled bool

	// closeTimeout bounds the synchronous CloseSession call on quit;
	// closing marks a graceful close as scheduled so a second quit press
	// force-quits, and status overrides the spinner line (closing/cancelling).
	closeTimeout time.Duration
	closing      bool
	status       string

	// overlays resolves /personality names; evolution backs /persona
	// commands. Both may be nil, which disables the corresponding commands.
	overlays    *persona.Overlays
	evolution   *persona.Evolution
	personality string // active overlay name for status display
}

// tokenMsg carries one streamed assistant token into the update loop.
type tokenMsg string

// streamDoneMsg signals that Brain.Step finished, successfully or not.
type streamDoneMsg struct {
	err error
}

// closedMsg signals that CloseSession finished and the program may quit.
type closedMsg struct{}

// historyMsg carries the restored conversation (or a load error) from Init.
type historyMsg struct {
	msgs []core.Message
	err  error
}

var _ tea.Model = (*Model)(nil)

// New returns a Bubbletea Model wired to brain and repo. stream is the channel
// the Brain's sink writes tokens into (see core.WithSink); the model drains it
// to paint tokens in real time and closes it when a step finishes.
func New(brain *core.Brain, repo core.Repository, stream chan string, opts ...Option) *Model {
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

	m := &Model{
		brain:        brain,
		repo:         repo,
		ctx:          context.Background(),
		stream:       stream,
		errCh:        make(chan error, 1),
		viewport:     viewport.New(defaultWidth, defaultHeight-reserveHeight),
		input:        input,
		spinner:      sp,
		width:        defaultWidth,
		height:       defaultHeight,
		closeTimeout: defaultCloseTimeout,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
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
			return m.handleQuit()
		case tea.KeyEnter:
			if m.streaming || m.closing {
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

	case closedMsg:
		return m, tea.Quit
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
	status := m.status
	if status == "" && m.streaming {
		status = m.spinner.View() + " thinking..."
	}
	return strings.Join([]string{m.viewport.View(), status, m.input.View()}, "\n")
}

// handleQuit implements the TUI-001 quit contract. While streaming, the first
// press cancels the stream so the partial reply drains; the second force-quits.
// When idle, the first press runs a bounded CloseSession and then quits; the
// second force-quits without waiting for the close to finish.
func (m *Model) handleQuit() (tea.Model, tea.Cmd) {
	if m.streaming {
		if !m.cancelled && m.cancel != nil {
			m.cancelled = true
			m.cancel()
			m.status = cancellingStatus
			return m, nil
		}
		return m, tea.Quit
	}
	if m.closing {
		return m, tea.Quit
	}
	m.closing = true
	m.status = closingStatus
	return m, m.closeSession()
}

// closeSession runs Brain.CloseSession bounded by the close timeout on its own
// context. CloseSession is non-fatal by contract: it logs its own failures and
// always returns nil, so the program quits either way.
func (m *Model) closeSession() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.closeTimeout)
		defer cancel()
		_ = m.brain.CloseSession(ctx)
		return closedMsg{}
	}
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
	if strings.HasPrefix(input, "/") {
		return m.runCommand(input)
	}

	m.streaming = true
	m.current.Reset()
	m.current.WriteString(assistantPrefix)
	m.history.WriteString(userPrefix)
	m.history.WriteString(input)
	m.history.WriteString("\n")
	m.refresh()

	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.cancelled = false

	go func() {
		m.errCh <- m.brain.Step(ctx, input)
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
// reports a step error, if any. It also clears the cancelling state left by a
// streaming quit request.
func (m *Model) finishStream(msg streamDoneMsg) (tea.Model, tea.Cmd) {
	m.streaming = false
	if m.cancelled {
		m.cancelled = false
		m.status = ""
	}
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

// commandFeedbackPrefix marks local command output lines in the viewport.
const commandFeedbackPrefix = "· "

// runCommand handles slash commands locally: they never reach the provider
// and never persist as conversation messages (spec TUI-001).
func (m *Model) runCommand(input string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(input)
	switch fields[0] {
	case "/personality":
		return m.cmdPersonality(fields[1:])
	case "/persona":
		return m.cmdPersona(fields[1:])
	default:
		return m.feedback("unknown command: " + fields[0]), nil
	}
}

// feedback appends a prefixed line to the viewport.
func (m *Model) feedback(line string) *Model {
	m.history.WriteString(commandFeedbackPrefix + line + "\n")
	m.refresh()
	return m
}

// cmdPersonality applies or clears the session overlay.
func (m *Model) cmdPersonality(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		current := m.personality
		if current == "" {
			current = "none"
		}
		names := ""
		if m.overlays != nil {
			names = strings.Join(m.overlays.Names(), ", ")
		}
		return m.feedbackEcho(fmt.Sprintf("personality: %s (available: %s)", current, names))
	}

	name := strings.ToLower(strings.TrimSpace(args[0]))
	if m.overlays == nil {
		return m.feedbackEcho("personalities are not wired")
	}
	text, err := m.overlays.Resolve(name)
	if err != nil {
		return m.feedbackEcho(fmt.Sprintf("unknown personality %q", name))
	}
	m.brain.SetOverlay(text)
	m.personality = name
	if text == "" {
		m.personality = ""
		return m.feedbackEcho("personality cleared")
	}
	return m.feedbackEcho("personality: " + name)
}

// cmdPersona drives the evolution layer commands.
func (m *Model) cmdPersona(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m.feedbackEcho("usage: /persona freeze|reset|status")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultCloseTimeout)
	defer cancel()

	switch args[0] {
	case "freeze":
		if m.evolution == nil {
			return m.feedbackEcho("evolution is not wired")
		}
		m.evolution.Freeze()
		return m.feedbackEcho("persona evolution frozen for this session")
	case "reset":
		if m.evolution == nil {
			return m.feedbackEcho("evolution is not wired")
		}
		if err := m.evolution.Reset(ctx); err != nil {
			return m.feedbackEcho("persona reset failed: " + err.Error())
		}
		return m.feedbackEcho("persona evolution reset to seed state")
	case "status":
		st, err := m.evolutionStatus(ctx)
		if err != nil {
			return m.feedbackEcho("persona status failed: " + err.Error())
		}
		mode := "active"
		switch {
		case m.evolution == nil:
			mode = "not wired"
		case st.Frozen:
			mode = "frozen"
		case !st.Active:
			mode = "learning (no rows yet)"
		}
		return m.feedbackEcho(fmt.Sprintf(
			"persona: evolution %s (%d rows) · personality %s",
			mode, st.Rows, m.personalityOrNone()))
	default:
		return m.feedbackEcho(fmt.Sprintf("unknown /persona command %q", args[0]))
	}
}

func (m *Model) evolutionStatus(ctx context.Context) (persona.Status, error) {
	if m.evolution == nil {
		return persona.Status{}, nil
	}
	return m.evolution.Status(ctx)
}

func (m *Model) personalityOrNone() string {
	if m.personality == "" {
		return "none"
	}
	return m.personality
}

// feedbackEcho writes the feedback line and returns the model with no
// follow-up command.
func (m *Model) feedbackEcho(line string) (tea.Model, tea.Cmd) {
	return m.feedback(line), nil
}
