# Apply Progress: Web Tools (web-tools) - Complete (Units 1-5)

## Completed Tasks

### Unit 1: Configuration & Secret Masking
- [x] **RED (Test)**: Added unit tests in `internal/config/config_test.go`, `internal/config/mask_test.go`, and `internal/config/accessor_test.go` asserting `WebConfig` loading, default values (`default_provider: "duckduckgo"`, `max_fetch_bytes: 2097152`, `fetch_timeout: 15s`, `user_agent: "AGIS/1.0 (+https://github.com/SalvucciFacundo/agis)"`), and secret masking of API keys (`tools.web.providers.*.api_key` -> `[MASKED]`).
- [x] **GREEN (Impl)**: Implemented `WebConfig`, `WebProviders`, and `ProviderConfig` structs and defaults in `internal/config/config.go` and secret masking in `internal/config/mask.go`.
- [x] **REFACTOR & VERIFY**: Verified dot-notation accessors (`tools.web.*`) in `internal/config/accessor.go` and verified 100% test pass with `-race`.

### Unit 2: Web Search Subsystem
- [x] **RED (Test)**: Wrote table-driven unit tests in `internal/tools/web/search/search_test.go` using `httptest.Server` mocking Brave, Tavily, SearXNG, and DuckDuckGo endpoints, asserting query validation (empty queries rejected), timeout handling, and result parsing.
- [x] **GREEN (Impl)**: Implemented `Searcher` interface, options, result structs, and concrete searchers:
  - `BraveSearcher` (Brave Search API with token auth and JSON parsing)
  - `TavilySearcher` (Tavily POST API with JSON payload and parsing)
  - `SearXNGSearcher` (SearXNG JSON endpoint parsing)
  - `DuckDuckGoSearcher` (DuckDuckGo HTML / lite endpoint scraping without API keys)
  - `NewSearcher` factory.
- [x] **REFACTOR & VERIFY**: Verified `goleak.VerifyTestMain` safety, zero goroutine leaks, zero race conditions with `go test -race -count=1 ./internal/tools/web/search/...`.

### Unit 3: Web Fetch & HTML-to-Markdown Extractor
- [x] **RED (Test)**: Wrote comprehensive unit tests in `internal/tools/web/fetch/extractor_test.go`, `ssrf_test.go`, and `fetch_test.go` covering HTML tag stripping, Markdown mapping (`h1`-`h6`, `p`, `a`, `code`, lists, `blockquote`, `strong`, `em`, `hr`, `br`), SSRF protection, size guard (`io.LimitReader`), binary `Content-Type` rejection, raw mode, redirect limits, and `goleak.VerifyTestMain`.
- [x] **GREEN (Impl)**: Implemented:
  - `internal/tools/web/fetch/ssrf.go`: `IsPrivateIP`, `ValidateURL`, and `NewSafeTransport` with SSRF dialer and DNS resolution validation.
  - `internal/tools/web/fetch/extractor.go`: Pure Go HTML parser and Markdown serializer using `golang.org/x/net/html`.
  - `internal/tools/web/fetch/fetch.go`: `Fetcher` struct, `FetchOptions`, `Fetch`, and `FetchMarkdown` methods with safe `http.Client`, redirect limits, size guards, and content type validation.
- [x] **REFACTOR & VERIFY**: Verified 2MB size limit, timeout boundaries, zero goroutine leaks, and clean execution under `go test -race -count=1 ./internal/tools/web/fetch/...`.

### Unit 4: Tool Runner Bridge & PolicyGuard Registration
- [x] **RED (Test)**: Wrote unit tests in `internal/tools/web_search_test.go`, `internal/tools/web_fetch_test.go`, `internal/tools/registry_test.go`, `internal/policy/guard_test.go`, and `internal/core/brain_tools_test.go` asserting schema, input parsing (`query`, `max_results`, `provider`, `url`, `max_bytes`, `raw`), output formatting, error handling, and `PolicyGuard` evaluation under `sandbox`, `standard`, and `full` tiers.
- [x] **GREEN (Impl)**:
  - Implemented `internal/tools/web_search.go` (`WebSearchRunner`).
  - Implemented `internal/tools/web_fetch.go` (`WebFetchRunner`).
  - Added `FromWebConfig` and registered runners in `internal/tools/registry.go`.
  - Updated `internal/policy/guard.go` and `internal/policy/store.go` (`matchPattern`) to support `network` category, wildcard pattern matching, and `web` backend tier rules.
  - Wired `web` backend and `CategoryNetwork` in `internal/core/brain.go`.
