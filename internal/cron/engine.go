package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/core"
)

// BrainRunner represents the interface required to execute user turns in the Brain.
type BrainRunner interface {
	Step(ctx context.Context, input string) error
	SetActiveConversation(id string)
}

// Sender delivers outbound notifications to chat platform adapters.
type Sender interface {
	Send(ctx context.Context, adapter string, target string, msg string) error
}

type scheduledEntry struct {
	job      Job
	schedule Schedule
	nextRun  time.Time
}

// Engine implements the cron Scheduler interface and coordinates periodic job executions.
type Engine struct {
	mu           sync.RWMutex
	jobs         []*scheduledEntry
	sessions     map[string]string
	sessionLocks map[string]*sync.Mutex

	brain   BrainRunner
	repo    core.Repository
	sender  Sender
	logger  *slog.Logger
	nowFunc func() time.Time

	wakeCh  chan struct{}
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	closed  bool
}

// EngineOption configures the Engine.
type EngineOption func(*Engine)

// WithEngineBrain wires the Brain execution runner.
func WithEngineBrain(b BrainRunner) EngineOption {
	return func(e *Engine) {
		e.brain = b
	}
}

// WithEngineRepository wires the persistence repository for conversation/message history.
func WithEngineRepository(r core.Repository) EngineOption {
	return func(e *Engine) {
		e.repo = r
	}
}

// WithEngineSender wires the notification sender.
func WithEngineSender(s Sender) EngineOption {
	return func(e *Engine) {
		e.sender = s
	}
}

// WithEngineLogger sets the structured logger.
func WithEngineLogger(l *slog.Logger) EngineOption {
	return func(e *Engine) {
		e.logger = l
	}
}

// WithEngineNow overrides the time provider (for deterministic tests).
func WithEngineNow(nowFunc func() time.Time) EngineOption {
	return func(e *Engine) {
		e.nowFunc = nowFunc
	}
}

// NewEngine constructs a new cron scheduler Engine.
func NewEngine(opts ...EngineOption) *Engine {
	e := &Engine{
		jobs:         make([]*scheduledEntry, 0),
		sessions:     make(map[string]string),
		sessionLocks: make(map[string]*sync.Mutex),
		logger:       slog.Default(),
		wakeCh:       make(chan struct{}, 1),
		nowFunc:      time.Now,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// now returns the current time using nowFunc or default time.Now.
func (e *Engine) now() time.Time {
	if e.nowFunc != nil {
		return e.nowFunc()
	}
	return time.Now()
}

// AddJob registers a new cron job with the engine.
func (e *Engine) AddJob(job Job) error {
	if err := ValidateJob(job); err != nil {
		return err
	}

	schedule, err := ParseSchedule(job.Schedule)
	if err != nil {
		return err
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return ErrSchedulerClosed
	}

	entry := &scheduledEntry{
		job:      job,
		schedule: schedule,
		nextRun:  schedule.Next(e.now()),
	}
	e.jobs = append(e.jobs, entry)
	e.mu.Unlock()

	e.triggerWake()
	return nil
}

// Jobs returns a copy of all registered jobs.
func (e *Engine) Jobs() []Job {
	e.mu.RLock()
	defer e.mu.RUnlock()

	res := make([]Job, len(e.jobs))
	for i, entry := range e.jobs {
		res[i] = entry.job
	}
	return res
}

func (e *Engine) triggerWake() {
	select {
	case e.wakeCh <- struct{}{}:
	default:
	}
}

// Start launches the background scheduler loop.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return ErrSchedulerClosed
	}
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("cron engine is already running")
	}
	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.running = true
	e.mu.Unlock()

	e.logger.Info("cron engine: starting background scheduler loop")

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.loop(ctx)
	}()

	return nil
}

