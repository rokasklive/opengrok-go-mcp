# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See
`docs/release-process.md` for changelog and versioning rules.

## [Unreleased]

### Added
- Hermetic stdio subprocess eval harness in `evals/` (dataset-driven cases,
  markdown/JSON reports, `go test ./evals/`).
- GitHub Actions CI (`.github/workflows/ci.yml`): full test suite on PR/push;
  README eval summary auto-update on push to `main`.
- `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` opt-out for web-UI project discovery.
- Actionable startup log when OpenGrok returns unauthorized responses and no auth
  token is configured.

### Changed
- Split `internal/mcpserver` monolith into per-concern files (non-functional
  refactor; MCP contract unchanged).
- README client setup: collapsible per-agent copy-paste configs; environment
  variable tables consolidated under Client Setup.
- **Breaking:** Web-UI project discovery runs by default when the REST project
  list fails (was opt-in via `OPENGROK_MCP_PROJECT_SCRAPE=true`). Set
  `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE=true` to restore the old no-scrape behavior.
- `OPENGROK_MCP_DEFAULT_PROJECT` is never required at startup; auto-set when
  exactly one project is discovered.
- Startup no longer exits when all search probes are unauthorized without a
  configured token; search tools are capability-gated with auth guidance in logs.
- **Breaking:** `OPENGROK_MCP_API_TOKEN` now takes the full `Authorization` header value
  (`Bearer <token>` or `Basic <credentials>`). `OPENGROK_MCP_BASIC_AUTH_TOKEN` is removed.

### Migration

| Old setup | New equivalent |
|---|---|
| Base URL + default project + auth + `PROJECT_SCRAPE=true` | Base URL + auth (scrape is default) |
| `PROJECT_SCRAPE=false` (default) on restricted instances | `DISABLE_PROJECT_SCRAPE=true` if you want zero web-UI fetches |
| `OPENGROK_MCP_BASIC_AUTH_TOKEN` | `OPENGROK_MCP_API_TOKEN="Basic <credentials>"` |
| Bare `OPENGROK_MCP_API_TOKEN=<token>` (no scheme) | `OPENGROK_MCP_API_TOKEN="Bearer <token>"` |

### Added (prior unreleased work)
- Web-UI project discovery via startup scrape ladder (`configured → api → scraped → none`)
  (default off): startup resolution ladder `configured → api → scraped → none`.
- Startup probe failure classification in logs (TLS hostname mismatch with cert
  SAN hostnames, endpoint-restricted vs unauthorized, feature-unsupported).
- `list_projects` serves the startup-resolved project snapshot consistently with
  search-project validation.

