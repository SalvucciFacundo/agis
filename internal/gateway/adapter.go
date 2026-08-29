// Package gateway provides external chat platform adapters (Telegram, Discord),
// a multiplexer for routing messages between chat platforms and the AGIS brain,
// and sandbox security guardrails for non-interactive execution.
package gateway

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrUnauthorized is returned or logged when an incoming message from a non-allowlisted
	// user is received.
	ErrUnauthorized = errors.New("unauthorized user")

	// ErrAdapterNotFound is returned when an outbound message specifies an unknown adapter.
	ErrAdapterNotFound = errors.New("adapter not found")

	// ErrAdapterClosed is returned when an operation is attempted on a closed adapter.
	ErrAdapterClosed = errors.New("adapter closed")
)

// MessageEvent represents an inbound chat event normalized across platforms.
type MessageEvent struct {
	Adapter   string
	UserID    string
	ChatID    string
	Content   string
	Timestamp time.Time
}

// Handler handles normalized inbound message events.
type Handler func(ctx context.Context, event MessageEvent) error

// Adapter defines the contract for external chat platform integrations.
type Adapter interface {
	// Name returns the unique identifier for this adapter (e.g. "telegram", "discord").
	Name() string

	// Start connects to the upstream platform and begins listening for incoming messages.
	Start(ctx context.Context) error

	// Stop gracefully shuts down platform listeners and drains inflight operations.
	Stop() error

	// Send transmits an outbound message to a target channel or chat ID.
	Send(ctx context.Context, target string, msg string) error
}

// IsAllowed reports whether userID is present in the configured allowlist.
// If allowlist is empty or nil, it returns false (fail closed).
func IsAllowed(allowlist []string, userID string) bool {
	cleanUser := strings.TrimSpace(userID)
	if cleanUser == "" {
		return false
	}
	for _, id := range allowlist {
		if strings.TrimSpace(id) == cleanUser {
			return true
		}
	}
	return false
}