// Stop gracefully terminates the scheduler and waits for inflight jobs to finish.
func (e *Engine) Stop() error {
	e.mu.Lock()
	if e.closed || !e.running {
		e.closed = true
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	e.running = false
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()

	e.logger.Info("cron engine: stopping background scheduler loop...")
	e.wg.Wait()
	e.logger.Info("cron engine: stopped successfully")
	return nil
}

func (e *Engine) loop(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	for {
		now := e.now()
		var readyJobs []*scheduledEntry
		var nextEarliest time.Time

		e.mu.Lock()
		for _, entry := range e.jobs {
			if entry.nextRun.IsZero() {
				entry.nextRun = entry.schedule.Next(now)
			}
			if !entry.nextRun.After(now) {
				readyJobs = append(readyJobs, entry)
				entry.nextRun = entry.schedule.Next(now)
			}
			if !entry.nextRun.IsZero() {
				if nextEarliest.IsZero() || entry.nextRun.Before(nextEarliest) {
					nextEarliest = entry.nextRun
				}
			}
		}
		e.mu.Unlock()

		// Execute ready jobs concurrently
		for _, r := range readyJobs {
			e.wg.Add(1)
			go func(j Job) {
				defer e.wg.Done()
				e.executeJob(ctx, j)
			}(r.job)
		}

		// Calculate wait duration
		var waitDur time.Duration
		if nextEarliest.IsZero() {
			waitDur = time.Hour
		} else {
			waitDur = nextEarliest.Sub(e.now())
			if waitDur < 0 {
				waitDur = 0
			}
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(waitDur)

		select {
		case <-ctx.Done():
			return
		case <-e.wakeCh:
			// Reset loop to recalculate next timer
			continue
		case <-timer.C:
			// Check and run due jobs
			continue
		}
	}
}

func (e *Engine) getSessionLock(sessionKey string) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	l, ok := e.sessionLocks[sessionKey]
	if !ok {
		l = &sync.Mutex{}
		e.sessionLocks[sessionKey] = l
	}
	return l
}

func (e *Engine) resolveConversation(ctx context.Context, sessionKey string) (string, error) {
	e.mu.RLock()
	convID, ok := e.sessions[sessionKey]
	e.mu.RUnlock()

	if ok && convID != "" {
		return convID, nil
	}

	if e.repo == nil {
		return "", fmt.Errorf("no repository wired")
	}

	conv, err := e.repo.CreateConversation(ctx, sessionKey)
	if err != nil {
		return "", fmt.Errorf("creating session conversation: %w", err)
	}

	e.mu.Lock()
	e.sessions[sessionKey] = conv.ID
	e.mu.Unlock()

	return conv.ID, nil
}

func (e *Engine) executeJob(ctx context.Context, job Job) {
	start := e.now()
	sessionKey := job.SessionID
	if sessionKey == "" {
		sessionKey = fmt.Sprintf("cron:%s", job.Name)
	}

	e.logger.Info("cron: job execution started", "job", job.Name, "session", sessionKey, "prompt", job.Prompt)

	sLock := e.getSessionLock(sessionKey)
	sLock.Lock()
	defer sLock.Unlock()

	convID, err := e.resolveConversation(ctx, sessionKey)
	if err != nil {
		e.logger.Error("cron: failed to resolve session conversation", "job", job.Name, "error", err)
		return
	}

	if e.brain == nil {
		e.logger.Warn("cron: no brain wired, skipping execution", "job", job.Name)
		return
	}

	e.brain.SetActiveConversation(convID)

	if err := e.brain.Step(ctx, job.Prompt); err != nil {
		e.logger.Error("cron: job execution failed", "job", job.Name, "duration", time.Since(start), "error", err)
		return
	}

	duration := time.Since(start)
	e.logger.Info("cron: job completed successfully", "job", job.Name, "duration", duration)

	var replyText string
	if e.repo != nil {
		msgs, err := e.repo.Messages(ctx, convID, 1)
		if err == nil && len(msgs) > 0 && msgs[0].Role == core.RoleAssistant {
			replyText = msgs[0].Content
		}
	}

	if ctx.Err() != nil {
		return
	}

	if job.Target != nil && job.Target.Adapter != "" && job.Target.Recipient != "" {
		if e.sender != nil && replyText != "" {
			if err := e.sender.Send(ctx, job.Target.Adapter, job.Target.Recipient, replyText); err != nil {
				e.logger.Error("cron: target notification send failed",
					"job", job.Name,
					"adapter", job.Target.Adapter,
					"recipient", job.Target.Recipient,
					"error", err,
				)
				return
			}
			e.logger.Info("cron: target notification delivered",
				"job", job.Name,
				"adapter", job.Target.Adapter,
				"recipient", job.Target.Recipient,
			)
		} else if e.sender == nil {
			e.logger.Warn("cron: notification target configured but no sender wired", "job", job.Name)
		}
	} else {
		e.logger.Info("cron: job output (no target configured)", "job", job.Name, "output", replyText)
	}
}
