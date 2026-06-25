# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See
`docs/release-process.md` for changelog and versioning rules.

## [Unreleased]

### Added
- **`citation.markdown`** output field on every search, symbol, and file result —
  a ready-to-surface `[title](url)` clickable link, present only when a URL is
  available. Tool descriptions (compact and full surfaces) now nudge agents to
  surface it so users get clickable source citations instead of bare or dropped
  URLs.

### Changed
- **Auto-quote note is informational, not a nudge to opt out.** The
  `AUTO_QUOTED_QUERY` warning and the `query`/`tokenized` schema text no longer
  frame `tokenized=true` as "opting out"; they state that the exact-phrase match
  is the default and usually the most precise result for code, and that
  `tokenized=true` is only for broader independent-term matching. Prevents agents
  from switching to tokenized before inspecting the phrase results.
- **`MISSING_REQUIRED_FIELD` also flags unrecognized fields** in the same error
  (message plus `details.unknown_fields`). Calling `opengrok_read` with
  `offset`/`limit` and no `file_path` now reports both causes at once, and the
  `opengrok_read` description clarifies there is no `offset`/`limit`: page a
  truncated file with `cursor` (from `next_cursor`) and read a slice with
  `operation=context`.
- **Compact tool descriptions** are now grounded by a claim registry and include
  explicit claim IDs for OpenGrok's text+ctags/non-AST nature, supported syntax,
  unsupported pitfalls, examples, and the configured default project.
- **Compact schemas no longer strip optional field descriptions.** Field docs on
  compact now match the full surface; token benchmarks report schema size as a
  secondary anomaly check and gate on cost per successful task.
- **Search diagnostics are opt-in.** Response `diagnostics` blocks are omitted
  by default and appear only when `OPENGROK_MCP_DIAGNOSTICS=true`.
- **Tool errors are more specific.** Compact validation failures now return
  structured `ToolErrorBody` values with `suggestion` and concrete codes such
  as `UNKNOWN_OPERATION`, `MISSING_REQUIRED_FIELD`, `INVALID_FIELD_TYPE`,
  `UNKNOWN_FIELD`, and `QUERY_PARSER_FAILED`.
- **Capability manifest (`opengrok://capabilities`)** now pins `interface_version`
  (`ergonomics-1`), emits `project_catalog.project_required` and nullable
  `default_project`, and generates tool `summary` strings from enabled
  operations only (gated families no longer oversold).
- **Compact tool descriptions** use capability-aware lead sentences and center
  token economy on `OPENGROK_MCP_AGENT_PROFILE` instead of `response_mode=compact`
  disambiguation prose.
- **Search responses** with auto-expanded context may emit `EXPANSION_BUDGET_HIGH`
  when expansion bytes exceed ~50% of page payload; `expansion.expanded_context_bytes`
  is always reported when expansion runs.
- **Default `OPENGROK_MCP_AGENT_PROFILE` is now `economy`** (lean payloads, no auto
  context expansion). Set `rich` to restore prior default behavior. Per-call
  `expand_context`, `response_mode`, and `include_links` still override.
- **`response_mode=compact`** now omits only redundant URL/title fields and
  skips context expansion; `citation` is always preserved. Dead fields
  (`column_number`, `score`, empty `metadata`) are no longer emitted on search
  results.
- **Default tool surface is now `compact`** (four consolidated tools with typed
  per-operation schemas). Set `OPENGROK_MCP_TOOL_SURFACE=full` to restore the
  previous fine-grained tool set unchanged. See `docs/migration-compact-default.md`.

### Added
- `OPENGROK_MCP_DIAGNOSTICS`: optional debug switch for internal search
  diagnostic counters in responses. Default is off.
- Live conformance coverage for registry-backed query-syntax claims, plus
  Pass^k trajectory evals and cost-per-successful-task token reporting.
- `OPENGROK_MCP_AGENT_PROFILE` (`economy` | `rich`): bundles default expansion,
  response detail, and link fields; per-call overrides win. Default is `economy`.
- `opengrok://capabilities` resource: runtime tool/gate manifest for cold agents.
- `list_projects` catalog metadata (`catalog_source`, `catalog_is_snapshot`).
- `list_symbols` kind-filter metadata (`kind_filter_active`, `kind_matches_on_page`,
  `total_hits_scope`).
