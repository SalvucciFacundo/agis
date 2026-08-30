package transport_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/mcp/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSSETransport_HandshakeAndEcho(t *testing.T) {
	defer goleak.VerifyNone(t)

	msgChan := make(chan string, 10)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}

			// 1. Send endpoint event
			fmt.Fprintf(w, "event: endpoint\ndata: /messages?session_id=test-123\n\n")
			flusher.Flush()

			// 2. Stream message events received from POST
			for {
				select {
				case <-r.Context().Done():
					return
				case msg, ok := <-msgChan:
					if !ok {
						return
					}
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
					flusher.Flush()
				}
			}

		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			msgChan <- string(body)
			w.WriteHeader(http.StatusAccepted)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer func() {
		close(msgChan)
		server.Close()
	}()

	tr, err := transport.NewSSE(transport.SSEConfig{
		URL: server.URL,
	})
	require.NoError(t, err)
	defer tr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Send message
	reqMsg := []byte(`{"jsonrpc":"2.0","id":"1","method":"ping"}`)
	err = tr.Send(ctx, reqMsg)
	require.NoError(t, err)

	// Receive message
	respMsg, err := tr.Receive(ctx)
	require.NoError(t, err)
	assert.JSONEq(t, string(reqMsg), string(respMsg))
}

func TestSSETransport_ConnectionFailure(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Attempt connecting to non-existent server
	tr, err := transport.NewSSE(transport.SSEConfig{
		URL: "http://127.0.0.1:54321/nonexistent",
	})
	if err == nil {
		defer tr.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, err = tr.Receive(ctx)
		assert.Error(t, err)
	} else {
		assert.Error(t, err)
	}
}

func TestSSETransport_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	tr, err := transport.NewSSE(transport.SSEConfig{
		URL: server.URL,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = tr.Receive(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	err = tr.Close()
	assert.NoError(t, err)
}
