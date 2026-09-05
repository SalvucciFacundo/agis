# Archive Report: web-tools

## Change Overview
- **Name**: `web-tools`
- **Archived Date**: 2026-09-05
- **Status**: Completed & Archived
- **Mode**: Automatic (`auto`)
- **Artifact Store**: Hybrid (`openspec/` + Engram)
- **Delivery Strategy**: `auto-chain` (`stacked-to-main`)

## Summary of Accomplishments
1. **Configuration & Secret Masking (`internal/config`)**:
   - Added `WebConfig` and `WebProviders` structs with defaults (`duckduckgo`, 2MB response size limit, 15s timeout, default `User-Agent`).
   - Implemented secret masking for search API keys (`[MASKED]`).
   - Extended reflection accessors (`Get`/`Set`) for `tools.web.*` dot-notation keys.
2. **Multi-Provider Web Search (`internal/tools/web/search`)**:
   - Implemented `Searcher` interface with `BraveSearcher`, `TavilySearcher`, `SearXNGSearcher`, and zero-credential `DuckDuckGoSearcher`.
   - Added query validation, timeout handling, and deterministic `httptest.Server` mock test suite with `goleak` verification.
3. **Safe Web Fetch & Pure Go HTML-to-Markdown Extractor (`internal/tools/web/fetch`)**:
   - Implemented SSRF protection preventing private IP resolution and DNS rebinding attacks via custom `http.Transport.DialContext`.
   - Enforced 2MB size limit via `io.LimitReader` and binary media-type rejection.
   - Built pure Go HTML-to-Markdown AST extractor using `golang.org/x/net/html` without headless browsers or CGO.
4. **Tool Runner Bridge & PolicyGuard Integration (`internal/tools`, `internal/policy`, `internal/core`)**:
   - Implemented `WebSearchRunner` and `WebFetchRunner` implementing `core.ToolRunner` under backend `"web"` and category `core.CategoryNetwork`.
   - Registered tools in `tools.Registry` and wired PolicyGuard evaluation for `sandbox`, `standard`, and `full` tiers.
5. **Diagnostics & Documentation (`internal/doctor`, `docs/`)**:
   - Implemented `checkWebTools` probe in `internal/doctor` for operational health inspection.
   - Updated `docs/configuration.md`, `docs/cli.md`, and `README.md`.
   - Master specification synced to `openspec/specs/web-tools/spec.md`.

## Verification Results
- **Strict TDD Compliance**: 100% verified across all 5 work units.
- **Specification Requirements**: 7/7 requirements and scenarios satisfied (PASS).
- **Test Suite**: 23/23 Go packages passing with `go test -race -count=1 ./...` and `go vet ./...` (zero races, zero leaks, zero vet warnings).

## Final State Facts
- Packages added/modified: `internal/config`, `internal/tools/web/search`, `internal/tools/web/fetch`, `internal/tools`, `internal/policy`, `internal/core`, `internal/doctor`, `cmd/agis`.
- Artifacts archived to: `openspec/changes/archive/2026-09-05-web-tools/`
- Master spec at: `openspec/specs/web-tools/spec.md`
