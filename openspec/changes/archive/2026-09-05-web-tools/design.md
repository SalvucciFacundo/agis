# Architecture and Design: Web Search & Fetch Tools (web-tools)

## 1. Executive Summary
This document details the architectural decisions, component design, data structures, and security considerations for introducing native `web_search` and `web_fetch` capabilities in the `agis` project. These tools rely exclusively on standard Go libraries and `golang.org/x/net/html`, avoiding headless browsers and CGO dependencies while maintaining a strict security posture.

## 2. Architecture Decision Records (ADRs)

*   **D1: Package Layout**
    *   **Decision**: Isolate the domain in `internal/tools/web/`. Provide subpackages `search` (for multi-provider search logic) and `fetch` (for HTML fetching and Markdown extraction) to keep responsibilities bounded. Tool registration remains in `internal/tools/registry.go`.
*   **D2: HTML Parsing and Markdown Conversion**
    *   **Decision**: Use `golang.org/x/net/html` for pure Go node traversal.
    *   **Rationale**: Prevents memory bloat and avoids binary dependencies (like Puppeteer or CGO wrappers). We will walk the AST, ignoring non-content tags (`script`, `style`, `nav`) and format valid content nodes (`h1-h6`, `p`, `a`, `code`) into standard Markdown.
*   **D3: Search Provider Factory Pattern**
    *   **Decision**: Expose a `Searcher` interface in `internal/tools/web/search`. Implementations for Brave, Tavily, SearXNG, and DuckDuckGo (lite fallback) will be instantiated via a factory based on the active `internal/config`.
*   **D4: Safe HTTP Client Design**
    *   **Decision**: Utilize a strict HTTP client with `FetchTimeout` (default 15s), `CheckRedirect` (capped at 10 to prevent downgrade/loops), and wrap all response bodies in `io.LimitReader(resp.Body, maxBytes)` (capped at 2MB).
*   **D5: PolicyGuard Integration**
    *   **Decision**: Map both tools to the `"web"` backend and `core.CategoryNetwork` category. Enforce standard security tiers (`sandbox`, `standard`, `full`). Log every access via `AuditEntry`.
*   **D6: Configuration Integration**
    *   **Decision**: Add a `WebConfig` struct to `Config.Tools.Web` in `internal/config/config.go`. API keys must use secret masking for logging safety.
*   **D7: Health Diagnostic Probe**
    *   **Decision**: Implement a `checkWebTools` probe in `internal/doctor` to assert configuration validity and network connectivity for the default provider.

## 3. Component Interactions

### 3.1. Web Search Execution Flow
1. **Agent** invokes `web_search` with arguments `{"query": "golang 1.26", "max_results": 5}`.
2. **Tools Registry** intercepts via `PolicyGuard`.
3. **PolicyGuard** assesses the `"web"` category against the current security tier (`ask`/`allow`/`deny`).
4. On allow, the request routes to `internal/tools/web/search.Runner`.
5. **Searcher Factory** initializes the configured provider (e.g., DuckDuckGo).
6. **HTTP Client** executes the remote query with a context timeout.
7. Results are mapped to `[]SearchResult` and returned as a JSON/Text snippet array.

### 3.2. Web Fetch Execution Flow
1. **Agent** invokes `web_fetch` for `https://example.com/article`.
2. **PolicyGuard** grants execution.
3. **HTTP Client** in `fetch` requests the URL with the custom `User-Agent`.
4. Response headers are validated (reject binary `Content-Type`).
5. `io.LimitReader(resp.Body, 2MB)` restricts the payload size.
6. `html.Parse` parses the bounded stream into an AST.
7. `extractor.ToMarkdown(node)` walks the AST, stripping boilerplate and returning Markdown.

## 4. Data Structures & Interfaces

