// Package transport defines communication layers for the Model Context Protocol (stdio, sse).
package transport

import "context"

// Transport defines the bidirectional wire transport for JSON-RPC 2.0 messages.
type Transport interface {
	// Send writes a complete JSON-RPC message payload to the transport.
	Send(ctx context.Context, msg []byte) error

	// Receive reads the next complete JSON-RPC message payload from the transport.
	// Returns io.EOF or descriptive error on disconnection.
	Receive(ctx context.Context) ([]byte, error)

	// Close terminates the transport, underlying subprocess or network stream, and releases resources.
	Close() error
}
