# Web Search & Fetch Tools (web-tools)

## Intent
Introduce native web searching (`web_search`) and content extraction (`web_fetch`) capabilities in pure Go. This will allow the system to gather real-time information from the web safely, without relying on external headless browsers or heavy dependencies.

## Scope
1. **Web Search**:
   - Multi-provider client abstraction supporting Brave Search API, Tavily API, SearXNG, and DuckDuckGo (HTML scraping/lite API fallback).
   - Configurable defaults: provider choice, API keys, maximum results per query, and request timeouts.
2. **Web Fetch & Content Extraction**:
   - Safe HTTP client featuring a custom User-Agent, maximum response size guards (e.g., hard limit of 2MB to prevent memory exhaustion), and strict timeouts.
   - Pure Go HTML-to-Markdown extraction logic that intelligently strips non-content tags (`<script>`, `<style>`, `<nav>`, `<footer>`, `<header>`) while preserving main text, headings, links, lists, and code blocks.
3. **Tool Integration & Security**:
   - Implement the `core.ToolRunner` interface for both `web_search` and `web_fetch`.
   - Register the new tools in `tools.Registry` alongside existing backends (local/docker/ssh).
   - Integrate with `PolicyGuard`: enforce configurable security tiers (`sandbox`, `standard`, `full`), default permissions, and ensure comprehensive audit logging of all external requests.
4. **Configuration & Diagnostics**:
   - Expand `internal/config/config.go` with a new `web` configuration block (providers, keys, limits).
   - Add a diagnostic probe in `internal/doctor` to verify web tools availability, API key presence, and provider connectivity.

## Affected Areas
- `internal/tools/` (new implementations for `web_search` and `web_fetch`, updates to `Registry`)
- `internal/core/` (interfaces and `PolicyGuard` integration)
- `internal/config/` (new `web` config struct and defaults)
- `internal/doctor/` (new health check probes)

## Risks
- **Resource Exhaustion**: Fetching infinitely streaming endpoints or massive files. Mitigated by a strict 2MB response size limit and context timeouts.
- **Provider Rate Limits**: API exhaustion or IP bans from scraping. Mitigated by configurable limits and clear error propagation.
- **Content Quality**: Pure Go parsers may struggle with JS-rendered Single Page Applications (SPAs). This is an accepted limitation; the focus is on robust static HTML extraction.

## Rollback
- Disable web tools globally via a configuration flag in the `web` block, removing them from the `tools.Registry` at initialization.

## Success Criteria
- Agent can successfully execute `web_search` against the default provider and receive structured results.
- Agent can successfully execute `web_fetch` to retrieve a web page and convert it cleanly to Markdown, with the 2MB size limit strictly enforced.
- Tool usage is governed by `PolicyGuard` according to the active security tier and is audit-logged.
- Running the `doctor` command reports the status of web tools configuration and API connectivity.