- [x] **REFACTOR & VERIFY**: Verified audit trail logging and security posture rules with `-race`.

### Unit 5: Health Check Probe, Main Wiring & Documentation
- [x] **RED (Test)**: Wrote unit tests in `internal/doctor/web_test.go` asserting `checkWebTools` probe with disabled state, duckduckgo, brave (valid & missing key), tavily (valid & missing key), and searxng.
- [x] **GREEN (Impl)**:
  - Implemented `checkWebTools` probe in `internal/doctor/web.go` and registered it in `internal/doctor/doctor.go`.
  - Verified main wiring through `tools.Select` in `cmd/agis/main.go`.
  - Updated documentation in `docs/configuration.md`, `docs/cli.md`, and `README.md`.
- [x] **REFACTOR & VERIFY**: Ran full test suite across the entire project with `go test -race -count=1 ./...` (100% PASS) and `go vet ./...` (zero warnings).

## TDD Cycle Evidence

| Phase | Task / Work Unit | Action / Evidence | Result |
|-------|------------------|-------------------|--------|
| RED | Unit 1 (Config & Mask) | Added `TestLoad_WebDefaultsAndExplicit`, `TestMaskSecrets_WebProviders`, `TestGet`/`TestSet` for web tools | Tests failed to compile due to missing `WebConfig` fields |
| GREEN | Unit 1 (Config & Mask) | Added `WebConfig`, `WebProviders`, `ProviderConfig`, defaults, masking, and helpers in `internal/config/` | All config tests passed |
| REFACTOR | Unit 1 (Config & Mask) | Ran `go test -race -count=1 ./internal/config/...` | PASS (1.029s) |
| RED | Unit 2 (Web Search) | Wrote `internal/tools/web/search/search_test.go` with mock servers, error cases, leak verification | Test failed (package missing non-test Go files) |
| GREEN | Unit 2 (Web Search) | Implemented `search.go`, `brave.go`, `tavily.go`, `searxng.go`, `duckduckgo.go` | Tests ran; fixed DDG HTML container recursion |
| REFACTOR | Unit 2 (Web Search) | Added test cases for uddg links, lite tables, no-results; ran `go test -race -count=1 ./internal/tools/web/search/...` | PASS (1.113s, zero leaks) |
| RED | Unit 3 (Web Fetch & Extractor) | Wrote `internal/tools/web/fetch/extractor_test.go`, `ssrf_test.go`, `fetch_test.go` | Tests failed to compile (no non-test Go files in `fetch`) |
| GREEN | Unit 3 (Web Fetch & Extractor) | Implemented `ssrf.go`, `extractor.go`, `fetch.go` | Initial tests ran; fixed `DocumentNode` traversal and whitespace handling in text nodes |
| REFACTOR | Unit 3 (Web Fetch & Extractor) | Ran `go test -v -race -count=1 ./internal/tools/web/fetch/...` | PASS (1.327s, zero leaks, zero races) |
| RED | Unit 4 (Tool Runners & Policy) | Wrote `internal/tools/web_search_test.go`, `web_fetch_test.go`, `registry_test.go`, `internal/policy/guard_test.go` | Tests failed (missing runner implementations & web policy rule mismatch) |
| GREEN | Unit 4 (Tool Runners & Policy) | Implemented `web_search.go`, `web_fetch.go`, `FromWebConfig` in `registry.go`, `matchPattern` in `store.go`, and `guard.go` web handling | All runner, registry, policy, and brain tests passed |
| REFACTOR | Unit 4 (Tool Runners & Policy) | Ran `go test -race -count=1 ./internal/tools/... ./internal/policy/... ./internal/core/...` | PASS (1.169s) |
| RED | Unit 5 (Doctor Probe & Docs) | Wrote `internal/doctor/web_test.go` | Tests failed to compile (missing `checkWebTools`) |
| GREEN | Unit 5 (Doctor Probe & Docs) | Implemented `internal/doctor/web.go`, wired into `doctor.go`, updated `docs/` and `README.md` | All doctor tests and full repo tests passed |
| REFACTOR | Unit 5 (Doctor Probe & Docs) | Ran full test suite `go test -race -count=1 ./...` and `go vet ./...` | 100% PASS across all 23 packages; 0 vet issues |

## Files Changed

