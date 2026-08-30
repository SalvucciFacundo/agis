# M8 — Model Context Protocol (MCP) Client (Delta Spec)

Delta specification for `m8-mcp-client`: Native MCP Client (`internal/mcp/`), `stdio` & `sse` Transports, JSON-RPC 2.0 Protocol, Tool Discovery (`tools/list`), Tool Execution (`tools/call`), `ToolRunner` & `PolicyGuard` Integration, Configuration Schema, and MCP CLI Subcommands.

---

## mcp (NEW)

### AGIS-M8-MCP-001: JSON-RPC 2.0 Wire Protocol and Lifecycle Handshake
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

### AGIS-M8-MCP-002: Stdio Subprocess Transport
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

#### Scenario: Premature subprocess exit detected
- GIVEN a stdio MCP subprocess that crashes or exits with code 1
- WHEN the client attempts to send or read a message
- THEN the transport detects EOF/process termination and returns a descriptive connection closed error

---

### AGIS-M8-MCP-003: SSE Network Transport
The system MUST provide an SSE (Server-Sent Events) network transport in `internal/mcp/transport/sse.go` for remote MCP servers:
1. The transport MUST establish an HTTP `GET` connection with the `Accept: text/event-stream` header to the configured server URL.
2. Upon connection, the transport MUST listen for an initial SSE `endpoint` event containing the relative or absolute URI for posting client messages.
3. Outgoing JSON-RPC requests MUST be transmitted via HTTP `POST` requests with `Content-Type: application/json` to the discovered session endpoint URI.
4. Incoming JSON-RPC responses and server notifications MUST be read as `message` events from the persistent SSE event stream and decoded.
5. The transport MUST respect HTTP client timeouts, proxy settings, and custom HTTP headers (such as `Authorization`).
6. Closing the transport MUST abort the persistent SSE stream, close idle HTTP connections, and fail pending request channels.

#### Scenario: SSE connection established and endpoint discovered
- GIVEN a remote MCP server running at `http://localhost:8080/sse`
- WHEN SSE transport connects
- THEN it establishes the event stream, receives an `endpoint` event with URI `http://localhost:8080/messages?session_id=abc`, and configures the POST target

#### Scenario: JSON-RPC message dispatch via HTTP POST
- GIVEN an established SSE transport session
- WHEN client sends a `tools/list` request
- THEN transport issues HTTP `POST` to the session endpoint and receives the JSON-RPC response via the SSE message stream

#### Scenario: Network disconnection triggers error and cleanup
- GIVEN an active SSE transport connection
- WHEN remote server closes the connection or network drops
- THEN SSE transport detects EOF, cancels pending callers with connection errors, and cleans up internal goroutines

---

### AGIS-M8-MCP-004: Tool Discovery and Schema Parsing
The system MUST discover and parse MCP tools via the `tools/list` protocol method:
1. The client MUST call `tools/list` on initialized MCP servers to query available tool definitions.
2. The client MUST handle paginated responses if the server returns a non-empty `nextCursor`, requesting successive pages until all tools are retrieved.
3. For each tool entry in `tools/list`, the client MUST extract `name` (string), `description` (string), and `inputSchema` (JSON schema object containing `type`, `properties`, `required`).
4. Extracted tools MUST be converted into AGIS skill/tool metadata definitions for exposure to the LLM Brain and registration with the tool registry.
5. If `tools/list` returns an empty array, the client MUST handle it gracefully without error, registering zero tools for that server.

#### Scenario: Discover tools from MCP server
- GIVEN a running MCP server exposing tools `read_file` and `write_file`
- WHEN `client.ListTools(ctx)` is invoked
- THEN both tool definitions are returned with their complete parameter schemas and descriptions

#### Scenario: Paginated tool discovery
- GIVEN an MCP server with 25 tools returning 10 per page with `nextCursor`
- WHEN `client.ListTools(ctx)` is invoked
- THEN client requests all 3 pages and returns the complete list of 25 tools

#### Scenario: MCP server with no tools
- GIVEN an MCP server returning `{"tools": []}`
- WHEN `client.ListTools(ctx)` is invoked
- THEN an empty slice of tools is returned with `nil` error

---

