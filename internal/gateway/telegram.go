package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SalvucciFacundo/agis/internal/config"
	"github.com/SalvucciFacundo/agis/internal/core"
)

// TelegramMaxMessageLength is Telegram's per-message character limit (4096).
const TelegramMaxMessageLength = 4096

// TelegramConfig carries Telegram adapter configuration.
type TelegramConfig = config.TelegramConfig

// SplitMessage splits a text into chunks of at most limit runes each, preserving
// rune integrity for multibyte characters.
func SplitMessage(msg string, limit int) []string {
	if limit <= 0 {
		limit = TelegramMaxMessageLength
	}
	runes := []rune(msg)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) <= limit {
		return []string{msg}
	}

	var chunks []string
	for len(runes) > 0 {
		chunkSize := limit
		if len(runes) < chunkSize {
			chunkSize = len(runes)
		}
		chunks = append(chunks, string(runes[:chunkSize]))
		runes = runes[chunkSize:]
	}
	return chunks
}

// TelegramOption configures a TelegramAdapter.
type TelegramOption func(*TelegramAdapter)

// WithTelegramBaseURL sets a custom base API URL (useful in tests).
func WithTelegramBaseURL(url string) TelegramOption {
	return func(a *TelegramAdapter) {
		a.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithTelegramHandler sets the message event handler.
func WithTelegramHandler(h Handler) TelegramOption {
	return func(a *TelegramAdapter) {
		a.handler = h
	}
}

// WithTelegramPollInterval configures the polling interval.
func WithTelegramPollInterval(d time.Duration) TelegramOption {
	return func(a *TelegramAdapter) {
		a.pollInterval = d
	}
}

// WithTelegramLogger sets the logger.
func WithTelegramLogger(logger *slog.Logger) TelegramOption {
	return func(a *TelegramAdapter) {
		a.logger = logger
	}
}

// WithTelegramHTTPClient sets a custom HTTP client.
func WithTelegramHTTPClient(client *http.Client) TelegramOption {
	return func(a *TelegramAdapter) {
		a.client = client
	}
}

// WithTelegramTranscriber sets the audio transcription service.
func WithTelegramTranscriber(transcriber core.Transcriber) TelegramOption {
	return func(a *TelegramAdapter) {
		a.transcriber = transcriber
	}
}

// WithTelegramMaxImageSize configures the maximum photo size limit in bytes.
func WithTelegramMaxImageSize(maxBytes int64) TelegramOption {
	return func(a *TelegramAdapter) {
		a.maxImageSize = maxBytes
	}
}

// WithTelegramMaxAudioSize configures the maximum audio size limit in bytes.
func WithTelegramMaxAudioSize(maxBytes int64) TelegramOption {
	return func(a *TelegramAdapter) {
		a.maxAudioSize = maxBytes
	}
}

// TelegramAdapter implements the Adapter port for Telegram Bot API.
type TelegramAdapter struct {
	cfg          TelegramConfig
	baseURL      string
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

// NewTelegramAdapter constructs a new TelegramAdapter.
func NewTelegramAdapter(cfg TelegramConfig, opts ...TelegramOption) *TelegramAdapter {
	a := &TelegramAdapter{
		cfg:          cfg,
		baseURL:      "https://api.telegram.org",
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
func (a *TelegramAdapter) Name() string {
	return "telegram"
}

// Start connects to the Telegram Bot API and begins polling for updates.
func (a *TelegramAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return ErrAdapterClosed
	}
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.mu.Unlock()

	a.logger.Info("gateway: starting telegram adapter listener")
	a.wg.Add(1)
	go a.pollLoop(ctx)
	return nil
}

// Stop gracefully stops polling and drains inflight updates.
func (a *TelegramAdapter) Stop() error {
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
	a.logger.Info("gateway: stopped telegram adapter listener")
	return nil
}

// Send delivers a message to a Telegram chat, chunking if it exceeds 4096 characters.
func (a *TelegramAdapter) Send(ctx context.Context, target string, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return nil
	}
	chunks := SplitMessage(msg, TelegramMaxMessageLength)
	for _, chunk := range chunks {
		if err := a.sendMessage(ctx, target, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (a *TelegramAdapter) sendMessage(ctx context.Context, chatID string, text string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", a.baseURL, a.cfg.Token)
	body, err := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("marshaling telegram sendMessage payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating telegram sendMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing telegram sendMessage request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendMessage status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

type tgPhotoSize struct {
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type tgVoice struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
	Duration int    `json:"duration"`
}

type tgAudio struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
	Duration int    `json:"duration"`
}

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      *struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat *struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text    string        `json:"text"`
		Caption string        `json:"caption"`
		Photo   []tgPhotoSize `json:"photo"`
		Voice   *tgVoice      `json:"voice"`
		Audio   *tgAudio      `json:"audio"`
		Date    int64         `json:"date"`
	} `json:"message"`
}

type tgGetUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

type tgGetFileResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		FileID   string `json:"file_id"`
		FilePath string `json:"file_path"`
		FileSize int64  `json:"file_size"`
	} `json:"result"`
}

func (a *TelegramAdapter) getFile(ctx context.Context, fileID string) (string, error) {
	url := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", a.baseURL, a.cfg.Token, fileID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating getFile request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing getFile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("telegram getFile HTTP %d: %s", resp.StatusCode, string(b))
	}

	var data tgGetFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decoding telegram getFile response: %w", err)
	}
	if !data.OK || data.Result.FilePath == "" {
		return "", fmt.Errorf("telegram getFile returned empty file_path")
	}

	return data.Result.FilePath, nil
}

func (a *TelegramAdapter) pollLoop(ctx context.Context) {
	defer a.wg.Done()

	var offset int64 = 0
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updates, nextOffset, err := a.fetchUpdates(ctx, offset)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				a.logger.Warn("gateway: telegram poll error", "error", err)
				continue
			}
			offset = nextOffset
			for _, u := range updates {
				a.processUpdate(ctx, u)
			}
		}
	}
}