- `go.mod`: Added `golang.org/x/net` dependency
- `go.sum`: Updated checksums for `golang.org/x/net`
- `README.md`: Added Native Web Search & Content Extraction capability
- `docs/cli.md`: Added doctor probe 10 (`web_tools`) documentation
- `docs/configuration.md`: Added `tools.web` configuration documentation
- `internal/config/config.go`: Added `WebConfig`, `WebProviders`, `ProviderConfig`, default constants and initialization
- `internal/config/mask.go`: Added web providers API key masking
- `internal/config/config_test.go`: Added `TestLoad_WebDefaultsAndExplicit`
- `internal/config/mask_test.go`: Added `TestMaskSecrets_WebProviders`
- `internal/config/accessor_test.go`: Added `Get`/`Set` tests for `tools.web.*`
- `internal/core/brain.go`: Added routing for `"web"` backend and `CategoryNetwork` in `executeTool`
- `internal/core/brain_tools_test.go`: Added `TestBrainLoop_WebTools_EvaluationAndExecution`
- `internal/doctor/doctor.go`: Wired `d.checkWebTools` into diagnostic probe suite
- `internal/doctor/doctor_test.go`: Updated expected doctor probes to include `"web_tools"`
- `internal/doctor/web.go`: Implemented `checkWebTools` diagnostic probe
- `internal/doctor/web_test.go`: Comprehensive unit tests for `checkWebTools` across providers
- `internal/policy/guard.go`: Supported `network` category and `web` backend policy tiers
- `internal/policy/guard_test.go`: Added `TestGuard_WebToolsEvaluation`
- `internal/policy/store.go`: Updated `matchPattern` with wildcard and prefix matching
- `internal/tools/registry.go`: Added `FromWebConfig` and registered web tools in `Select`
- `internal/tools/registry_test.go`: Added registration and disabled state tests for web tools
- `internal/tools/web_fetch.go`: Implemented `WebFetchRunner`
- `internal/tools/web_fetch_test.go`: Unit tests for `WebFetchRunner`
- `internal/tools/web_search.go`: Implemented `WebSearchRunner`
- `internal/tools/web_search_test.go`: Unit tests for `WebSearchRunner`
- `internal/tools/web/fetch/extractor.go`: Pure Go HTML to Markdown converter
- `internal/tools/web/fetch/extractor_test.go`: Markdown extraction unit tests
- `internal/tools/web/fetch/fetch.go`: Fetcher HTTP client, size limiter, content type validator
- `internal/tools/web/fetch/fetch_test.go`: HTTP fetcher unit tests with httptest servers
- `internal/tools/web/fetch/ssrf.go`: SSRF IP checker and safe transport dialer
- `internal/tools/web/fetch/ssrf_test.go`: SSRF unit tests
- `internal/tools/web/search/brave.go`: Brave searcher
- `internal/tools/web/search/duckduckgo.go`: DuckDuckGo searcher
- `internal/tools/web/search/search.go`: Searcher interface and factory
- `internal/tools/web/search/search_test.go`: Searcher unit tests
- `internal/tools/web/search/searxng.go`: SearXNG searcher
- `internal/tools/web/search/tavily.go`: Tavily searcher
- `openspec/changes/web-tools/tasks.md`: Checked off all tasks (Units 1-5)

## Test Commands Run

- `go test -race -count=1 ./internal/config/...` -> PASS
- `go test -race -count=1 ./internal/tools/...` -> PASS
- `go test -race -count=1 ./internal/tools/web/search/...` -> PASS
- `go test -race -count=1 ./internal/tools/web/fetch/...` -> PASS
- `go test -race -count=1 ./internal/policy/...` -> PASS
- `go test -race -count=1 ./internal/doctor/...` -> PASS
- `go test -race -count=1 ./internal/core/...` -> PASS
- `go test -race -count=1 ./...` -> PASS (All 23 packages across entire repository)
- `go vet ./...` -> PASS (Zero warnings)

## Deviations from Design

None. Implementation strictly conforms to `openspec/changes/web-tools/design.md` and `spec.md`.

## Remaining Tasks

All tasks in Units 1, 2, 3, 4, and 5 are fully implemented, verified with strict TDD, and marked complete (`- [x]`).
Remaining lifecycle actions: verification and archive (`sdd-verify`, `sdd-archive`).

## Review Workload & PR Boundary

- **Completed Slices**: Batch 1 (Unit 1 & Unit 2), Batch 2 (Unit 3), Batch 3 (Unit 4 & Unit 5)
- **Delivery Strategy**: `auto-chain` (Stacked PRs to main)
- **Status**: All implementation units complete and verified. Ready for verify phase.
