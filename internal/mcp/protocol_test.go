package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/SalvucciFacundo/agis/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocol_RequestSerialization(t *testing.T) {
	t.Run("creates valid request with ID and params", func(t *testing.T) {
		req, err := mcp.NewRequest("req-1", "tools/list", map[string]any{"cursor": "abc"})
		require.NoError(t, err)

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var raw map[string]any
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.Equal(t, mcp.JSONRPCVersion, raw["jsonrpc"])
		assert.Equal(t, "req-1", raw["id"])
		assert.Equal(t, "tools/list", raw["method"])
		params, ok := raw["params"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "abc", params["cursor"])
	})

	t.Run("creates valid request without params", func(t *testing.T) {
		req, err := mcp.NewRequest("req-2", "ping", nil)
		require.NoError(t, err)

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var raw map[string]any
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.Equal(t, mcp.JSONRPCVersion, raw["jsonrpc"])
		assert.Equal(t, "req-2", raw["id"])
		assert.Equal(t, "ping", raw["method"])
		assert.Nil(t, raw["params"])
	})
}

func TestProtocol_NotificationSerialization(t *testing.T) {
	t.Run("creates valid notification without ID", func(t *testing.T) {
		notif, err := mcp.NewNotification("notifications/initialized", map[string]any{"status": "ready"})
		require.NoError(t, err)

		data, err := json.Marshal(notif)
		require.NoError(t, err)

		var raw map[string]any
		err = json.Unmarshal(data, &raw)
		require.NoError(t, err)

		assert.Equal(t, mcp.JSONRPCVersion, raw["jsonrpc"])
		assert.Equal(t, "notifications/initialized", raw["method"])
		assert.Nil(t, raw["id"], "notifications MUST omit ID")
		params, ok := raw["params"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "ready", params["status"])
	})
}

func TestProtocol_ResponseParsing(t *testing.T) {
	t.Run("parses successful response with result", func(t *testing.T) {
		rawJSON := `{"jsonrpc":"2.0","id":"123","result":{"tools":[{"name":"echo"}]}}`
		resp, err := mcp.ParseResponse([]byte(rawJSON))
		require.NoError(t, err)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, "123", resp.ID)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Result)

		var resultData struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		err = json.Unmarshal(resp.Result, &resultData)
		require.NoError(t, err)
		require.Len(t, resultData.Tools, 1)
		assert.Equal(t, "echo", resultData.Tools[0].Name)
	})

	t.Run("parses error response with structured JSONRPCError", func(t *testing.T) {
		rawJSON := `{"jsonrpc":"2.0","id":"456","error":{"code":-32601,"message":"Method not found","data":"additional info"}}`
		resp, err := mcp.ParseResponse([]byte(rawJSON))
		require.NoError(t, err)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, "456", resp.ID)
		require.NotNil(t, resp.Error)
		assert.Equal(t, mcp.CodeMethodNotFound, resp.Error.Code)
		assert.Equal(t, "Method not found", resp.Error.Message)
		assert.Equal(t, "additional info", resp.Error.Data)
		assert.Contains(t, resp.Error.Error(), "JSON-RPC error -32601: Method not found")
	})

	t.Run("handles numeric ID in response by normalizing to string", func(t *testing.T) {
		rawJSON := `{"jsonrpc":"2.0","id":42,"result":"ok"}`
		resp, err := mcp.ParseResponse([]byte(rawJSON))
		require.NoError(t, err)
		assert.Equal(t, "42", resp.ID)
	})
}

func TestProtocol_MessageClassification(t *testing.T) {
	t.Run("identifies request", func(t *testing.T) {
		raw := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{}}`)
		kind := mcp.ClassifyMessage(raw)
		assert.Equal(t, mcp.MessageKindRequest, kind)
	})

	t.Run("identifies notification", func(t *testing.T) {
		raw := []byte(`{"jsonrpc":"2.0","method":"notifications/progress","params":{}}`)
		kind := mcp.ClassifyMessage(raw)
		assert.Equal(t, mcp.MessageKindNotification, kind)
	})

	t.Run("identifies response", func(t *testing.T) {
		raw := []byte(`{"jsonrpc":"2.0","id":"1","result":{}}`)
		kind := mcp.ClassifyMessage(raw)
		assert.Equal(t, mcp.MessageKindResponse, kind)
	})

	t.Run("identifies malformed message", func(t *testing.T) {
		raw := []byte(`{not valid json}`)
		kind := mcp.ClassifyMessage(raw)
		assert.Equal(t, mcp.MessageKindInvalid, kind)
	})
}

func TestProtocol_ValidationErrors(t *testing.T) {
	t.Run("rejects invalid jsonrpc version", func(t *testing.T) {
		raw := []byte(`{"jsonrpc":"1.0","id":"1","result":{}}`)
		_, err := mcp.ParseResponse(raw)
		assert.ErrorIs(t, err, mcp.ErrInvalidJSONRPC)
	})

	t.Run("rejects malformed payload", func(t *testing.T) {
		raw := []byte(`invalid`)
		_, err := mcp.ParseResponse(raw)
		assert.Error(t, err)
	})
}
