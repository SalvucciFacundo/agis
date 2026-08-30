# MCP Client Spec

## Purpose

Provide a native Model Context Protocol (MCP) client in AGIS, supporting both `stdio` and `sse` transports, JSON-RPC 2.0 wire protocol, dynamic tool discovery (`tools/list`), tool execution (`tools/call`), and multi-server pool management.

## Requirements

### Requirement AGIS-M8-MCP-001: JSON-RPC 2.0 Wire Protocol and Lifecycle Handshake
The system MUST implement a pure Go JSON-RPC 2.0 wire protocol client in `internal/mcp/` conforming to the Model Context Protocol specification:
1. Every JSON-RPC request and response MUST include `"jsonrpc": "2.0"`.
2. Requests MUST include a unique numeric or string `id`, a string `method`, and an optional `params` object.
3. Notifications (such as `notifications/initialized` or `notifications/cancelled`) MUST omit the `id` field.
4. The client MUST initialize connections by sending an `initialize` request with client info (name: `"agis"`, version) and supported capabilities (tools), negotiating protocol version `"2024-11-05"`.
5. Upon receiving a successful `initialize` response, the client MUST send a `notifications/initialized` notification before executing any tool or resource operations.
6. The client MUST correlate incoming JSON-RPC responses to pending requests using atomic request ID tracking, resolving the associated caller channel or promise.
7. Requests with exceeded context deadlines MUST be canceled and surface context deadline errors.
8. Standard JSON-RPC 2.0 error codes (e.g. `-32700` Parse error, `-32600` Invalid Request, `-32601` Method not found, `-32602` Invalid params, `-32603` Internal error) and application error objects MUST be decoded into structured Go error types.

#### Scenario: Successful protocol handshake
- GIVEN an active MCP transport connection
- WHEN client initiates connection handshake
- THEN client sends `initialize` request with protocol version `"2024-11-05"`, receives server capabilities, and sends `notifications/initialized` notification

#### Scenario: Request-response correlation via ID matching
- GIVEN three concurrent tool requests with IDs 1, 2, and 3
- WHEN server returns response for ID 2 before ID 1
- THEN the client resolves the caller awaiting response ID 2 with matching payload without cross-talk

#### Scenario: JSON-RPC error response decoded
- GIVEN a request to an unknown method
- WHEN server returns `{"jsonrpc": "2.0", "id": 10, "error": {"code": -32601, "message": "Method not found"}}`
- THEN the client returns a structured error containing code `-32601` and message `"Method not found"`

#### Scenario: Context cancellation aborts pending request
- GIVEN a tool invocation request with a 100ms timeout
- WHEN server does not respond within 100ms
- THEN the client cancels the pending request, cleans up tracking state, and returns `context.DeadlineExceeded`

---

### Requirement AGIS-M8-MCP-002: Stdio Subprocess Transport
The system MUST provide a `stdio` process transport in `internal/mcp/transport/stdio.go` for local MCP servers:
1. The transport MUST spawn the target binary using `os/exec.CommandContext` with configured command, arguments, working directory, and environment variables.
2. The transport MUST communicate with the subprocess via standard I/O streams: writing newline-delimited JSON-RPC messages to `stdin` and reading newline-delimited JSON-RPC messages from `stdout`.
3. Standard error (`stderr`) MUST be captured and redirected to AGIS logger/diagnostics and MUST NOT pollute or corrupt the `stdout` JSON-RPC stream parser.
4. Process termination MUST use process group cleanup (`Setpgid: true` on POSIX systems) to ensure child subprocesses spawned by the server process are also terminated.
5. Graceful shutdown MUST enforce a `WaitDelay` (default 100ms) after context cancellation or EOF to prevent orphan or zombie processes from hanging AGIS.
6. If the child process crashes or exits unexpectedly, the transport MUST detect EOF on `stdout`, mark the transport as disconnected, and return a descriptive process exit error on subsequent calls.

#### Scenario: Stdio transport executes subprocess and exchanges messages
- GIVEN an executable MCP server configured with `command: "node"` and `args: ["server.js"]`
- WHEN transport starts and client sends `initialize`
- THEN the subprocess is spawned, `initialize` is written to `stdin`, and the server response is parsed from `stdout`