### AGIS-M8-MCP-005: Tool Execution and Result Mapping
The system MUST execute MCP tools via the `tools/call` protocol method and map results:
1. The client MUST issue `tools/call` with parameters `name` (string) and `arguments` (map or JSON object).
2. The client MUST parse the `CallToolResult` response containing `content` (slice of content blocks) and optional `isError` (boolean).
3. Supported content block types MUST include:
   - `text`: Extracted as UTF-8 string content.
   - `image`: Extracted as base64-encoded image data with MIME type.
   - `resource`: Extracted as embedded URI/text resource content.
4. Multiple text content blocks MUST be concatenated into a single output string separated by newlines.
5. If the server returns `isError: true` in the `CallToolResult`, the tool execution MUST be treated as an execution error and surface the combined content as the error message.
6. JSON-RPC level errors returned by the server MUST be returned directly as transport/protocol errors.

#### Scenario: Successful tool call with text content
- GIVEN an MCP server with tool `echo`
- WHEN `client.CallTool(ctx, "echo", map[string]any{"msg": "hello"})` is called
- THEN server returns `{"content": [{"type": "text", "text": "hello"}], "isError": false}` and the method returns output string `"hello"` with `nil` error

#### Scenario: Tool call returning multiple content blocks
- GIVEN a tool returning two text blocks `["header line", "detail line"]`
- WHEN tool execution completes
- THEN client returns combined string `"header line\ndetail line"`

#### Scenario: Tool execution error with isError true
- GIVEN a tool returning `{"content": [{"type": "text", "text": "file not found"}], "isError": true}`
- WHEN tool execution completes
- THEN client returns a tool failure error wrapping `"file not found"`

---

## tools (MODIFIED)

### AGIS-M8-TLS-001: MCP ToolRunner Bridge & Policy Guard Integration
The system MUST bridge discovered MCP tools into the `core.ToolRunner` interface in `internal/tools/`:
1. Each discovered MCP tool MUST be wrapped in an adapter implementing `core.ToolRunner` with methods `Name() string`, `Description() string`, and `Execute(ctx context.Context, args string, guard PolicyGuard) (string, error)`.
2. The adapter MUST assign the backend name `mcp:<server_name>` (e.g. `mcp:filesystem`, `mcp:github`) for Policy Guard evaluation and audit logging.
3. Policy Guard evaluation for MCP tools MUST enforce:
   - **Sandbox posture**: Denies all MCP tool invocations by default unless an explicit `allow` rule exists for `mcp:<server_name>` in `policy.yaml`.
   - **Standard posture**: Evaluates category and subject rules; prompts via `AskApprover` in interactive sessions when no matching rule exists.
   - **Full posture**: Permits execution for the current session.
   - **AutoDenyApprover**: In non-interactive or background daemon execution (e.g. Gateway, Cron, Webhook), any unapproved MCP tool invocation requiring user approval MUST be automatically denied.
4. Tool arguments passed as raw JSON strings MUST be validated before forwarding to `client.CallTool`.
5. Every MCP tool execution, decision (`allow`, `deny`, `ask`), and result MUST be recorded in the AGIS audit log.
(Previously: `ToolRunner` only supported static backends `local`, `docker`, and `ssh`.)

#### Scenario: Sandbox posture blocks unapproved MCP tool
- GIVEN `policy.yaml` with backend `mcp:github` set to posture `sandbox` and no explicit allow rules
- WHEN model invokes MCP tool `mcp:github:create_issue`
- THEN Policy Guard returns `deny`, tool execution is prevented, and a deny audit entry is recorded

#### Scenario: Standard posture prompts user in interactive session
- GIVEN backend `mcp:filesystem` set to posture `standard` and no matching rule for tool `delete_file`
- WHEN model invokes `mcp:filesystem:delete_file`
- THEN Policy Guard returns `ask`, presenting approval to the interactive session

#### Scenario: AutoDenyApprover denies MCP tool in headless cron daemon
- GIVEN a background cron job triggering a brain step that calls an unapproved MCP tool
- WHEN Policy Guard evaluates the tool call using `AutoDenyApprover`
- THEN the decision is `deny` and the tool does not execute

#### Scenario: Approved MCP tool executes and audits result
- GIVEN an `allow` rule for `mcp:sqlite:query`
- WHEN model invokes the tool with `{"query": "SELECT 1"}`
- THEN tool executes via MCP client, returns query result, and logs success in audit log

---

## config-loader (MODIFIED)

### AGIS-M8-CONF-001: MCP Configuration Schema
The configuration loader in `internal/config/config.go` MUST support the optional `mcp` root configuration block:

