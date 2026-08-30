package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// DiscordMaxMessageLength is Discord's per-message character limit (2000).
const DiscordMaxMessageLength = 2000

// DiscordConfig carries Discord adapter configuration.
type DiscordConfig = config.DiscordConfig

// DiscordOption configures a DiscordAdapter.
type DiscordOption func(*DiscordAdapter)

// WithDiscordBaseURL sets a custom base API URL (useful in tests).
func WithDiscordBaseURL(url string) DiscordOption {
	return func(a *DiscordAdapter) {
		a.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithDiscordHandler sets the message event handler.
func WithDiscordHandler(h Handler) DiscordOption {
	return func(a *DiscordAdapter) {
		a.handler = h
	}
}

// WithDiscordPollInterval configures the polling interval.
func WithDiscordPollInterval(d time.Duration) DiscordOption {
	return func(a *DiscordAdapter) {
		a.pollInterval = d
	}
}

// WithDiscordPollChannels configures channel IDs to poll for messages.
func WithDiscordPollChannels(channels []string) DiscordOption {
	return func(a *DiscordAdapter) {
		a.channels = channels
	}
}

// WithDiscordLogger sets the logger.
func WithDiscordLogger(logger *slog.Logger) DiscordOption {
	return func(a *DiscordAdapter) {
		a.logger = logger
	}
}

// WithDiscordHTTPClient sets a custom HTTP client.
func WithDiscordHTTPClient(client *http.Client) DiscordOption {
	return func(a *DiscordAdapter) {
		a.client = client
	}
}

// WithDiscordTranscriber sets the audio transcription service.
func WithDiscordTranscriber(transcriber core.Transcriber) DiscordOption {
	return func(a *DiscordAdapter) {
		a.transcriber = transcriber
	}
}

// WithDiscordMaxImageSize configures the maximum image size limit in bytes.
func WithDiscordMaxImageSize(maxBytes int64) DiscordOption {
	return func(a *DiscordAdapter) {
		a.maxImageSize = maxBytes
	}
}

// WithDiscordMaxAudioSize configures the maximum audio size limit in bytes.
func WithDiscordMaxAudioSize(maxBytes int64) DiscordOption {
	return func(a *DiscordAdapter) {
		a.maxAudioSize = maxBytes
	}
}

// DiscordAdapter implements the Adapter port for Discord Bot REST/Gateway API.
type DiscordAdapter struct {
	cfg          DiscordConfig
	baseURL      string
	channels     []string
	handler      Handler
	logger       *slog.Logger
	client       *http.Client
	pollInterval time.Duration
	transcriber  core.Transcriber
	maxImageSize int64
	maxAudioSize int64

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

// NewDiscordAdapter constructs a new DiscordAdapter.
func NewDiscordAdapter(cfg DiscordConfig, opts ...DiscordOption) *DiscordAdapter {
	a := &DiscordAdapter{
		cfg:          cfg,
		baseURL:      "https://discord.com/api/v10",
		pollInterval: 1 * time.Second,
		client:       &http.Client{Timeout: 30 * time.Second},
		logger:       slog.Default(),
		maxImageSize: DefaultMaxImageSize,
		maxAudioSize: DefaultMaxAudioSize,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name returns the adapter name.
func (a *DiscordAdapter) Name() string {
	return "discord"
}

// Start connects or starts listening for Discord message events.
func (a *DiscordAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return ErrAdapterClosed
	}
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.mu.Unlock()

	a.logger.Info("gateway: starting discord adapter listener")
	a.wg.Add(1)
	go a.pollLoop(ctx)
	return nil
}

// Stop gracefully stops listeners and drains inflight operations.
func (a *DiscordAdapter) Stop() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()

	a.wg.Wait()
	a.logger.Info("gateway: stopped discord adapter listener")
	return nil
}

// Send delivers a message to a Discord channel, splitting if it exceeds 2000 characters.
func (a *DiscordAdapter) Send(ctx context.Context, target string, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}
	chunks := SplitMessage(msg, DiscordMaxMessageLength)
	for _, chunk := range chunks {
		if err := a.sendMessage(ctx, target, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (a *DiscordAdapter) sendMessage(ctx context.Context, channelID string, text string) error {
	url := fmt.Sprintf("%s/channels/%s/messages", a.baseURL, channelID)
	body, err := json.Marshal(map[string]any{
		"content": text,
	})
	if err != nil {
		return fmt.Errorf("marshaling discord message payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating discord sendMessage request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+a.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing discord sendMessage request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord sendMessage status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

type discordAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	ProxyURL    string `json:"proxy_url"`
}

type discordMessage struct {
	ID          string              `json:"id"`
	ChannelID   string              `json:"channel_id"`
	Content     string              `json:"content"`
	Author      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
	Attachments []discordAttachment `json:"attachments"`
	Timestamp   string              `json:"timestamp"`
}

func (a *DiscordAdapter) pollLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	lastSeen := make(map[string]string) // channelID -> last message ID

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, chID := range a.channels {
				msgs, err := a.fetchChannelMessages(ctx, chID, lastSeen[chID])
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					a.logger.Warn("gateway: discord poll error", "channel_id", chID, "error", err)
					continue
				}
				if len(msgs) > 0 {
					lastSeen[chID] = msgs[0].ID
					// Messages from discord API are returned newest first; process oldest first
					for i := len(msgs) - 1; i >= 0; i-- {
						a.processMessage(ctx, msgs[i])
					}
				}
			}
		}
	}
}

func (a *DiscordAdapter) fetchChannelMessages(ctx context.Context, channelID, after string) ([]discordMessage, error) {
	url := fmt.Sprintf("%s/channels/%s/messages?limit=20", a.baseURL, channelID)
	if after != "" {
		url += "&after=" + after
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+a.cfg.Token)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord getMessages HTTP %d: %s", resp.StatusCode, string(b))
	}

	var msgs []discordMessage
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, fmt.Errorf("decoding discord messages: %w", err)
	}
	return msgs, nil
}

func (a *DiscordAdapter) processMessage(ctx context.Context, msg discordMessage) {
	// Skip bot-authored messages to prevent self-trigger loops
	if msg.Author.Bot {
		return
	}

	// Enforce static allowlist security
	if !IsAllowed(a.cfg.Allowlist, msg.Author.ID) {
		a.logger.Warn("gateway: discord unauthorized message dropped",
			"user_id", msg.Author.ID,
			"channel_id", msg.ChannelID,
			"platform", "discord",
		)
		return
	}

	if a.handler == nil {
		return
	}

	content := msg.Content
	var attachments []core.Attachment

	for _, att := range msg.Attachments {
		targetURL := att.URL
		if targetURL == "" {
			targetURL = att.ProxyURL
		}
		if targetURL == "" {
			continue
		}

		// Determine if attachment is likely audio or image
		isAudio := IsAllowedAudioMime(att.ContentType) || strings.HasSuffix(strings.ToLower(att.Filename), ".ogg") ||
			strings.HasSuffix(strings.ToLower(att.Filename), ".wav") || strings.HasSuffix(strings.ToLower(att.Filename), ".mp3") ||
			strings.HasSuffix(strings.ToLower(att.Filename), ".m4a")

		maxSize := a.maxImageSize
		if isAudio {
			maxSize = a.maxAudioSize
		}

		data, mime, err := DownloadMedia(ctx, a.client, targetURL, maxSize)
		if err != nil {
			a.logger.Warn("gateway: discord attachment download failed", "filename", att.Filename, "url", targetURL, "error", err)
			continue
		}

		if IsAllowedImageMime(mime) {
			attachments = append(attachments, core.Attachment{
				Type:     "image",
				MimeType: mime,
				Data:     data,
				Name:     att.Filename,
				URL:      targetURL,
			})
		} else if IsAllowedAudioMime(mime) {
			attachments = append(attachments, core.Attachment{
				Type:     "audio",
				MimeType: mime,
				Data:     data,
				Name:     att.Filename,
				URL:      targetURL,
			})

			if a.transcriber != nil {
				transcript, tErr := a.transcriber.Transcribe(ctx, data, mime)
				if tErr != nil {
					a.logger.Warn("gateway: discord audio transcription failed", "filename", att.Filename, "error", tErr)
				} else if transcript != "" {
					if content == "" {
						content = transcript
					} else {
						content = content + "\n" + transcript
					}
				}
			}
		}
	}

	t, _ := time.Parse(time.RFC3339, msg.Timestamp)
	if t.IsZero() {
		t = time.Now()
	}

	ev := MessageEvent{
		Adapter:     "discord",
		UserID:      msg.Author.ID,
		ChatID:      msg.ChannelID,
		Content:     content,
		Attachments: attachments,
		Timestamp:   t,
	}

	if err := a.handler(ctx, ev); err != nil {
		a.logger.Error("gateway: discord handler failed", "error", err, "user_id", msg.Author.ID)
	}
}