- Trajectory eval suite with deterministic graders; compact `ListTools` byte ceiling
  in `evals/baselines/token_report.json`.
- Compact parity: `opengrok_projects` `files` and `overview` operations; symbol
  work consolidated in `opengrok_symbols` (`find`, `cross_project`, etc.).
- Eval harness runs contract cases on both `full` and `compact` with cross-surface
  equivalence checks.

### Removed
- Compact `opengrok_compound` and `opengrok_memory` tools (use `opengrok_search.read`,
  `opengrok_symbols.find`, or the full surface for memory).

## [0.4.0] - 2026-06-10

### Added
- **Minimal setup surface:** `OPENGROK_MCP_BASE_URL` alone is sufficient for
  typical instances; projects and capabilities are discovered at startup.
- `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` opt-out for web-UI project discovery.
- Web-UI project discovery via startup resolution ladder (`configured → api →
  scraped → none`) when the REST project list is unavailable.
- Startup probe failure classification in logs (TLS hostname mismatch with cert
  SAN hostnames, endpoint-restricted vs unauthorized, feature-unsupported).
- Actionable startup log when OpenGrok returns unauthorized responses and no auth
  token is configured.
- Hermetic stdio subprocess eval harness in `evals/` (dataset-driven cases,
  markdown/JSON reports, `go test ./evals/`).
- GitHub Actions CI (`.github/workflows/ci.yml`): full test suite on PR/push;
  README eval summary auto-update on push to `main`.
- GoReleaser release workflow on `v*` tags: cross-compiled binaries for
  linux/darwin/windows on amd64/arm64, `checksums.txt`, SPDX SBOMs, and GitHub
  Release attachments (`.github/workflows/release.yml`, `.goreleaser.yml`).
- `internal/mcpserver/README.md` contributor file map after server split.

### Changed
- **Breaking:** Web-UI project discovery runs by default when the REST project
  list fails (was opt-in via `OPENGROK_MCP_PROJECT_SCRAPE=true`). Set
  `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE=true` to restore the old no-scrape behavior.
- **Breaking:** `OPENGROK_MCP_API_TOKEN` now takes the full `Authorization` header
  value (`Bearer <token>` or `Basic <credentials>`). `OPENGROK_MCP_BASIC_AUTH_TOKEN`
  is removed.
- `OPENGROK_MCP_DEFAULT_PROJECT` is never required at startup; auto-set when
  exactly one project is discovered.
- Startup no longer exits when all search probes are unauthorized without a
  configured token; search tools are capability-gated with auth guidance in logs.
- Split `internal/mcpserver` monolith into per-concern files (non-functional
  refactor; MCP contract unchanged).
- `list_projects` serves the startup-resolved project snapshot consistently with
  search-project validation.
- Default-project validation is deferred until after startup project discovery.
- README client setup: collapsible per-agent configs, released-binary install
  path, CI status badge; trimmed duplicated tool-surface and workflow sections
  (canon remains in `docs/`).

### Fixed
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY` now preserves default transport
  behavior including `http.ProxyFromEnvironment` for forward-proxy setups.
- Non-2xx OpenGrok HTTP responses surface as typed `StatusError` for clearer
  probe diagnostics.

### Migration

| Old setup | New equivalent |
|---|---|
| Base URL + default project + auth + `PROJECT_SCRAPE=true` | Base URL + auth (scrape is default) |
| `PROJECT_SCRAPE=false` (default) on restricted instances | `DISABLE_PROJECT_SCRAPE=true` if you want zero web-UI fetches |
| `OPENGROK_MCP_BASIC_AUTH_TOKEN` | `OPENGROK_MCP_API_TOKEN="Basic <credentials>"` |
| Bare `OPENGROK_MCP_API_TOKEN=<token>` (no scheme) | `OPENGROK_MCP_API_TOKEN="Bearer <token>"` |

### Compatibility Notes
- Pre-1.0 release: minor-version changes may still alter tool descriptions,
  documentation, and configuration defaults.
- MCP tool names, schemas, and response shapes are unchanged in this release;
  the `internal/mcpserver` split is a maintainability refactor only.
- Install via `go run …@v0.4.0` or download cross-compiled binaries from GitHub
  Releases (verify `checksums.txt`).

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