### 4.1. Configuration (`internal/config/config.go`)
```go
type WebConfig struct {
    Enabled         bool          `json:"enabled" yaml:"enabled"`
    DefaultProvider string        `json:"default_provider" yaml:"default_provider"` // e.g., "duckduckgo"
    FetchTimeout    time.Duration `json:"fetch_timeout" yaml:"fetch_timeout"`       // default: 15s
    MaxFetchBytes   int64         `json:"max_fetch_bytes" yaml:"max_fetch_bytes"`   // default: 2097152 (2MB)
    UserAgent       string        `json:"user_agent" yaml:"user_agent"`             // default: AGIS/...
    Providers       WebProviders  `json:"providers" yaml:"providers"`
}

type WebProviders struct {
    BraveAPIKey  string `json:"brave_api_key" yaml:"brave_api_key"` // Masked in logs
    TavilyAPIKey string `json:"tavily_api_key" yaml:"tavily_api_key"`
    SearxngURL   string `json:"searxng_url" yaml:"searxng_url"`
}
```

### 4.2. Search Package (`internal/tools/web/search`)
```go
// SearchRequest represents a normalized query
type SearchRequest struct {
    Query      string
    MaxResults int
    Provider   string // Optional override
}

// SearchResult represents a single engine result
type SearchResult struct {
    Title   string `json:"title"`
    URL     string `json:"url"`
    Snippet string `json:"snippet"`
}

// Searcher is the provider abstraction
type Searcher interface {
    Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
}
```

### 4.3. Fetch Package (`internal/tools/web/fetch`)
```go
// FetchOptions dictates fetching rules
type FetchOptions struct {
    Timeout   time.Duration
    MaxBytes  int64
    UserAgent string
}

// Fetcher handles the HTTP lifecycle and extraction
type Fetcher struct {
    client *http.Client
    opts   FetchOptions
}

func NewFetcher(opts FetchOptions) *Fetcher
func (f *Fetcher) FetchMarkdown(ctx context.Context, targetURL string) (string, error)

// Extractor logic
func ExtractMarkdown(r io.Reader) (string, error)
```

## 5. Security & Threat Modeling

According to STRIDE and Defense-in-Depth principles, the web tools expose the system to external threats.
- **SSRF Defense**: 
  - `web_fetch` must resolve the target URL and reject private IP ranges (`10.0.0.0/8`, `127.0.0.0/8`, `192.168.0.0/16`, `172.16.0.0/12`) to prevent scanning internal networks.
  - Redirects must be capped (maximum 10), and the redirect policy (`CheckRedirect`) must enforce protocol preservation (e.g., block `http` -> `file://` or `gopher://` downgrades).
- **Resource/Memory Exhaustion**:
  - Unbounded reads cause OOMs. The HTTP response body must immediately be wrapped: `limitReader := io.LimitReader(resp.Body, config.MaxFetchBytes)`.
  - The AST parser will consume the `limitReader`. If EOF isn't reached but `LimitReader` hits its capacity limit, we parse what we have or return an error.
- **Rate Limiting & Tarpits**: 
  - Strict `context.WithTimeout` on all outbound requests. If a server acts as a tarpit (drip-feeding 1 byte per second), the context timeout (15s) breaks the connection.
- **Error Leakage**:
  - Detailed errors (e.g., DNS lookup failures or internal stack traces from `net/http`) should be sanitized when returned to the agent, providing only actionable feedback like "Target unreachable" or "Timeout exceeded".

## 6. Testing Strategy

Strict TDD Mode is enabled. Adhere to `golang-testing` skill constraints.

1. **Table-Driven Tests for Extractor (`gotests`)**:
   - `ExtractMarkdown` must be tested using raw HTML string fixtures.
   - Cases must assert correct stripping of `<script>`, `<style>`, `<nav>`, `<header>`.
   - Cases must assert proper mapping of `<h1>` to `#`, `<p>` spacing, `<a>` and `<code>` tags.
2. **Mock HTTP Servers (`httptest.Server`)**:
   - Both `web_search` providers and `web_fetch` clients must be tested against an `httptest.Server` to avoid hitting live endpoints during unit testing.
   - Test cases: Normal response, Timeout (server sleeps > `FetchTimeout`), Size Limit (server streams 5MB, client must truncate and close at 2MB).
3. **Leak Checks (`goleak`)**:
   - Include `goleak.VerifyTestMain(m)` in `TestMain` for both `search` and `fetch` packages to ensure no goroutines are leaked by HTTP clients.
4. **Race Detector (`-race`)**:
   - Test suites must run with `-race` to ensure concurrent fetches (if spawned by agent) do not step on shared state (e.g., modifying the HTTP client instance or shared configuration).