### Fixed
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY` now preserves default transport
  behavior including `http.ProxyFromEnvironment` for forward-proxy setups.

### Changed
- Default-project validation is deferred until after startup project discovery.
- Non-2xx OpenGrok HTTP responses surface as typed `StatusError` for clearer
  probe diagnostics.

## [0.3.0] - 2026-05-27

### Added
- Automatic search-result context expansion with configurable line windows,
  result/file limits, concurrency, diagnostics, and `minimal` / `default` /
  `maximal` context budgets.
- `list_symbols` for ctags definition inventory, including kind filtering,
  pagination, and cost warnings for broad enumerations.
- `tokenized` and `path_exclude` search parameters, with multiple
  space-separated exclude terms supported.
- Auto-quoting of bare multi-word code queries, with a warning when applied.
- Warnings for `date:` misuse, broad tokenized result sets, heuristic
  cross-project attribution, truncation, and best-effort operations.
- Pagination fields (`page`, `total_pages`, `has_more`, `next_cursor`) across
  search and listing responses.
- `ReadOnlyHint` annotations on all tools.
- Configurable retries, optional in-process response caching, optional cursor
  signing, and context-expansion budget overrides.
- Process-scoped memory tools for stdio sessions.
- Experimental compact and gateway tool surfaces for clients that prefer fewer
  static tools.
- Spec Kit setup: constitution, templates, and contribution-workflow policy.
- Contributor and project documentation: tool contract reference, agent UX
  guide, review checklist, release process, changelog, security policy, PR and
  issue templates, and a documentation source-of-truth map.
- Local-clone and remote MCP client setup examples for OpenCode, Claude Code,
  Codex, and manual HTTP mode.

### Changed
- Default tool surface is now `full`.
- `list_files` pagination now reports `total_files` as `total_hits` for a
  consistent pagination shape.
- `list_symbols` no longer reports a misleading global `filtered_total_hits`;
  responses instead expose normal pagination plus an explicit kind-filter
  warning when filtering is page-local.
- Search and tool descriptions were rewritten to be usable by cold,
  uninitiated agents without relying on repository-specific context.
- OpenGrok structural-query guidance now explicitly separates full-text/ctags
  search from AST- or call-graph-level analysis.
- README and configuration docs now document remote OpenGrok use, local clone
  development, expired TLS certificate handling, and full environment-variable
  coverage.
- Upgraded `github.com/modelcontextprotocol/go-sdk` from v1.2.0 to v1.4.0.
- License posture changed to Apache-2.0 for releases starting with
  `v0.3.0-beta.2`.

### Fixed
- `date:` warning detection now matches the field token instead of unrelated
  substrings.
- String-encoded scalar tool arguments are coerced for clients that pass JSON
  scalars as strings.
- Compact wrapper tools accept object payloads.
- Cursor round trips remain valid when the original bare query is auto-quoted
  deterministically.
- File-content fetches in tests are guarded for concurrent context expansion.
- Several known limitations were clarified or handled with explicit warnings,
  including page-local sorting, optimistic file-read capability detection, and
  project traversal truncation.

### Compatibility Notes
- Pre-1.0 release: minor-version changes may still alter tool descriptions,
  response details, configuration defaults, and experimental surfaces.
- `full` is now the default tool surface. Set
  `OPENGROK_MCP_TOOL_SURFACE=compact` to prefer the smaller wrapper-tool
  surface, or `gateway` for the experimental discovery/dispatch surface.
- `list_symbols` kind filtering is page-local because OpenGrok does not expose
  server-side ctags-kind filtering. Use `next_cursor` or narrower paths for
  broader inventories.
- Search outputs now favor explicit pagination fields over ad hoc total fields.
  Clients should rely on `has_more` and `next_cursor` for paging.
- Gateway mode remains experimental and may change before 1.0.

## [0.3.0-beta.2] - 2026-05-26

### Added
- `tokenized` and `path_exclude` search parameters (multiple space-separated
  exclude terms supported).
- Auto-quoting of bare multi-word code queries, with a warning when applied.
- Warnings for `date:` misuse and large tokenized result sets.
- Pagination fields (`page`, `total_pages`, `has_more`) on search results.
- `ReadOnlyHint` annotations on all tools.

### Changed
- Default tool surface is now `full`.
- Unified `list_files` pagination (`total_files` reported as `total_hits`).
- Replaced `filtered_total_hits` with pagination plus an honest kind-filter
  warning.
- Upgraded go-sdk from v1.2.0 to v1.4.0.

### Fixed
- `date:` warning now matches the field token instead of a substring.
- Coerce string-encoded scalar tool arguments.
- Accept object payloads in compact wrapper tools.

## [0.3.0-beta.1] - 2026-04-25

### Added
- `list_symbols` tool with ctags kind filtering and a pagination cost warning,
  gated behind the `ListSymbols` capability.
- Automatic search-result context expansion (`expand_context`, with
  `AutoExpandContext`, `ContextBefore`, `ContextAfter` config).
- `Result.Kind` wired from the ctags tag rather than search mode.

### Fixed
- Added a mutex for concurrent file-content fetches.

## [0.2.0] - 2026-04-22

### Added
- Cursor pagination for `list_projects` and `read_file` (500-line page cap).
- Pagination and warning fields on MCP types.
- Search warning when `total_hits` exceeds 500.
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY` for legacy TLS certificates.

### Fixed
- Guard against empty-file panic in `pagedFileContext`.
- Guard against out-of-bounds offset in `list_projects` pagination.

## [0.1.0] - 2026-04-22

### Added
- Initial release: OpenGrok-backed MCP server with code search, file read, and
  symbol search; required default project; 32 MiB response-body cap.
