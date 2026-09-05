package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/SalvucciFacundo/agis/internal/core"
)

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", "invalid_request_error", nil, "method_not_allowed")
		return
	}

	// Limit request body size to 10MB to prevent DoS attacks
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON request body: %v", err), "invalid_request_error", nil, "invalid_json")
		return
	}

	if len(req.Messages) == 0 {
		param := "messages"
		s.writeError(w, http.StatusBadRequest, "messages array is required and cannot be empty", "invalid_request_error", &param, "missing_required_parameter")
		return
	}

	sessionID := strings.TrimSpace(r.Header.Get("X-Session-ID"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(req.User)
	}

	prompt, attachments, err := extractPromptAndAttachments(req.Messages)
	if err != nil || prompt == "" {
		s.writeError(w, http.StatusBadRequest, "No user prompt provided in messages", "invalid_request_error", nil, "invalid_prompt")
		return
	}

	if s.opts.Brain == nil {
		s.writeError(w, http.StatusInternalServerError, "No brain engine configured", "api_error", nil, "internal_error")
		return
	}

	modelID := req.Model
	if modelID == "" {
		modelID = s.opts.Model
	}
	if modelID == "" {
		modelID = "llama3.2"
	}

	if req.Stream {
		flusher, ok := w.(http.Flusher)
		if !ok {
			s.writeError(w, http.StatusInternalServerError, "Streaming unsupported by response writer", "api_error", nil, "streaming_unsupported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		chunkID := "chatcmpl-" + uuid.New().String()
		created := time.Now().Unix()

		sink := func(text string) {
			if text == "" {
				return
			}
			chunk := ChatCompletionChunk{
				ID:      chunkID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   modelID,
				Choices: []ChunkChoice{
					{
						Index: 0,
						Delta: ChunkDelta{
							Content: text,
						},
						FinishReason: nil,
					},
				},
			}
			data, marshalErr := json.Marshal(chunk)
			if marshalErr == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}

		err = s.opts.Brain.StepWithSessionAndAttachments(r.Context(), sessionID, prompt, attachments, sink)
		if err != nil {
			if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
				return
			}
			if s.opts.Logger != nil {
				s.opts.Logger.Error("streaming chat completion failed", "error", err)
			}
			return
		}

		stopReason := "stop"
		stopChunk := ChatCompletionChunk{
			ID:      chunkID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   modelID,
			Choices: []ChunkChoice{
				{
					Index:        0,
					Delta:        ChunkDelta{},
					FinishReason: &stopReason,
				},
			},
		}
		if data, marshalErr := json.Marshal(stopChunk); marshalErr == nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	// Non-streaming completion
	var replyBuf strings.Builder
	sink := func(text string) {
		replyBuf.WriteString(text)
	}

	err = s.opts.Brain.StepWithSessionAndAttachments(r.Context(), sessionID, prompt, attachments, sink)
	if err != nil {
		if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
			return
		}
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Internal completion error: %v", err), "api_error", nil, "internal_error")
		return
	}

	resp := ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelID,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChoiceMessage{
					Role:    "assistant",
					Content: replyBuf.String(),
				},
				FinishReason: "stop",
			},
		},
		Usage: &Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func extractPromptAndAttachments(msgs []ChatCompletionMessage) (string, []core.Attachment, error) {
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if strings.EqualFold(msg.Role, "user") || i == len(msgs)-1 {
			switch c := msg.Content.(type) {
			case string:
				return c, nil, nil
			case []any:
				var textParts []string
				var attachments []core.Attachment
				for _, part := range c {
					if m, ok := part.(map[string]any); ok {
						if typ, ok := m["type"].(string); ok {
							switch typ {
							case "text":
								if txt, ok := m["text"].(string); ok {
									textParts = append(textParts, txt)
								}
							case "image_url":
								if imgObj, ok := m["image_url"].(map[string]any); ok {
									if urlStr, ok := imgObj["url"].(string); ok {
										attachments = append(attachments, core.Attachment{
											Name:     "image",
											URL:      urlStr,
											MimeType: "image/jpeg",
										})
									}
								}
							}
						}
					}
				}
				return strings.Join(textParts, "\n"), attachments, nil
			}
		}
	}
	return "", nil, fmt.Errorf("no user message found")
}

func (s *Server) writeError(w http.ResponseWriter, status int, message, errType string, param *string, code string) {
	resp := ErrorResponse{
		Error: APIError{
			Message: message,
			Type:    errType,
			Param:   param,
			Code:    code,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
