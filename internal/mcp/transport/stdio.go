package transport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"
)

// StdioConfig defines configuration for spawning a subprocess MCP server.
type StdioConfig struct {
	Command string
	Args    []string
	Env     map[string]string
	Dir     string
	Logger  *slog.Logger
}

type readResult struct {
	data []byte
	err  error
}

// StdioTransport manages communication with an MCP server subprocess via stdin/stdout.
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	logger    *slog.Logger

	incoming  chan readResult
	closed    chan struct{}
	closeOnce sync.Once
	closeErr  error

	sendMu sync.Mutex
	wg     sync.WaitGroup
}

// NewStdio creates and spawns a new StdioTransport subprocess.
func NewStdio(cfg StdioConfig) (*StdioTransport, error) {
	if cfg.Command == "" {
		return nil, errors.New("mcp stdio transport: command is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Dir = cfg.Dir
	cmd.WaitDelay = 100 * time.Millisecond
	setSysProcAttr(cmd)

	if len(cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("starting subprocess %q: %w", cfg.Command, err)
	}

	t := &StdioTransport{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		logger:   logger,
		incoming: make(chan readResult, 16),
		closed:   make(chan struct{}),
	}

	t.wg.Add(2)
	go t.readStdoutLoop()
	go t.drainStderrLoop()

	return t, nil
}

func (t *StdioTransport) readStdoutLoop() {
	defer t.wg.Done()
	reader := bufio.NewReader(t.stdout)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				dataCopy := make([]byte, len(trimmed))
				copy(dataCopy, trimmed)
				select {
				case t.incoming <- readResult{data: dataCopy}:
				case <-t.closed:
					return
				}
			}
		}

		if err != nil {
			select {
			case t.incoming <- readResult{err: err}:
			case <-t.closed:
			}
			return
		}
	}
}

func (t *StdioTransport) drainStderrLoop() {
	defer t.wg.Done()
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			t.logger.Debug("mcp stderr", "msg", text)
		}
	}
}

// Send transmits a message payload followed by a newline to the subprocess stdin.
func (t *StdioTransport) Send(ctx context.Context, msg []byte) error {
	select {
	case <-t.closed:
		return errors.New("mcp stdio transport: closed")
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	t.sendMu.Lock()
	defer t.sendMu.Unlock()

	select {
	case <-t.closed:
		return errors.New("mcp stdio transport: closed")
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	payload := append(msg, '\n')
	_, err := t.stdin.Write(payload)
	if err != nil {
		return fmt.Errorf("writing to mcp stdin: %w", err)
	}
	return nil
}

// Receive reads the next message payload from the subprocess stdout.
func (t *StdioTransport) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-t.closed:
		return nil, errors.New("mcp stdio transport: closed")
	case res, ok := <-t.incoming:
		if !ok {
			return nil, io.EOF
		}
		if res.err != nil {
			return nil, res.err
		}
		return res.data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close gracefully terminates the subprocess and closes all streams.
func (t *StdioTransport) Close() error {
	t.closeOnce.Do(func() {
		close(t.closed)

		_ = t.stdin.Close()
		_ = t.stdout.Close()
		_ = t.stderr.Close()

		if t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}

		_ = t.cmd.Wait()
		t.wg.Wait()
	})
	return t.closeErr
}
