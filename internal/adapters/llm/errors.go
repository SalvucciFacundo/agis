package llm

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
)

// isTransientError returns true if err represents a transient, retryable failure
// such as HTTP 429, 500, 502, 503, 504, network timeouts, connection resets, or EOF.
// Non-transient errors (400, 401, 403, 404, context cancellation) return false.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Explicit context cancellation must fast-fail.
	if errors.Is(err, context.Canceled) {
		return false
	}

	// Check for net.Error timeout.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for network op errors / connection refused / reset.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) ||
			errors.Is(opErr.Err, syscall.ECONNRESET) ||
			errors.Is(opErr.Err, syscall.EPIPE) {
			return true
		}
	}

	// EOF / unexpected EOF during read.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	errStr := strings.ToLower(err.Error())

	// Check fatal status codes and invalid request markers first.
	if strings.Contains(errStr, "status 400") ||
		strings.Contains(errStr, "status 401") ||
		strings.Contains(errStr, "status 403") ||
		strings.Contains(errStr, "status 404") ||
		strings.Contains(errStr, "invalid_request_error") {
		return false
	}

	// Check transient status codes and error markers.
	if strings.Contains(errStr, "status 429") ||
		strings.Contains(errStr, "status 500") ||
		strings.Contains(errStr, "status 502") ||
		strings.Contains(errStr, "status 503") ||
		strings.Contains(errStr, "status 504") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "rate_limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "gateway timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") {
		return true
	}

	return false
}
