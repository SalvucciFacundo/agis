package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/core"
)

type mockProvider struct {
	chatFunc   func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error)
	streamFunc func(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error)
	modelsFunc func() []core.ModelInfo
}

func (m *mockProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return core.ChatResponse{Content: "mock response"}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan core.StreamEvent, 1)
	ch <- core.StreamEvent{Text: "mock stream"}
	close(ch)
	return ch, nil
}

func (m *mockProvider) Models() []core.ModelInfo {
	if m.modelsFunc != nil {
		return m.modelsFunc()
	}
	return []core.ModelInfo{{ID: "mock-model", Provider: "mock"}}
}

func TestFallbackProvider_PrimarySuccess(t *testing.T) {
	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			primaryCalled.Add(1)
			return core.ChatResponse{Content: "primary ok"}, nil
		},
	}
	fallback := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			fallbackCalled.Add(1)
			return core.ChatResponse{Content: "fallback ok"}, nil
		},
	}

	fbProvider := NewFallbackProvider(primary, fallback)
	resp, err := fbProvider.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}
	if resp.Content != "primary ok" {
		t.Errorf("Chat() content = %q, want 'primary ok'", resp.Content)
	}
	if primaryCalled.Load() != 1 {
		t.Errorf("primary called %d times, want 1", primaryCalled.Load())
	}
	if fallbackCalled.Load() != 0 {
		t.Errorf("fallback called %d times, want 0", fallbackCalled.Load())
	}
}

func TestFallbackProvider_PrimaryTransientFail_SecondarySucceeds(t *testing.T) {
	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			primaryCalled.Add(1)
			return core.ChatResponse{}, fmt.Errorf("chat completion: Service Unavailable (status 503)")
		},
	}
	fallback := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			fallbackCalled.Add(1)
			return core.ChatResponse{Content: "fallback ok"}, nil
		},
	}

	fbProvider := NewFallbackProvider(primary, fallback)
	resp, err := fbProvider.Chat(context.Background(), core.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat() error = %v, want nil", err)
	}
	if resp.Content != "fallback ok" {
		t.Errorf("Chat() content = %q, want 'fallback ok'", resp.Content)
	}
	if primaryCalled.Load() != 1 {
		t.Errorf("primary called %d times, want 1", primaryCalled.Load())
	}
	if fallbackCalled.Load() != 1 {
		t.Errorf("fallback called %d times, want 1", fallbackCalled.Load())
	}
}

func TestFallbackProvider_NonTransientFail_FastFails(t *testing.T) {
	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			primaryCalled.Add(1)
			return core.ChatResponse{}, fmt.Errorf("chat completion: Bad Request (status 400)")
		},
	}
	fallback := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			fallbackCalled.Add(1)
			return core.ChatResponse{Content: "fallback ok"}, nil
		},
	}

	fbProvider := NewFallbackProvider(primary, fallback)
	_, err := fbProvider.Chat(context.Background(), core.ChatRequest{})
	if err == nil {
		t.Fatal("Chat() error = nil, want 400 error")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("Chat() error = %v, want to contain status 400", err)
	}
	if primaryCalled.Load() != 1 {
		t.Errorf("primary called %d times, want 1", primaryCalled.Load())
	}
	if fallbackCalled.Load() != 0 {
		t.Errorf("fallback called %d times, want 0 (fast fail on non-transient)", fallbackCalled.Load())
	}
}

func TestFallbackProvider_ContextCanceled_FastFails(t *testing.T) {
	var primaryCalled, fallbackCalled atomic.Int32

	primary := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			primaryCalled.Add(1)
			return core.ChatResponse{}, context.Canceled
		},
	}
	fallback := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			fallbackCalled.Add(1)
			return core.ChatResponse{Content: "fallback ok"}, nil
		},
	}

	fbProvider := NewFallbackProvider(primary, fallback)
	_, err := fbProvider.Chat(context.Background(), core.ChatRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Chat() error = %v, want context.Canceled", err)
	}
	if fallbackCalled.Load() != 0 {
		t.Errorf("fallback called %d times, want 0", fallbackCalled.Load())
	}
}

func TestFallbackProvider_AllProvidersFail(t *testing.T) {
	primary := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			return core.ChatResponse{}, fmt.Errorf("status 503")
		},
	}
	fallback := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			return core.ChatResponse{}, fmt.Errorf("status 500")
		},
	}

	fbProvider := NewFallbackProvider(primary, fallback)
	_, err := fbProvider.Chat(context.Background(), core.ChatRequest{})
	if err == nil {
		t.Fatal("Chat() error = nil, want all providers failed error")
	}
	if !strings.Contains(err.Error(), "all LLM providers failed") {
		t.Errorf("Chat() error = %v, want to contain 'all LLM providers failed'", err)
	}
}

func TestFallbackProvider_Models(t *testing.T) {
	primary := &mockProvider{
		modelsFunc: func() []core.ModelInfo {
			return []core.ModelInfo{{ID: "gpt-4o", Provider: "openai"}}
		},
	}
	fallback := &mockProvider{
		modelsFunc: func() []core.ModelInfo {
			return []core.ModelInfo{{ID: "claude-3-5-sonnet", Provider: "openrouter"}}
		},
	}

	fbProvider := NewFallbackProvider(primary, fallback)
	models := fbProvider.Models()
	if len(models) != 2 {
		t.Fatalf("len(Models()) = %d, want 2", len(models))
	}
	if models[0].ID != "gpt-4o" || models[1].ID != "claude-3-5-sonnet" {
		t.Errorf("Models() = %+v, want [gpt-4o, claude-3-5-sonnet]", models)
	}
}

func TestFallbackProvider_SingleProviderDirect(t *testing.T) {
	primary := &mockProvider{
		chatFunc: func(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
			return core.ChatResponse{Content: "solo"}, nil
		},
		modelsFunc: func() []core.ModelInfo {
			return []core.ModelInfo{{ID: "llama3.2", Provider: "ollama"}}
		},
	}

	fbProvider := NewFallbackProvider(primary)
	resp, err := fbProvider.Chat(context.Background(), core.ChatRequest{})
	if err != nil || resp.Content != "solo" {
		t.Errorf("Chat() = (%+v, %v), want ('solo', nil)", resp, err)
	}
	models := fbProvider.Models()
	if len(models) != 1 || models[0].ID != "llama3.2" {
		t.Errorf("Models() = %+v, want 1 model llama3.2", models)
	}
}
