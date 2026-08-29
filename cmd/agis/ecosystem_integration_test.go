package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
	"github.com/SalvucciFacundo/agis/internal/cron"
	"github.com/SalvucciFacundo/agis/internal/gateway"
	"github.com/SalvucciFacundo/agis/internal/memory"
	"github.com/SalvucciFacundo/agis/internal/plugins"
	"github.com/SalvucciFacundo/agis/internal/policy"
	"github.com/SalvucciFacundo/agis/internal/webhook"
	"go.uber.org/goleak"
)

type recordAdapter struct {
	name      string
	mu        sync.Mutex
	started   bool
	stopped   bool
	sentMsgs  []struct{ Target, Msg string }
	allowlist []string
}

func newRecordAdapter(name string, allowlist []string) *recordAdapter {
	return &recordAdapter{
		name:      name,
		allowlist: allowlist,
	}
}

func (a *recordAdapter) Name() string { return a.name }

func (a *recordAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	a.started = true
	a.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (a *recordAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopped = true
	return nil
}

func (a *recordAdapter) Send(ctx context.Context, target string, msg string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sentMsgs = append(a.sentMsgs, struct{ Target, Msg string }{Target: target, Msg: msg})
	return nil
}

func (a *recordAdapter) GetSent() []struct{ Target, Msg string } {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]struct{ Target, Msg string }, len(a.sentMsgs))
	copy(cp, a.sentMsgs)
	return cp
}

type mockEchoProvider struct {
	mu     sync.Mutex
	calls  []core.ChatRequest
	toolFn func(req core.ChatRequest) (<-chan core.StreamEvent, error)
}

func (p *mockEchoProvider) Chat(ctx context.Context, req core.ChatRequest) (core.ChatResponse, error) {
	p.mu.Lock()
	p.calls = append(p.calls, req)
	p.mu.Unlock()
	return core.ChatResponse{Content: "echo: " + req.Messages[len(req.Messages)-1].Content}, nil
}

func (p *mockEchoProvider) Stream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	p.mu.Lock()
	p.calls = append(p.calls, req)
	fn := p.toolFn
	p.mu.Unlock()

	if fn != nil {
		return fn(req)
	}

	ch := make(chan core.StreamEvent, 2)
	go func() {
		defer close(ch)
		last := req.Messages[len(req.Messages)-1].Content
		ch <- core.StreamEvent{Text: "Processed: " + last}
	}()
	return ch, nil
}

func (p *mockEchoProvider) Models() []core.ModelInfo {
	return []core.ModelInfo{{ID: "mock-model", Provider: "mock"}}
}