func (a *TelegramAdapter) fetchUpdates(ctx context.Context, offset int64) ([]tgUpdate, int64, error) {
	url := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=10", a.baseURL, a.cfg.Token, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, offset, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, offset, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, offset, fmt.Errorf("telegram getUpdates HTTP %d: %s", resp.StatusCode, string(b))
	}

	var data tgGetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, offset, fmt.Errorf("decoding telegram getUpdates: %w", err)
	}

	nextOffset := offset
	for _, u := range data.Result {
		if u.UpdateID >= nextOffset {
			nextOffset = u.UpdateID + 1
		}
	}

	return data.Result, nextOffset, nil
}

func (a *TelegramAdapter) processUpdate(ctx context.Context, u tgUpdate) {
	if u.Message == nil || u.Message.From == nil || u.Message.Chat == nil {
		return
	}
	userID := strconv.FormatInt(u.Message.From.ID, 10)
	chatID := strconv.FormatInt(u.Message.Chat.ID, 10)

	// Enforce static allowlist security
	if !IsAllowed(a.cfg.Allowlist, userID) {
		a.logger.Warn("gateway: telegram unauthorized message dropped",
			"user_id", userID,
			"chat_id", chatID,
			"platform", "telegram",
		)
		return
	}

	if a.handler == nil {
		return
	}

	content := u.Message.Text
	if content == "" {
		content = u.Message.Caption
	}

	var attachments []core.Attachment

	// 1. Process photos
	if len(u.Message.Photo) > 0 {
		var best tgPhotoSize
		for _, p := range u.Message.Photo {
			if p.Width*p.Height >= best.Width*best.Height {
				best = p
			}
		}

		filePath, err := a.getFile(ctx, best.FileID)
		if err != nil {
			a.logger.Warn("gateway: telegram getFile photo failed", "file_id", best.FileID, "error", err)
		} else {
			downloadURL := fmt.Sprintf("%s/file/bot%s/%s", a.baseURL, a.cfg.Token, filePath)
			data, mime, dlErr := DownloadMedia(ctx, a.client, downloadURL, a.maxImageSize)
			if dlErr != nil {
				a.logger.Warn("gateway: telegram photo download failed", "file_id", best.FileID, "error", dlErr)
			} else {
				attachments = append(attachments, core.Attachment{
					Type:     "image",
					MimeType: mime,
					Data:     data,
					Name:     "photo.jpg",
				})
			}
		}
	}

	// 2. Process voice or audio notes
	var audioFileID string
	var audioMime string
	var audioName string

	if u.Message.Voice != nil {
		audioFileID = u.Message.Voice.FileID
		audioMime = u.Message.Voice.MimeType
		if audioMime == "" {
			audioMime = "audio/ogg"
		}
		audioName = "voice.ogg"
	} else if u.Message.Audio != nil {
		audioFileID = u.Message.Audio.FileID
		audioMime = u.Message.Audio.MimeType
		audioName = u.Message.Audio.FileName
		if audioName == "" {
			audioName = "audio.mp3"
		}
	}

	if audioFileID != "" {
		filePath, err := a.getFile(ctx, audioFileID)
		if err != nil {
			a.logger.Warn("gateway: telegram getFile audio failed", "file_id", audioFileID, "error", err)
		} else {
			downloadURL := fmt.Sprintf("%s/file/bot%s/%s", a.baseURL, a.cfg.Token, filePath)
			data, mime, dlErr := DownloadMedia(ctx, a.client, downloadURL, a.maxAudioSize)
			if dlErr != nil {
				a.logger.Warn("gateway: telegram audio download failed", "file_id", audioFileID, "error", dlErr)
			} else {
				if mime == "" {
					mime = audioMime
				}
				attachments = append(attachments, core.Attachment{
					Type:     "audio",
					MimeType: mime,
					Data:     data,
					Name:     audioName,
				})

				if a.transcriber != nil {
					transcript, tErr := a.transcriber.Transcribe(ctx, data, mime)
					if tErr != nil {
						a.logger.Warn("gateway: telegram audio transcription failed", "error", tErr)
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
	}

	ev := MessageEvent{
		Adapter:     "telegram",
		UserID:      userID,
		ChatID:      chatID,
		Content:     content,
		Attachments: attachments,
		Timestamp:   time.Unix(u.Message.Date, 0),
	}

	if err := a.handler(ctx, ev); err != nil {
		a.logger.Error("gateway: telegram handler failed", "error", err, "user_id", userID)
	}
}
