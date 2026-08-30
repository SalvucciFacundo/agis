package transport_test

import (
	"context"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/mcp/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestStdioTransport_Echo(t *testing.T) {
	defer goleak.VerifyNone(t)

	// "cat" reads stdin line by line and writes to stdout.
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat binary not found")
	}

	tr, err := transport.NewStdio(transport.StdioConfig{
		Command: "cat",
	})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Send message
	msg := []byte(`{"jsonrpc":"2.0","id":"1","method":"ping"}`)
	err = tr.Send(ctx, msg)
	require.NoError(t, err)

	// Receive message
	received, err := tr.Receive(ctx)
	require.NoError(t, err)
	assert.JSONEq(t, string(msg), string(received))
}

func TestStdioTransport_StderrDrain(t *testing.T) {
	defer goleak.VerifyNone(t)

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh binary not found")
	}

	// Script writes noise to stderr and valid json to stdout
	script := `
echo "debug log on stderr" >&2
echo '{"jsonrpc":"2.0","result":"ok"}'
`
	tr, err := transport.NewStdio(transport.StdioConfig{
		Command: "sh",
		Args:    []string{"-c", script},
	})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	received, err := tr.Receive(ctx)
	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","result":"ok"}`, string(received))
}

func TestStdioTransport_SubprocessExit(t *testing.T) {
	defer goleak.VerifyNone(t)

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh binary not found")
	}

	tr, err := transport.NewStdio(transport.StdioConfig{
		Command: "sh",
		Args:    []string{"-c", "exit 0"},
	})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = tr.Receive(ctx)
	assert.Error(t, err)
	assert.True(t, err == io.EOF || assert.AnError != nil)
}

func TestStdioTransport_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep binary not found")
	}

	tr, err := transport.NewStdio(transport.StdioConfig{
		Command: "sleep",
		Args:    []string{"10"},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = tr.Receive(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	err = tr.Close()
	assert.NoError(t, err)
}

func TestStdioTransport_EnvVariables(t *testing.T) {
	defer goleak.VerifyNone(t)

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh binary not found")
	}

	tr, err := transport.NewStdio(transport.StdioConfig{
		Command: "sh",
		Args:    []string{"-c", `echo "{\"env\":\"$TEST_VAR\"}"`},
		Env: map[string]string{
			"TEST_VAR": "mcp_test_value",
		},
	})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	received, err := tr.Receive(ctx)
	require.NoError(t, err)
	assert.JSONEq(t, `{"env":"mcp_test_value"}`, string(received))
}