func TestEcosystem_EndToEnd_CrossComponentIntegration(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Memory DB
	repo, err := memory.NewRepository(ctx, ":memory:")
	if err != nil {
		t.Fatalf("memory.NewRepository() error = %v", err)
	}
	defer repo.Close()

	// 2. Gateway Adapters & Multiplexer
	tgAdapter := newRecordAdapter("telegram", []string{"12345"})
	discordAdapter := newRecordAdapter("discord", []string{"user-99"})

	// 3. Plugin Manager with dynamic plugin tool
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "sample-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir error = %v", err)
	}
	manifestJSON := `{
		"name": "sample-plugin",
		"version": "1.0.0",
		"description": "Integration Test Plugin",
		"tools": [
			{
				"name": "plugin_greet",
				"description": "Greets from plugin"
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("write plugin.json error = %v", err)
	}

	pluginMgr := plugins.NewManager(plugins.WithStateDir(tmpDir))
	if err := pluginMgr.Load(tmpDir); err != nil {
		t.Fatalf("pluginMgr.Load() error = %v", err)
	}
	if err := pluginMgr.Enable("sample-plugin"); err != nil {
		t.Fatalf("pluginMgr.Enable() error = %v", err)
	}

	// 4. Policy Guard & Brain
	policyFile := filepath.Join(tmpDir, "policy.yaml")
	_ = os.WriteFile(policyFile, []byte("tier: sandbox\n"), 0o600)
	pstore, err := policy.Load(policyFile)
	if err != nil {
		t.Fatalf("policy.Load() error = %v", err)
	}

	// Provider setup
	provider := &mockEchoProvider{
		toolFn: func(req core.ChatRequest) (<-chan core.StreamEvent, error) {
			ch := make(chan core.StreamEvent, 2)
			go func() {
				defer close(ch)
				last := req.Messages[len(req.Messages)-1].Content
				ch <- core.StreamEvent{Text: "Ecosystem Response: " + last}
			}()
			return ch, nil
		},
	}

	brain := core.NewBrain(
		repo,
		provider,
		core.WithTools(
			pluginMgr.Runners(),
			pstore,
			gateway.NewAutoDenyApprover(nil),
		),
	)

	mux := gateway.NewMultiplexer(
		gateway.WithMultiplexerBrain(brain),
		gateway.WithMultiplexerRepository(repo),
	)
	mux.RegisterAdapter(tgAdapter)
	mux.RegisterAdapter(discordAdapter)

	muxCtx, muxCancel := context.WithCancel(ctx)
	defer muxCancel()

	muxErrCh := make(chan error, 1)
	go func() {
		muxErrCh <- mux.Start(muxCtx)
	}()

	// Allow multiplexer to start
	time.Sleep(50 * time.Millisecond)

	// 5. Cron Engine with Gateway as Sender and time simulation
	cronEngine := cron.NewEngine(
		cron.WithEngineBrain(brain),
		cron.WithEngineRepository(repo),
		cron.WithEngineSender(mux),
	)

	err = cronEngine.AddJob(cron.Job{
		Name:     "ecosystem-health",
		Schedule: "@every 50ms",
		Prompt:   "Check ecosystem status",
		Target: &cron.Target{
			Adapter:   "discord",
			Recipient: "channel-general",
		},
	})
	if err != nil {
		t.Fatalf("cronEngine.AddJob() error = %v", err)
	}

	cronCtx, cronCancel := context.WithCancel(ctx)
	cronErrCh := make(chan error, 1)
	go func() {
		cronErrCh <- cronEngine.Start(cronCtx)
	}()

	// 6. Webhook Server with Gateway as Sender and HMAC secret
	webhookSecret := "top-secret-signing-key"
	webhookCfg := webhook.Config{
		Host:             "127.0.0.1",
		Port:             0,
		Path:             "/events",
		Secret:           webhookSecret,
		DefaultSessionID: "webhook-session",
		Target: &webhook.Target{
			Adapter:   "telegram",
			Recipient: "12345",
		},
	}

	whServer := webhook.NewServer(
		webhookCfg,
		webhook.WithBrain(brain),
		webhook.WithRepo(repo),
		webhook.WithSender(mux),
	)

	// --- TEST SCENARIO A: Valid Webhook Event -> Brain -> Gateway Telegram Adapter ---
	payloadA := []byte(`{"event":"deploy_success","service":"api-v2"}`)
	macA := hmac.New(sha256.New, []byte(webhookSecret))
	macA.Write(payloadA)
	sigA := "sha256=" + hex.EncodeToString(macA.Sum(nil))

	reqA := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(payloadA))
	reqA.Header.Set("X-Hub-Signature-256", sigA)
	wA := httptest.NewRecorder()

	whServer.ServeHTTP(wA, reqA)
	if wA.Code != http.StatusOK {
		t.Errorf("Webhook POST valid sig returned status = %d, want 200", wA.Code)
	}

	// Verify Telegram adapter received notification from webhook event
	tgSent := tgAdapter.GetSent()
	if len(tgSent) != 1 {
		t.Fatalf("Telegram adapter received %d messages, want 1", len(tgSent))
	}
	if tgSent[0].Target != "12345" {
		t.Errorf("Telegram sent target = %q, want '12345'", tgSent[0].Target)
	}
	if !bytes.Contains([]byte(tgSent[0].Msg), []byte("deploy_success")) {
		t.Errorf("Telegram message %q does not contain 'deploy_success'", tgSent[0].Msg)
	}

	// --- TEST SCENARIO B: Invalid Webhook Event -> 401 Unauthorized, No Gateway Send ---
	reqB := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader([]byte(`{"tampered":true}`)))
	reqB.Header.Set("X-Hub-Signature-256", "sha256=invalidhexsignature")
	wB := httptest.NewRecorder()

	whServer.ServeHTTP(wB, reqB)
	if wB.Code != http.StatusUnauthorized {
		t.Errorf("Webhook POST invalid sig returned status = %d, want 401", wB.Code)
	}
	// Telegram messages count must remain 1
	if len(tgAdapter.GetSent()) != 1 {
		t.Errorf("Telegram adapter received extra message on unauthorized webhook")
	}

	// --- TEST SCENARIO C: Direct Inbound Gateway Telegram Message -> Brain -> Reply ---
	inboundEv := gateway.MessageEvent{
		Adapter: "telegram",
		ChatID:  "12345",
		UserID:  "12345",
		Content: "Hello AGIS Telegram",
	}
	if err := mux.HandleEvent(ctx, inboundEv); err != nil {
		t.Fatalf("mux.HandleEvent() error = %v", err)
	}

	tgSent = tgAdapter.GetSent()
	if len(tgSent) != 2 {
		t.Fatalf("Telegram adapter received %d messages, want 2", len(tgSent))
	}
	if !bytes.Contains([]byte(tgSent[1].Msg), []byte("Hello AGIS Telegram")) {
		t.Errorf("Telegram response %q does not contain prompt echo", tgSent[1].Msg)
	}

	// --- TEST SCENARIO D: Trigger Cron Job -> Brain -> Gateway Discord Adapter ---
	// Wait for the @every 50ms cron job to trigger and deliver
	var discordSent []struct{ Target, Msg string }
	for i := 0; i < 20; i++ {
		discordSent = discordAdapter.GetSent()
		if len(discordSent) > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if len(discordSent) < 1 {
		t.Fatalf("Discord adapter received %d messages, want at least 1", len(discordSent))
	}
	if discordSent[0].Target != "channel-general" {
		t.Errorf("Discord target = %q, want 'channel-general'", discordSent[0].Target)
	}
	if !bytes.Contains([]byte(discordSent[0].Msg), []byte("Check ecosystem status")) {
		t.Errorf("Discord message %q does not contain cron prompt echo", discordSent[0].Msg)
	}

	// --- TEST SCENARIO E: Graceful shutdown of all subsystems ---
	_ = whServer.Stop()

	cronCancel()
	_ = cronEngine.Stop()

	muxCancel()
	_ = mux.Stop()

	select {
	case err := <-cronErrCh:
		if err != nil {
			t.Errorf("cronEngine exit error = %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("cronEngine did not exit cleanly")
	}

	select {
	case err := <-muxErrCh:
		if err != nil {
			t.Errorf("mux exit error = %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("mux did not exit cleanly")
	}
}
