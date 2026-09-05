# Specification: Native Web Search and Content Extraction Tools (web-tools)

## Purpose

Define the functional requirements, architectural boundaries, data contracts, and security policies for native web searching (`web_search`) and web page content extraction (`web_fetch`) in AGIS (`agis`). These capabilities operate in pure Go without headless browsers or heavyweight CGO dependencies, providing resilient real-time web intelligence governed by `PolicyGuard`, configurable through `internal/config`, and verifiable through `internal/doctor`.

---

## Tool Contracts & Wire Format

### Requirement WEB-TOOL-001: `web_search` Tool Contract
The system MUST provide a `web_search` tool implementing the `core.ToolRunner` interface with backend identifier `"web"`.
- **Tool Name**: `"web_search"`
- **Backend**: `"web"`
- **Description**: `"Search the web for real-time information, documentation, news, and technical references. Returns top search results containing title, URL, and snippet."`
- **Input Parameters (JSON schema)**:
  - `query` (string, required): The search query string. MUST NOT be empty or whitespace-only.
  - `max_results` (integer, optional): Maximum number of search results to return. Default is `5`. Allowed range MUST be clamped to `[1, 20]`.
  - `provider` (string, optional): Search provider override (`"brave"`, `"tavily"`, `"searxng"`, `"duckduckgo"`). If omitted or empty, the configured `default_provider` MUST be used.
- **Output Format**:
  - The tool MUST return a JSON array of search result objects or structured text formatted for LLM ingestion.
  - Each search result item MUST contain:
    - `title` (string): Title of the web page or document.
    - `url` (string): Canonical destination URL (MUST be valid `http` or `https` URI).
    - `snippet` (string): Text excerpt summarizing the result.
- **Error Handling**:
  - If `query` is empty or whitespace-only, the tool MUST return an error indicating that `query` is required.
  - If all search providers fail or if no results are found, the tool MUST return a clear message indicating zero results rather than crashing.

#### Scenario: Successful web search with default provider
- GIVEN a valid search query `"golang 1.26 release notes"`
- WHEN `web_search` is executed with arguments `{"query": "golang 1.26 release notes", "max_results": 3}`
- THEN the tool executes the search against the default provider
- AND returns a formatted list of 3 items containing `title`, `url`, and `snippet`

#### Scenario: Web search with explicit provider override
- GIVEN `web_search` is invoked with arguments `{"query": "modernc sqlite fts5", "provider": "duckduckgo", "max_results": 5}`
- WHEN the tool executes
- THEN it routes the search request to DuckDuckGo regardless of the configured default provider
- AND returns the corresponding search results

#### Scenario: Web search with empty query
- GIVEN `web_search` is invoked with arguments `{"query": "   "}`
- WHEN the tool executes
- THEN the tool returns an error stating that the search query cannot be empty

---

### Requirement WEB-TOOL-002: `web_fetch` Tool Contract
The system MUST provide a `web_fetch` tool implementing the `core.ToolRunner` interface with backend identifier `"web"`.
- **Tool Name**: `"web_fetch"`
- **Backend**: `"web"`
- **Description**: `"Fetch a web page by URL and extract its main readable text content converted to clean Markdown. Strips navigation, scripts, styles, and boilerplate."`
- **Input Parameters (JSON schema)**:
  - `url` (string, required): The target HTTP or HTTPS URL to fetch. MUST have a valid `http://` or `https://` scheme.
  - `max_bytes` (integer, optional): Maximum response size in bytes before truncation. Default is `2097152` (2MB). Maximum allowed is `10485760` (10MB).
  - `raw` (boolean, optional): When `true`, returns the raw response body (HTML/text/JSON) without Markdown conversion. Default is `false`.
- **Output Format**:
  - By default (`raw: false`), the tool MUST return cleaned Markdown representing the main content of the web page.
  - When `raw: true`, the tool MUST return the raw payload decoded as a UTF-8 string.
- **Error Handling**:
  - If `url` is missing, malformed, or contains an unsupported scheme (e.g. `file://`, `ftp://`), the tool MUST return a descriptive validation error.
  - If the target host is unreachable, times out, or returns an HTTP 4xx/5xx status code, the tool MUST return an error containing the status code and error description.