#### Scenario: Stderr output is routed to logger without breaking JSON parser
- GIVEN an MCP server that writes debug text to `stderr` and JSON-RPC to `stdout`
- WHEN transport reads from the server
- THEN debug text is logged via AGIS logger and the JSON-RPC parser on `stdout` completes without error

#### Scenario: Context cancellation terminates process group cleanly
- GIVEN a running stdio MCP subprocess
- WHEN the supervising context is canceled
- THEN SIGTERM/SIGKILL is sent to the process group, the process is reaped within `WaitDelay`, and no zombie process remains

---

### Requirement AGIS-M8-MCP-003: SSE Network Transport
The system MUST provide an SSE (Server-Sent Events) network transport in `internal/mcp/transport/sse.go` for remote MCP servers:
1. The transport MUST establish an HTTP `GET` connection with the `Accept: text/event-stream` header to the configured server URL.
2. Upon connection, the transport MUST listen for an initial SSE `endpoint` event containing the relative or absolute URI for posting client messages.
3. Outgoing JSON-RPC requests MUST be transmitted via HTTP `POST` requests with `Content-Type: application/json` to the discovered session endpoint URI.
4. Incoming JSON-RPC responses and server notifications MUST be read as `message` events from the persistent SSE event stream and decoded.
5. The transport MUST respect HTTP client timeouts, proxy settings, and custom HTTP headers.
6. Closing the transport MUST abort the persistent SSE stream, close idle HTTP connections, and fail pending request channels.

#### Scenario: SSE connection established and endpoint discovered
- GIVEN a remote MCP server running at `http://localhost:8080/sse`
- WHEN SSE transport connects
- THEN it establishes the event stream, receives an `endpoint` event with URI `http://localhost:8080/messages?session_id=abc`, and configures the POST target

#### Scenario: JSON-RPC message dispatch via HTTP POST
- GIVEN an established SSE transport session
- WHEN client sends a `tools/list` request
- THEN transport issues HTTP `POST` to the session endpoint and receives the JSON-RPC response via the SSE message stream

---

### Requirement AGIS-M8-MCP-004: Tool Discovery and Schema Parsing
The system MUST discover and parse MCP tools via the `tools/list` protocol method:
1. When `ListTools` is invoked, the client MUST send a `tools/list` JSON-RPC request.
2. If the response contains a non-empty `nextCursor`, the client MUST issue subsequent paginated requests until all tools are discovered.
3. The client MUST extract tool definitions (`name`, `description`, `inputSchema`) from the response `tools` array.
4. Discovered tools MUST be indexed by server name and tool name, preventing collisions across multiple configured MCP servers.

#### Scenario: Discover single page of tools
- GIVEN an MCP server providing tools `"query"` and `"schema"`
- WHEN `ListTools` is executed
- THEN both tool definitions are returned with their respective names and input schemas

#### Scenario: Paginated tool discovery
- GIVEN an MCP server returning 50 tools on page 1 with `nextCursor: "page2"` and 25 tools on page 2
- WHEN `ListTools` is called
- THEN the client retrieves both pages and returns all 75 tools

---

### Requirement AGIS-M8-MCP-005: Tool Execution and Result Mapping
The system MUST execute MCP tools via the `tools/call` protocol method:
1. When a tool is invoked, the client MUST send a `tools/call` request containing `name` and `arguments`.
2. The response `result` object MUST be parsed:
   - Text content blocks (`type: "text"`) MUST be concatenated into the result string.
   - Image content blocks (`type: "image"`) MUST be preserved in binary format or base64 representation.
   - Embedded resource blocks MUST be extracted into the response.
3. If the server response contains `isError: true`, the client MUST return the content as a descriptive tool execution error.
4. If the call times out or context cancels, the client MUST send a `notifications/cancelled` notification to the server if supported.

#### Scenario: Successful tool call returns text content
- GIVEN an MCP tool `"get_user"`
- WHEN called with arguments `{"id": "123"}`
- THEN the server returns `{"content": [{"type": "text", "text": "Alice"}]}` and the client returns `"Alice"` with `nil` error

#### Scenario: Tool call returning error flag surfaced as error
- GIVEN an invalid query to a database MCP tool
- WHEN called with malformed SQL
- THEN the server returns `{"isError": true, "content": [{"type": "text", "text": "Syntax error"}]}` and the client surfaces the syntax error
