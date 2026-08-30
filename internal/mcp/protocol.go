// Package mcp implements the Model Context Protocol (MCP) JSON-RPC 2.0 wire protocol,
// transports (stdio, sse), tool discovery and invocation.
package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

const (
	// JSONRPCVersion is the required JSON-RPC protocol version string.
	JSONRPCVersion = "2.0"

	// ProtocolVersion is the MCP specification version negotiated during handshake.
	ProtocolVersion = "2024-11-05"
)

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ErrInvalidJSONRPC is returned when a payload does not conform to JSON-RPC 2.0.
var ErrInvalidJSONRPC = errors.New("invalid JSON-RPC version: expected 2.0")

// MessageKind classifies an incoming raw JSON-RPC payload.
type MessageKind int

const (
	MessageKindInvalid MessageKind = iota
	MessageKindRequest
	MessageKindNotification
	MessageKindResponse
)

// JSONRPCRequest represents a JSON-RPC 2.0 request message.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification message (no ID).
type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response message.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a standard or application-level JSON-RPC error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Data != nil {
		return fmt.Sprintf("JSON-RPC error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// NewRequest constructs a new JSONRPCRequest with the standard JSON-RPC 2.0 version.
func NewRequest(id string, method string, params any) (*JSONRPCRequest, error) {
	return &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}, nil
}

// NewNotification constructs a new JSONRPCNotification with the standard JSON-RPC 2.0 version.
func NewNotification(method string, params any) (*JSONRPCNotification, error) {
	return &JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}, nil
}

type rawResponseWrapper struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// ParseResponse deserializes and validates a JSON-RPC 2.0 response payload.
func ParseResponse(data []byte) (*JSONRPCResponse, error) {
	var raw rawResponseWrapper
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decoding JSON-RPC response: %w", err)
	}

	if raw.JSONRPC != JSONRPCVersion {
		return nil, ErrInvalidJSONRPC
	}

	idStr := normalizeID(raw.ID)

	return &JSONRPCResponse{
		JSONRPC: raw.JSONRPC,
		ID:      idStr,
		Result:  raw.Result,
		Error:   raw.Error,
	}, nil
}

// normalizeID extracts a string representation of the ID whether numeric or string.
func normalizeID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	var num int64
	if err := json.Unmarshal(raw, &num); err == nil {
		return strconv.FormatInt(num, 10)
	}
	var flt float64
	if err := json.Unmarshal(raw, &flt); err == nil {
		return strconv.FormatFloat(flt, 'f', -1, 64)
	}
	return string(raw)
}

type messageProbe struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

// ClassifyMessage inspects a raw JSON payload and determines if it is a Request, Notification, or Response.
func ClassifyMessage(data []byte) MessageKind {
	var probe messageProbe
	if err := json.Unmarshal(data, &probe); err != nil {
		return MessageKindInvalid
	}
	if probe.JSONRPC != JSONRPCVersion {
		return MessageKindInvalid
	}

	hasID := len(probe.ID) > 0 && string(probe.ID) != "null"
	hasMethod := probe.Method != ""

	if hasMethod {
		if hasID {
			return MessageKindRequest
		}
		return MessageKindNotification
	}

	if hasID && (len(probe.Result) > 0 || len(probe.Error) > 0) {
		return MessageKindResponse
	}

	return MessageKindInvalid
}