#### Scenario: Successful web fetch converted to Markdown
- GIVEN a valid public URL `"https://example.com/article"` containing standard HTML with headings, paragraphs, and links
- WHEN `web_fetch` is executed with arguments `{"url": "https://example.com/article"}`
- THEN the tool retrieves the HTML, strips boilerplate elements (`<nav>`, `<script>`, `<style>`), converts headings and text to Markdown, and returns the Markdown string

#### Scenario: Web fetch with raw format requested
- GIVEN a public JSON or raw text endpoint `"https://api.example.com/status"`
- WHEN `web_fetch` is executed with arguments `{"url": "https://api.example.com/status", "raw": true}`
- THEN the tool retrieves the raw response and returns the unparsed payload string without Markdown transformation

#### Scenario: Web fetch with invalid URL scheme
- GIVEN `web_fetch` is executed with arguments `{"url": "file:///etc/passwd"}`
- WHEN the tool validates the input
- THEN the tool rejects the request with an error indicating unsupported URL scheme `"file"`

---

## Provider Specifications (`internal/tools/web/search`)

### Requirement WEB-PROV-001: Multi-Provider Searcher Interface
The search subsystem in `internal/tools/web/search` MUST define a unified `Searcher` interface.
```go
type SearchOptions struct {
    MaxResults int
    Timeout    time.Duration
}

type SearchResult struct {
    Title   string `json:"title"`
    URL     string `json:"url"`
    Snippet string `json:"snippet"`
}

type Searcher interface {
    Name() string
    Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}
```

The system MUST implement four concrete search providers:
1. **Brave Search API (`BraveSearcher`)**:
   - Endpoint: `https://api.search.brave.com/res/v1/web/search`
   - Authentication: HTTP header `X-Subscription-Token: <api_key>`
   - Query parameters: `q=<query>`, `count=<max_results>`
   - Result parsing: Extract from `web.results[]` fields `title`, `url`, `description`.
   - Error handling: Returns authentication error if API key is missing or invalid (HTTP 401/403).
2. **Tavily API (`TavilySearcher`)**:
   - Endpoint: `https://api.tavily.com/search`
   - Authentication & Payload: JSON POST body `{"api_key": "<api_key>", "query": "<query>", "max_results": <max_results>}`
   - Result parsing: Extract from `results[]` fields `title`, `url`, `content`.
   - Error handling: Returns error on non-200 responses with API error message.
3. **SearXNG (`SearXNGSearcher`)**:
   - Endpoint: Configurable `base_url` (e.g. `http://localhost:8080/search` or public instance)
   - Parameters: `q=<query>&format=json`
   - Result parsing: Extract from `results[]` fields `title`, `url`, `content`.
   - Error handling: Handles connection failures and non-JSON responses gracefully.
4. **DuckDuckGo (`DuckDuckGoSearcher`)**:
   - Zero-configuration / No API key required.
   - Mechanism: Fetch DuckDuckGo HTML/lite interface (`https://html.duckduckgo.com/html/?q=...` or `https://lite.duckduckgo.com/lite/`) with standard browser User-Agent.
   - Result parsing: Pure Go token/node HTML extraction of result links, titles, and snippets.
   - Fallback resilience: Respects rate limit responses (HTTP 202/429) and returns actionable error message.

#### Scenario: Brave Search executes with valid API key
- GIVEN Brave Search is configured with a valid API key
- WHEN a search request for `"sqlite fts5 ranking"` is dispatched
- THEN the HTTP request includes the `X-Subscription-Token` header
- AND parsed results contain populated `Title`, `URL`, and `Snippet` fields

#### Scenario: Brave Search handles missing API key
- GIVEN Brave Search is selected as provider but `api_key` is empty
- WHEN a search is attempted
- THEN `BraveSearcher` immediately returns an error indicating that Brave Search API key is not configured

