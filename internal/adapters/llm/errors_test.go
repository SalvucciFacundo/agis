package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
)

type customTimeoutError struct{}

func (e *customTimeoutError) Error() string   { return "custom network timeout" }
func (e *customTimeoutError) Timeout() bool   { return true }
func (e *customTimeoutError) Temporary() bool { return true }

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "nil error",
			err:       nil,
			transient: false,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			transient: false,
		},
		{
			name:      "wrapped context canceled",
			err:       fmt.Errorf("posting chat: %w", context.Canceled),
			transient: false,
		},
		{
			name:      "HTTP 429 rate limit",
			err:       fmt.Errorf("chat completion: Too Many Requests (status 429)"),
			transient: true,
		},
		{
			name:      "HTTP 429 in apiError",
			err:       errors.New("rate_limit_exceeded: Rate limit reached"),
			transient: true,
		},
		{
			name:      "HTTP 500 Internal Server Error",
			err:       fmt.Errorf("chat completion: Internal Server Error (status 500)"),
			transient: true,
		},
		{
			name:      "HTTP 502 Bad Gateway",
			err:       fmt.Errorf("chat completion: Bad Gateway (status 502)"),
			transient: true,
		},
		{
			name:      "HTTP 503 Service Unavailable",
			err:       fmt.Errorf("chat completion: Service Unavailable (status 503)"),
			transient: true,
		},
		{
			name:      "HTTP 504 Gateway Timeout",
			err:       fmt.Errorf("chat completion: Gateway Timeout (status 504)"),
			transient: true,
		},
		{
			name:      "network timeout implementing net.Error",
			err:       &customTimeoutError{},
			transient: true,
		},
		{
			name:      "wrapped net timeout error",
			err:       fmt.Errorf("request failed: %w", &customTimeoutError{}),
			transient: true,
		},
		{
			name:      "connection refused syscall error",
			err:       &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			transient: true,
		},
		{
			name:      "connection reset syscall error",
			err:       &net.OpError{Op: "read", Err: syscall.ECONNRESET},
			transient: true,
		},
		{
			name:      "EOF error",
			err:       io.EOF,
			transient: true,
		},
		{
			name:      "unexpected EOF",
			err:       io.ErrUnexpectedEOF,
			transient: true,
		},
		{
			name:      "HTTP 400 Bad Request",
			err:       fmt.Errorf("chat completion: Bad Request (status 400)"),
			transient: false,
		},
		{
			name:      "HTTP 401 Unauthorized",
			err:       fmt.Errorf("chat completion: Unauthorized (status 401)"),
			transient: false,
		},
		{
			name:      "HTTP 403 Forbidden",
			err:       fmt.Errorf("chat completion: Forbidden (status 403)"),
			transient: false,
		},
		{
			name:      "HTTP 404 Model Not Found",
			err:       fmt.Errorf("chat completion: Not Found (status 404)"),
			transient: false,
		},
		{
			name:      "Invalid schema / generic bad request",
			err:       errors.New("invalid_request_error: unrecognized parameter 'foo'"),
			transient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientError(tt.err)
			if got != tt.transient {
				t.Errorf("isTransientError(%v) = %v, want %v", tt.err, got, tt.transient)
			}
		})
	}
}
