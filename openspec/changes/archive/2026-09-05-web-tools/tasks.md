## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1200 - 1800 additions across 12 files |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Config & Secret Masking) → PR 2 (Web Search Subsystem) → PR 3 (Web Fetch & HTML-to-Markdown) → PR 4 (Registry, PolicyGuard & Doctor Probes) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

## Work Units

### Unit 1: Configuration & Secret Masking
- [x] **RED (Test)**: Write unit tests in `internal/config/config_test.go` and `internal/config/mask_test.go` asserting `WebConfig` loading, default values (`default_provider: "duckduckgo"`, `max_fetch_bytes: 2097152`, `fetch_timeout: 15s`), and secret masking of API keys (`tools.web.providers.*.api_key` -> `[MASKED]`). <!-- sdd-owner: implementation -->
- [x] **GREEN (Impl)**: Implement `WebConfig` struct and defaults in `internal/config/config.go` and secret masking in `internal/config/mask.go`. <!-- sdd-owner: implementation -->
- [x] **REFACTOR & VERIFY**: Run tests with `-race` and verify dot-notation accessors in `internal/config/accessor.go`. <!-- sdd-owner: implementation -->

### Unit 2: Web Search Subsystem
- [x] **RED (Test)**: Write table-driven unit tests in `internal/tools/web/search/search_test.go` using `httptest.Server` mocking Brave, Tavily, SearXNG, and DuckDuckGo endpoints, asserting query validation, timeout handling, and result parsing. <!-- sdd-owner: implementation -->
- [x] **GREEN (Impl)**: Implement `Searcher` interface, request structs, and providers (`BraveSearcher`, `TavilySearcher`, `SearXNGSearcher`, `DuckDuckGoSearcher`) in `internal/tools/web/search/`. <!-- sdd-owner: implementation -->
- [x] **REFACTOR & VERIFY**: Verify `goleak` safety and test coverage across search providers. <!-- sdd-owner: implementation -->

### Unit 3: Web Fetch & HTML-to-Markdown Extractor
- [x] **RED (Test)**: Write unit tests in `internal/tools/web/fetch/extractor_test.go` and `client_test.go` covering HTML tag stripping, Markdown mapping (`h1`-`h6`, `p`, `a`, `code`, lists), SSRF protection, size guard (`io.LimitReader`), and binary `Content-Type` rejection. <!-- sdd-owner: implementation -->
- [x] **GREEN (Impl)**: Implement HTML-to-Markdown converter using `golang.org/x/net/html` and safe HTTP client in `internal/tools/web/fetch/`. <!-- sdd-owner: implementation -->
- [x] **REFACTOR & VERIFY**: Ensure 2MB size limit and timeout boundaries work under load and test with `-race`. <!-- sdd-owner: implementation -->

### Unit 4: Tool Runner Bridge & PolicyGuard Registration
- [x] **RED (Test)**: Write unit tests in `internal/tools/` asserting `web_search` and `web_fetch` execution, input validation (empty query, unsupported URL scheme), and `PolicyGuard` evaluation under `sandbox`, `standard`, and `full` security tiers. <!-- sdd-owner: implementation -->
- [x] **GREEN (Impl)**: Implement `WebSearchRunner` (`internal/tools/web_search.go`) and `WebFetchRunner` (`internal/tools/web_fetch.go`), and register both runners in `internal/tools/registry.go`. <!-- sdd-owner: implementation -->
- [x] **REFACTOR & VERIFY**: Verify audit trail logging and security posture rules. <!-- sdd-owner: implementation -->

### Unit 5: Health Check Probe & Documentation
- [x] **RED (Test)**: Write unit tests in `internal/doctor/web_test.go` asserting `checkWebTools` probe output when web tools are disabled, enabled with valid config, or missing required API keys. <!-- sdd-owner: implementation -->
- [x] **GREEN (Impl)**: Implement `checkWebTools` probe in `internal/doctor/web.go` and register it in `internal/doctor/doctor.go`. Update documentation in `docs/tools.md` or `README.md`. <!-- sdd-owner: implementation -->
- [x] **REFACTOR & VERIFY**: Run full test suite across the project with `go test -race ./...`. <!-- sdd-owner: implementation -->