#### Scenario: DuckDuckGo search executes without credentials
- GIVEN DuckDuckGo provider is selected with no API keys configured
- WHEN a search query `"open source ai agents"` is dispatched
- THEN `DuckDuckGoSearcher` sends an HTTP GET to the HTML search endpoint
- AND parses HTML result elements into structured `SearchResult` items

---

## Extractor Specifications (`internal/tools/web/fetch`)

### Requirement WEB-EXTR-001: Safe HTTP Fetcher & Size Guard
The HTTP client in `internal/tools/web/fetch` MUST enforce strict security and resource bounds:
1. **User-Agent Header**: Every HTTP request MUST set a configurable `User-Agent` header (default: `"AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"`).
2. **Size Guard**: The fetcher MUST wrap the response stream with `io.LimitReader(resp.Body, maxBytes)` where `maxBytes` is at most `2097152` (2MB) by default. The fetcher MUST NOT buffer unbounded streams into memory.
3. **Timeout Enforcement**: Every fetch operation MUST respect `context.Context` cancellation and a configurable `FetchTimeout` (default: 15 seconds).
4. **Redirect Control**: The HTTP client MUST follow at most 10 redirects and MUST prevent protocol downgrade or redirection to unsafe non-HTTP schemes.
5. **Content-Type Validation**: The fetcher MUST inspect the `Content-Type` header and reject binary payloads (such as `application/octet-stream`, `application/zip`, `audio/*`, `video/*`, `image/*` when not requested) with an informative error stating that binary content cannot be rendered as text.

#### Scenario: Response exceeds 2MB size limit
- GIVEN a web page or file of size 10MB
- WHEN `web_fetch` requests the URL with default `max_bytes: 2097152`
- THEN the fetcher reads at most 2MB via `io.LimitReader`
- AND processes the bounded 2MB slice without memory exhaustion or runtime panic

#### Scenario: Fetch request times out
- GIVEN a slow or unresponsive web server that does not respond within `FetchTimeout`
- WHEN `web_fetch` is invoked
- THEN the request is aborted and returns a context deadline exceeded error

---

### Requirement WEB-EXTR-002: Pure Go HTML-to-Markdown Converter
The extraction package in `internal/tools/web/fetch` MUST convert HTML documents to readable Markdown using standard Go and `golang.org/x/net/html` without external binary dependencies.
1. **Boilerplate Stripping**: The converter MUST strip and discard:
   - `<script>`, `<style>`, `<noscript>`, `<template>`
   - `<nav>`, `<header>`, `<footer>`, `<aside>`
   - `<svg>`, `<canvas>`, `<iframe>`, `<form>`