```yaml
mcp:
  enabled: false
  servers:
    filesystem:
      command: "npx"
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
      env:
        DEBUG: "mcp:*"
      disabled: false
    remote-tools:
      url: "http://localhost:8080/sse"
      disabled: false
```

Rules:
1. `mcp.enabled` MUST default to `false` (opt-in feature).
2. `mcp.servers` MUST default to an empty map (`map[string]MCPServerConfig{}`).
3. Each server entry in `mcp.servers` MUST define:
   - `command` (string, optional): Executable binary for stdio transport.
   - `args` ([]string, optional): Command-line arguments for stdio transport.
   - `env` (map[string]string, optional): Additional environment variables.
   - `url` (string, optional): HTTP/HTTPS URL for SSE transport.
   - `disabled` (bool, default `false`): When `true`, the server is skipped during initialization.
4. Server validation: A server configuration MUST specify either `command` (stdio) or `url` (sse); specifying both or neither MUST produce a configuration validation error or warning.
5. Omission of the `mcp` block in `config.yaml` MUST preserve default values without error.
(Previously: `Config` struct did not contain an `mcp` section.)

#### Scenario: Default configuration disables MCP
- GIVEN an empty or standard `config.yaml` without an `mcp` block
- WHEN `config.Load()` is executed
- THEN `cfg.MCP.Enabled` is `false` and `cfg.MCP.Servers` is empty

#### Scenario: Load stdio MCP server configuration
- GIVEN `config.yaml` containing an `mcp` block with stdio server `filesystem`
- WHEN `config.Load()` is executed
- THEN `cfg.MCP.Enabled` is `true`, `cfg.MCP.Servers["filesystem"].Command` is `"npx"`, and `Args` are parsed correctly

#### Scenario: Load SSE MCP server configuration
- GIVEN `config.yaml` containing an SSE server with `url: "http://localhost:8080/sse"`
- WHEN `config.Load()` is executed
- THEN `cfg.MCP.Servers["remote-tools"].URL` is `"http://localhost:8080/sse"` and `Disabled` is `false`

#### Scenario: Disabled server is recognized
- GIVEN a server entry with `disabled: true`
- WHEN `config.Load()` is executed
- THEN `cfg.MCP.Servers["test-server"].Disabled` is `true`

---

## cli (MODIFIED)

### AGIS-M8-CLI-001: MCP CLI Subcommands (`agis mcp list`, `agis mcp test`)
The `cmd/agis/` CLI entry points MUST provide the following `mcp` subcommands:
1. `agis mcp list`:
   - Iterates over all configured servers in `cfg.MCP.Servers`.
   - For each enabled server, establishes connection, performs handshake, queries `tools/list`, and prints a formatted table displaying server name, transport type (`stdio` or `sse`), status (`online`, `offline`, `disabled`), and list of discovered tool names with descriptions.
   - Accepts `--config` flag to specify custom config path.
   - Returns exit code 0 if all enabled servers respond, or non-zero if connection to an enabled server fails.
2. `agis mcp test <server> <tool> [args]`:
   - Connects to the specified `<server>`, verifies `<tool>` exists, and executes `tools/call` directly with the provided JSON `[args]` string (or empty `{}` if omitted).
   - Outputs tool response text/content and execution duration.
   - Bypasses LLM orchestration for standalone testing and troubleshooting.
   - Returns exit code 0 on successful tool execution, and non-zero on tool error, policy denial, or transport failure.
(Previously: `agis` CLI did not have an `mcp` root command or subcommands.)

#### Scenario: `agis mcp list` displays configured servers and tools
- GIVEN a configured stdio MCP server `filesystem` exposing tool `read_file`
- WHEN `agis mcp list` is executed
- THEN output displays server `filesystem`, transport `stdio`, status `online`, and tool `read_file` with its description

#### Scenario: `agis mcp list` marks disabled servers
- GIVEN a server configured with `disabled: true`
- WHEN `agis mcp list` is executed
- THEN output lists the server with status `disabled` without attempting connection

#### Scenario: `agis mcp test` executes tool directly
- GIVEN configured server `echo-server` with tool `echo`
- WHEN `agis mcp test echo-server echo '{"msg": "ping"}'` is executed
- THEN tool executes via MCP client, prints output `"ping"`, and exits with code 0

#### Scenario: `agis mcp test` handles non-existent server gracefully
- GIVEN an invalid server name `"unknown-server"`
- WHEN `agis mcp test unknown-server echo` is executed
- THEN an error `"server unknown-server not found in configuration"` is printed and command exits with non-zero code
