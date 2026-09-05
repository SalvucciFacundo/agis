package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// FallbackProvider is a composite core.Provider that automatically fails over across
// an ordered chain of LLM providers upon encountering transient errors.
type FallbackProvider struct {
	providers []core.Provider
}

// NewFallbackProvider creates a FallbackProvider with primary as index 0 and fallbacks in order.
func NewFallbackProvider(primary core.Provider, fallbacks ...core.Provider) *FallbackProvider {
	var list []core.Provider
	if primary != nil {
		list = append(list, primary)
	}
	for _, fb := range fallbacks {
		if fb != nil {
			list = append(list, fb)
		}
	}
	return &FallbackProvider{
		providers: list,
	}
}

// Models returns the available models from the primary provider and all configured fallback providers.
func (f *FallbackProvider) Models() []core.ModelInfo {
	if len(f.providers) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	var models []core.ModelInfo

	for _, p := range f.providers {
		for _, m := range p.Models() {
			key := m.Provider + "/" + m.ID
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				models = append(models, m)
			}
		}
	}
	return models
}

// Chat executes a non-streaming chat completion across the provider chain.
// If a transient error occurs, it advances to the next provider in the chain.
// Non-transient errors (such as HTTP 400 or context.Canceled) fail fast immediately.
func (f *FallbackProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	if len(f.providers) == 0 {
		return core.ChatResponse{}, errors.New("no LLM providers configured")
	}

	var recordedErrors []string

	for i, p := range f.providers {
		if ctx.Err() != nil {
			return core.ChatResponse{}, ctx.Err()
		}

		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}

		// Non-transient errors fail immediately without attempting secondary providers.
		if !isTransientError(err) {
			return core.ChatResponse{}, err
		}

		recordedErrors = append(recordedErrors, fmt.Sprintf("provider %d: %v", i, err))
	}

	return core.ChatResponse{}, fmt.Errorf("all LLM providers failed: %s", strings.Join(recordedErrors, ", "))
}

// Stream executes a streaming chat completion with pre-token failover semantics.
// If an error occurs before any tokens are emitted, failover seamlessly transitions to the next provider.
// If an error occurs after >= 1 token has been emitted, the stream terminates with an error event to prevent corrupted completions.
func (f *FallbackProvider) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	if len(f.providers) == 0 {
		return nil, errors.New("no LLM providers configured")
	}

	out := make(chan core.StreamEvent)

	go func() {
		defer close(out)

		var recordedErrors []string

		for i, p := range f.providers {
			if ctx.Err() != nil {
				out <- core.StreamEvent{Err: ctx.Err()}
				return
			}

			pCtx, pCancel := context.WithCancel(ctx)
			streamCh, err := p.Stream(pCtx, req)
			if err != nil {
				pCancel()
				if !isTransientError(err) {
					out <- core.StreamEvent{Err: err}
					return
				}
				recordedErrors = append(recordedErrors, fmt.Sprintf("provider %d: %v", i, err))
				continue
			}

			tokensEmitted := false
			preTokenFailed := false
			var preTokenErr error

			for ev := range streamCh {
				if ev.Err != nil {
					if !tokensEmitted {
						// Pre-token failure: check if transient
						if isTransientError(ev.Err) {
							preTokenFailed = true
							preTokenErr = ev.Err
							pCancel()
							// Drain streamCh to allow provider goroutine cleanup
							for range streamCh {
							}
							break
						}
						// Non-transient pre-token error: emit and terminate
						pCancel()
						out <- ev
						return
					}

					// Mid-stream failure: emit error event and terminate cleanly
					pCancel()
					out <- ev
					return
				}

				if ev.Text != "" || ev.ToolCall != nil {
					tokensEmitted = true
				}

				select {
				case out <- ev:
				case <-ctx.Done():
					pCancel()
					return
				}
			}

			pCancel()

			if preTokenFailed {
				recordedErrors = append(recordedErrors, fmt.Sprintf("provider %d: %v", i, preTokenErr))
				continue
			}

			// If tokens were emitted or stream finished normally, we are done
			if tokensEmitted {
				return
			}
			return
		}

		// All providers failed pre-token
		out <- core.StreamEvent{
			Err: fmt.Errorf("all LLM providers failed: %s", strings.Join(recordedErrors, ", ")),
		}
	}()

	return out, nil
}