2. **Semantic Element Mapping**:
   - `<h1>` through `<h6>` -> `# ` through `###### ` prefixed lines.
   - `<p>` -> Text blocks separated by blank lines.
   - `<a>` with `href` -> `[anchor text](href)`. If anchor text is empty and `href` is present, use `[href](href)`. If `href` is empty, render anchor text only.
   - `<ul>` and `<ol>` with `<li>` -> Bulleted list items (`- `) or numbered list items (`1. `).
   - `<code>` inline -> `` `code` ``.
   - `<pre>` and `<pre><code>` -> Multi-line fenced code blocks (```` ``` ````).
   - `<strong>`, `<b>` -> `**text**`.
   - `<em>`, `<i>` -> `*text*`.
   - `<blockquote>` -> `> text` lines.
   - `<br>` and `<hr>` -> Newline and `---` respectively.
3. **Whitespace Normalization**:
   - Multiple consecutive whitespace characters within text nodes MUST be collapsed into a single space.
   - Multiple consecutive blank lines MUST be collapsed into at most two newlines (`\n\n`).
   - Leading and trailing whitespace of the final output MUST be trimmed.

#### Scenario: HTML with navigation, script, and article content
- GIVEN HTML input:
  ```html
  <html>
    <head><script>alert('xss');</script><style>body{color:red;}</style></head>
    <body>
      <nav><a href="/">Home</a><a href="/about">About</a></nav>
      <h1>Release Notes</h1>
      <p>AGIS now supports <strong>web tools</strong>. See the <a href="https://example.com/docs">documentation</a>.</p>
      <pre><code>go build ./...</code></pre>
      <footer>Copyright 2026</footer>
    </body>
  </html>
  ```
- WHEN the HTML is converted to Markdown
- THEN the output contains:
  ```markdown
  # Release Notes

  AGIS now supports **web tools**. See the [documentation](https://example.com/docs).

  ```
  go build ./...
  ```
  ```
- AND script, style, nav, and footer contents are completely omitted.

#### Scenario: Nested lists and inline formatting
- GIVEN an HTML fragment with ordered lists and nested bold/code tags
- WHEN converted to Markdown
- THEN list numbers/bullets and nested inline formatting are preserved in correct Markdown syntax

---

## PolicyGuard & Security Governance

### Requirement WEB-POL-001: PolicyGuard Integration & Tiers
All invocations of `web_search` and `web_fetch` MUST be evaluated by `PolicyGuard` prior to network execution.
- **Backend Identifier**: `"web"`
- **Category**: `core.CategoryNetwork` (`"network"`)
- **Subject**:
  - For `web_search`: `"web_search:<provider>:<query>"` or the query string.
  - For `web_fetch`: The destination URL host or full URL (e.g., `"web_fetch:https://example.com/page"`).
- **Security Posture Rules**:
  - `PostureSandbox` (`"sandbox"`):
    - By default, network operations under sandbox require explicit user confirmation (`DecisionAsk`) or an explicit allow rule in policy store.
    - If a rule `allow network web *` exists, the operation is permitted (`DecisionAllow`).
  - `PostureStandard` (`"standard"`):
    - Returns `DecisionAsk` on first encounter unless an existing allow rule or session grant matches.
    - Supports interactive user approval (grant once, session, always).
  - `PostureFull` (`"full"`):
    - Returns `DecisionAllow` automatically for all web operations.
- **Audit Logging**:
  - Every evaluation outcome (`allow`, `deny`, `ask`) MUST generate an `AuditEntry` recorded in the audit trail with timestamp, backend `"web"`, category `"network"`, subject, and decision.

#### Scenario: Web fetch under standard posture without prior grant
- GIVEN policy tier for backend `"web"` is `"standard"` and no prior rules exist
- WHEN `web_fetch` is called for `"https://example.com"`
- THEN `PolicyGuard.Evaluate` returns `DecisionAsk`
- AND prompts the user for approval
- AND logs an audit entry for the evaluation

#### Scenario: Web search under sandbox posture with allow rule
- GIVEN policy tier is `"sandbox"` and a policy rule allows `network web *`
- WHEN `web_search` is called
- THEN `PolicyGuard.Evaluate` returns `DecisionAllow`
- AND search executes without interactive prompting

---

## Configuration (`internal/config`)

### Requirement WEB-CFG-001: Web Configuration Schema & Defaults
The configuration subsystem MUST include a `WebConfig` struct within `Config.Tools.Web` (or root `Config.Web`).
```go
type WebConfig struct {
    Enabled         bool                      `yaml:"enabled"`
    DefaultProvider string                    `yaml:"default_provider"` // "duckduckgo", "brave", "tavily", "searxng"
    Providers       map[string]ProviderConfig `yaml:"providers"`
    FetchTimeout    time.Duration             `yaml:"fetch_timeout"`   // default: 15s
    MaxFetchBytes   int64                     `yaml:"max_fetch_bytes"` // default: 2097152 (2MB)
    UserAgent       string                    `yaml:"user_agent"`      // default: "AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"
}

type ProviderConfig struct {
    APIKey  string        `yaml:"api_key"`
    BaseURL string        `yaml:"base_url,omitempty"`
    Timeout time.Duration `yaml:"timeout,omitempty"`
}
```

- **Built-in Defaults**:
  - `Enabled`: `false` (opt-in security default; enabled when tools are active and configured)
  - `DefaultProvider`: `"duckduckgo"`
  - `FetchTimeout`: `15 * time.Second`
  - `MaxFetchBytes`: `2097152` (2MB)
  - `UserAgent`: `"AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"`
- **Secret Masking (`internal/config/mask.go`)**:
  - `MaskSecrets` MUST mask all `api_key` values in `WebConfig.Providers` with `"[MASKED]"`.
- **Accessor Support (`internal/config/accessor.go`)**:
  - `Get` and `Set` MUST support dot-notation keys such as `tools.web.default_provider`, `tools.web.providers.brave.api_key`, `tools.web.fetch_timeout`, etc.

#### Scenario: Default configuration initialization
- GIVEN no configuration file exists
- WHEN `config.Load` or default config is initialized
- THEN `WebConfig` defaults to `DefaultProvider: "duckduckgo"`, `MaxFetchBytes: 2097152`, and `FetchTimeout: 15s`

#### Scenario: Secret masking of web provider API keys
- GIVEN a configuration containing `tools.web.providers.brave.api_key: "BSA_secret_key_123"`
- WHEN `config.MaskSecrets` is called
- THEN the returned config copy contains `tools.web.providers.brave.api_key: "[MASKED]"`

---

## Tool Registration & Lifecycle (`internal/tools/registry.go`)

### Requirement WEB-REG-001: Tool Registration in Registry
The tools factory in `internal/tools/registry.go` MUST instantiate and register `web_search` and `web_fetch` runners when web tools are enabled.
- If `cfg.Tools.Enabled` is `true` and `cfg.Tools.Web.Enabled` is `true`:
  - Create the searcher according to `cfg.Tools.Web.DefaultProvider` and configured providers map.
  - Create `WebSearchRunner` with the searcher and default options.
  - Create `WebFetchRunner` with the HTTP client, size limit, and HTML-to-Markdown converter.
  - Append both runners to the active `[]core.ToolRunner` list.
- If `cfg.Tools.Web.Enabled` is `false`, web tools MUST NOT be registered and remain inactive.

#### Scenario: Tools enabled with web tools active
- GIVEN `ToolsConfig.Enabled` is `true` and `WebConfig.Enabled` is `true`
- WHEN `tools.Select` is invoked
- THEN the returned runners slice includes `web_search` and `web_fetch` runners

#### Scenario: Web tools disabled
- GIVEN `WebConfig.Enabled` is `false`
- WHEN `tools.Select` is invoked
- THEN neither `web_search` nor `web_fetch` runners are included in the returned slice

---

## Diagnostic Probes (`internal/doctor`)

### Requirement WEB-DOC-001: Doctor Web Tools Health Probe
The diagnostic suite in `internal/doctor` MUST include a `checkWebTools` probe.
- **Probe Name**: `"web_tools"`
- **Probe Title**: `"Web Search & Content Extraction Tools"`
- **Behavior**:
  - If `WebConfig.Enabled` is `false`:
    - Status: `StatusPass`
    - Message: `"Web tools disabled"`
  - If `WebConfig.Enabled` is `true`:
    - Check default provider configuration:
      - If provider requires an API key (e.g. `brave`, `tavily`) and `api_key` is missing:
        - Status: `StatusWarn` (or `StatusFail` if default provider cannot function)
        - Message: `"Default web search provider '<provider>' requires an API key that is not configured"`
      - If provider is `duckduckgo` or API key is present:
        - Status: `StatusPass`
        - Message: `"Web tools enabled (default provider: <provider>)"`
        - Details: Include configured fetch timeout, max bytes limit, and registered providers list.

#### Scenario: Doctor check when web tools are disabled
- GIVEN `cfg.Tools.Web.Enabled` is `false`
- WHEN `doctor.Run(ctx)` executes
- THEN the `"web_tools"` check returns `StatusPass` with message `"Web tools disabled"`

#### Scenario: Doctor check when Brave search is default but missing API key
- GIVEN `cfg.Tools.Web.Enabled` is `true`, `DefaultProvider` is `"brave"`, but Brave API key is empty
- WHEN `doctor.Run(ctx)` executes
- THEN the `"web_tools"` check returns `StatusWarn` or `StatusFail` with instructions to configure `tools.web.providers.brave.api_key`
