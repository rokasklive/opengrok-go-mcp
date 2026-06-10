# Known Limitations

This document describes current operational limitations of `opengrok-go-mcp`.
The project is pre-1.0, so experimental surfaces and configuration may still
change before a stable release.

## Search And Discovery

- **Multi-project result attribution is heuristic.** OpenGrok search results
  provide paths rather than a distinct project field. The server attributes
  results using the longest matching requested project prefix; when none
  matches, it marks the hit with `attribution_uncertain=true` and emits a
  warning. Verify flagged results before relying on citations or links.

- **Project traversal is capped at 5,000 entries.** The OpenGrok `/list`
  response is capped before `list_files` pagination and
  `get_project_overview` aggregation. These operations report
  `truncated=true` and a warning, but cannot page beyond that cap. Narrow the
  requested `path` for large projects.

- **`list_symbols` kind filtering is page-local.** OpenGrok supplies a page of
  definition matches and the MCP filters that page by ctags `kind`; OpenGrok has
  no server-side ctags-kind filter. `total_hits` therefore counts all definitions
  *before* the kind filter, and a global kind-filtered inventory is not available
  in one call. The response counts the kind matches on the current page and warns
  when a `kind` filter is active with more pages. Continue with `next_cursor` or
  narrow `path_prefix`.

- **`search_implementations` is best-effort.** OpenGrok does not expose
  language-semantic implementation relationships. This operation returns
  candidate symbol-reference matches, not guaranteed implementations.

- **No structural or AST-aware search.** OpenGrok is a full-text index plus ctags
  *definition* metadata. It does not model relationships (`extends`, `implements`,
  call graphs) and exposes no AST query. There is intentionally no `ast_query` tool,
  and this server has no local source checkout to run one. For structural questions,
  use OpenGrok to scope to packages, then a local AST tool (e.g. ast-grep) for
  precise matching — see [agent-usage-patterns.md](agent-usage-patterns.md).

- **Search sorting is page-local.** `sort=path` sorts only results fetched for
  the current page. `sort=date` preserves OpenGrok order and returns a
  warning; there is no global date-sorted or path-sorted result set.

- **Query normalization is helpful but not semantic.** Bare multi-word code
  queries are auto-quoted as exact phrases unless `tokenized=true` is set or the
  query already appears to use Lucene syntax. The response warns when this
  happens. This reduces noisy broad searches, but it does not infer user intent;
  agents should still inspect `query` and `warning` in the response.

- **Web-UI project discovery is best-effort.** When the REST project list fails,
  the server parses the landing-page `<select id="project">` element once at
  startup unless `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE=true`. Markup changes, auth
  differences on the web UI, truncated responses, or missing dropdowns fall
  through to the `none` source. The resolved list is a startup snapshot — projects
  added after startup are rejected as unknown until restart.

- **Startup may succeed with gated tools when auth is missing.** If all search
  probes return unauthorized responses and no auth token is configured, the server
  starts, logs which token environment variables to set, and omits search tools
  until credentials are provided and the process is restarted.

- **Zero discovered projects is a valid startup state.** When API and scrape
  discovery both fail and no explicit project list or default is configured, the
  server still starts with `source=none`. Agents must pass `project` explicitly or
  configure projects before scoped searches succeed.

- **`list_projects` serves the startup snapshot.** The tool no longer calls
  `/projects/indexed` per request; it paginates over the resolved allowlist (or
  the default project when the source is `none`). Explicitly named projects
  outside a non-empty allowlist are rejected with `UNKNOWN_PROJECT`; explicit
  all-projects search bypasses the allowlist.

- **Startup probe failures are classified in logs.** TLS hostname mismatches log
  certificate SAN hostnames; `401`/`403` on a restricted endpoint (when another
  authenticated probe succeeded) is labeled separately from credential failure;
  unsupported search modes (`400`) are distinguished from unauthorized responses.

- **Some Lucene syntax is mode-sensitive.** `date:` constraints only work in
  OpenGrok history mode; in other modes they are ignored and surfaced through a
  warning. Wildcards (`*` and `?`) cannot be used inside quoted phrases and may
  silently produce no OpenGrok matches. Use tokenized queries or unquoted Lucene
  syntax when wildcard matching is required.

## Capability And Response Boundaries

- **File-read capability detection can be optimistic.** Without
  `OPENGROK_MCP_PROBE_FILE`, a configured web fallback URL is sufficient to
  expose file-read and compound-read operations. If access is unavailable,
  failure appears on the first real read. Configure a readable
  `project/path/to/file` probe to verify access during startup.

- **OpenGrok response bodies are limited to 32 MiB.** API and raw fallback
  responses exceeding this bound fail explicitly. Narrow searches or
  directory listings, and avoid requesting very large files through the raw
  fallback.

- **Automatic context expansion is bounded and best-effort.** Search results may
  include fetched source context, but only within the configured context budget,
  result limit, file limit, and fetch concurrency. Results beyond those limits
  are skipped, compact response mode omits expansion, and failed file fetches
  leave matching results without expanded context. Inspect the `expansion`
  diagnostics before assuming every search hit includes source context.

- **Context budgets trade completeness for response size.** `minimal`,
  `default`, and `maximal` budgets change how many lines, results, and files may
  be expanded. Operators can override these defaults with environment variables,
  so agents should treat budget names as deployment-specific policy hints rather
  than exact token counts.

- **Gateway mode is experimental.** `OPENGROK_MCP_TOOL_SURFACE=gateway` exposes
  a discovery-and-dispatch surface instead of the full static tool list. Gateway
  operations use the same backend behavior, pagination, warnings, and capability
  gating, but the gateway manifest and operation names may change before 1.0.

## State And Transport

- **Memory is process-scoped and ephemeral.** Memory tools use one in-process
  store for the lifetime of a stdio server. Data is not durable or separated
  by client session, and memory tools are intentionally not exposed in HTTP
  mode.

- **HTTP mode has no inbound client authentication.** The MCP HTTP handler
  relies on its loopback default bind address or external authentication and
  network controls. Do not expose it directly to untrusted networks.

- **Cursor integrity is optional unless configured.** Pagination cursors encode
  query context and offsets. Without `OPENGROK_MCP_CURSOR_SECRET`, cursors are
  unsigned; malformed or mismatched cursors are rejected, but clients in a
  shared deployment should not rely on tamper resistance unless cursor signing
  is enabled.

- **The response cache is in-process and optional.** When
  `OPENGROK_MCP_CACHE_ENABLED=true`, supported project, file-list, file-content,
  and project-overview calls are cached with a TTL and max-entry bound. Search
  results are not cached. Cache entries are not shared across server processes,
  are not durable, and may reflect stale OpenGrok state until the TTL expires.

- **Stdio mode does not install signal-aware graceful shutdown.** HTTP mode
  handles `SIGINT` and `SIGTERM` through a shutdown context; stdio mode runs
  the MCP transport using a background context. Clients should close stdio
  transport streams when ending a session.
