# Tool Contract Guidelines

This is the detailed reference for anyone changing an MCP tool's input or
output. The *normative* rules live in
[`.specify/memory/constitution.md`](../.specify/memory/constitution.md)
(Principle I) and the quick rules in [`AGENTS.md`](../AGENTS.md). This doc is
the detailed *how* — what each field means, when it is required, and what breaks
compatibility. Do not look here for the full limitations list; that lives in
[`docs/limitations.md`](limitations.md).

> **Warnings, pagination metadata, truncation indicators, and attribution
> uncertainty are user-facing contract fields. They are not decorative.**

---

## Inputs

Every tool input field is either required or optional with a documented safe
default.

**Project scoping** is controlled by three mutually exclusive fields:

- `project` (string) — search within a single named project.
- `projects` (array of strings) — search within a specific subset of projects.
- `allow_all_projects` (boolean, default `false`) — search across all
  configured projects. Must be explicitly set to `true`; it does not activate
  automatically.

Only one of these should be supplied per call. When none is supplied, behavior
is project-scoped to the server's configured default.

When the startup-resolved allowlist is non-empty (`configured`, `api`, or
`scraped` source), explicitly naming a project outside that list returns
`UNKNOWN_PROJECT` with a message naming the allowlist and its source. Setting
`allow_all_projects=true` bypasses the allowlist for that call.

**`list_projects` result source:** returns a paginated view of the startup-resolved
project snapshot (`cfg.Projects`), not a live `/projects/indexed` fetch. Responses
include `catalog_source` (how the list was resolved: `configured`, `api`, `scraped`,
or `none`) and `catalog_is_snapshot` (always `true` in v1 — restart after OpenGrok
adds or removes projects). When the
source is `none` and no allowlist exists, the tool lists the configured default
project only. Pagination cursors remain deterministic over the snapshot for the
process lifetime.

**Pagination inputs:**

- `page_size` (integer) — controls how many results are returned per page.
  Servers apply a capped maximum; passing a value above the cap silently clamps
  to the cap. The `results` array length never exceeds the effective
  `page_size` (if OpenGrok over-delivers, results are truncated and a
  `warning` is emitted).
- `cursor` (string) — opaque token from a previous response's `next_cursor`.
  Pass it verbatim to retrieve the next page. Do not construct or modify cursors
  manually.

**Compatibility rule:** adding a *required* input field to an existing tool is
a breaking change. New inputs must be optional with a safe default. Removing
or renaming an existing input is equally breaking and requires a spec and
migration note.

---

## Outputs

**Search and paginated responses** always carry:

- `total_hits` (integer) — the count of all matching results, *before*
  pagination. Use this before deciding to paginate. It reflects the
  pre-kind-filter count for tools like `list_symbols` where kind filtering is
  page-local.

When `list_symbols` is called with `kind` set, the response also includes:

- `kind_filter_active` (boolean, `true`) — kind filtering was applied page-locally.
- `kind_matches_on_page` (integer) — count of symbols on this page after the kind filter.
- `total_hits_scope` (`"pre_kind_filter"`) — `total_hits` counts all definition
  matches before the kind filter.

**Individual result items** carry:

- `citation.url` (string) — a URL pointing to the matching file or symbol in
  the OpenGrok web UI. Always preserve this in answers so users can navigate
  to source. This is the canonical link field; `display_url` and `raw_url` are
  omitted when `response_mode=compact` because they duplicate `citation.url` or
  are not needed when `opengrok_read` / `read_file` is available.

**Response detail** (`response_mode` on search tools; default from agent profile —
`economy` → `compact`, `rich` → `full`; per-call value overrides):

- `full` (default) — normal result shape including `display_url` / `raw_url`
  when `include_links` is true, and automatic context expansion when
  `expand_context` is true (server default).
- `compact` — skips automatic context expansion (same as `expand_context=false`
  for that call) and omits redundant per-result fields (`display_title`,
  `display_url`, `raw_url`, empty `metadata`). **`citation` is always kept.**
  Use this for large sweeps where inlined context and duplicate URLs are too
  costly.

`response_mode` is independent of the tool surface (`compact` vs `full` tools).
The shipped compact **tool surface** does not imply `response_mode=compact`.

Prefer `OPENGROK_MCP_AGENT_PROFILE=economy` (shipped default) or per-call
`expand_context`, `include_links`, and `page_size` when you need fine control.
Set `OPENGROK_MCP_AGENT_PROFILE=rich` or `response_mode=full` for answer-ready
payloads.

**Additive-only rule:** new output fields may be added freely — they are
additive and do not break existing consumers. Never repurpose or rename an
existing output field without a spec, a migration note, and a version bump.

---

## Errors

Fail explicitly; never return a silent empty result where an error is correct.

Failed tool calls return structured error content in MCP `StructuredContent`:

- `error_code` (string) — machine-readable code for branching (e.g.
  `FILE_NOT_FOUND`, `INVALID_CURSOR`, `UPSTREAM_HTTP_ERROR`).
- `message` (string) — actionable human-readable explanation.
- `details` (object, optional) — extra context such as `http_status` and
  `path` for upstream HTTP failures.

The MCP result `Content` text duplicates `message` for clients that do not read
structured content. Agents should prefer `error_code` over parsing prose.

Concrete error conditions that must fail with a clear message:

- **32 MiB response-body cap** — API and raw fallback responses exceeding this
  limit fail with an explicit error. Do not silently truncate or return partial
  data; report that the response exceeded the size limit and suggest narrowing
  the search or requesting a smaller file.
- **Malformed or mismatched cursors** — rejected with `INVALID_CURSOR`, not
  silently discarded.
- **File-read failures** — HTTP 404 on raw file fetch maps to `FILE_NOT_FOUND`
  with project/path guidance; other upstream HTTP errors use
  `UPSTREAM_HTTP_ERROR` with `details.http_status`.

Error messages must be actionable: tell the agent *why* the call failed and
what to try next. For wording guidance on agent-facing messages, see
[`docs/agent-ux.md`](agent-ux.md).

---

## Warnings

Responses carry structured warnings:

- `warnings` (array) — `{ "code": "<CODE>", "message": "<text>" }` entries.
  Prefer matching on `code` rather than parsing `message`.
- `warning` (string, legacy) — space-joined messages from `warnings`; kept for
  backward compatibility.

Agents must read warnings — they carry information that changes how results
should be interpreted.

**Warning codes** (non-exhaustive):

| Code | Meaning |
|---|---|
| `PAGE_SIZE_TRUNCATED` | OpenGrok returned more hits than `page_size`; results clipped |
| `AUTO_QUOTED_QUERY` | Bare multi-word query auto-wrapped in quotes |
| `DATE_IGNORED_OUTSIDE_HISTORY` | `date:` ignored outside history mode |
| `HIGH_HIT_COUNT` | `total_hits` above advisory threshold |
| `SORT_UNSUPPORTED` | Requested sort not supported server-side |
| `KIND_FILTER_PAGE_LOCAL` | `list_symbols` kind filter applied page-locally |
| `LARGE_SYMBOL_LIST` | Large definition set; narrowing advised |
| `FILE_LIST_TRUNCATED` | `/list` 5,000-entry cap hit |
| `FILE_READ_FAILED` | Compound read could not fetch some files |
| `EXPANSION_BUDGET_HIGH` | Auto-expanded context exceeds ~50% of page payload; use economy profile or `expand_context=false` |
| `NO_DEFINITION_FOUND` | Symbol definition missing in compound search |
| `BEST_EFFORT_IMPLEMENTATION` | Implementation search is heuristic |

Live warning triggers:

- **Auto-quoting of bare multi-word queries.** When a query looks like a plain
  phrase and `tokenized=true` is not set and no Lucene syntax is detected, the
  server wraps it in quotes and emits a warning. Agents should inspect `query`
  alongside `warning` to understand what was actually sent to OpenGrok.
- **`date:` misuse outside history mode.** `date:` constraints are only
  meaningful in OpenGrok history mode. In other modes they are ignored and a
  warning is emitted.
- **`total_hits > 500`.** High result counts trigger a warning advising the
  agent to narrow with `path_prefix`, `file_type`, or a more specific query.
  Treat this as a signal to iterate rather than paginate through hundreds of
  pages.
- **Page-local kind filtering in `list_symbols`.** OpenGrok has no server-side
  ctags-kind filter. The MCP layer filters the returned page. When a `kind`
  filter is active and more pages exist, a warning is emitted. `total_hits`
  counts all definitions before the filter; the per-page kind match count is
  reported separately.
- **Truncation.** Any response that hits a cap emits a warning alongside
  `truncated=true`. See `## Truncation` below.
- **Attribution uncertainty.** When cross-project result paths cannot be
  matched to a requested project, the server sets `attribution_uncertain=true`
  on affected results and emits a warning. Verify flagged results before relying
  on citations.

---

## Pagination

Paginated tools use a `next_cursor` / `cursor` round-trip.

**Response fields:**

- `next_cursor` (string) — present when more pages exist; absent or empty when
  there are no more results. Pass the value verbatim as `cursor` on the next
  call.
- `page` (integer) — current page number (1-based).
- `total_pages` (integer) — total number of pages for the current query.
- `has_more` (boolean) — `true` when `next_cursor` is present.

**Cursor internals:** the cursor encodes the original query context — query,
mode, project, offset, and page size. When `OPENGROK_MCP_CURSOR_SECRET` is set,
cursors are HMAC-signed to prevent tampering; without it they are unsigned.
Malformed or mismatched cursors are always rejected with an explicit error.

**Stopping condition:** when `next_cursor` is absent or empty, there are no
more pages. Do not re-request the last page.

**When to stop early:** if `total_hits > 500` and a warning is present, narrow
the query rather than consuming more pages. If `truncated=true`, further
pagination will not retrieve the truncated entries — narrow the path instead.

---

## Citations

Every search result and file result carries a `citation.url` field pointing to
the matching location in the OpenGrok web UI.

Always preserve `citation.url` in agent answers. It lets users navigate
directly to source without reconstructing the URL from path components. Do not
synthesize or reconstruct this URL — use the value from the response.

---

## Truncation

When a response hits a cap, it sets `truncated=true` and emits a `warning`.
Truncation is never silent.

Known truncation points:

- **5,000-entry `/list` cap.** The OpenGrok `/list` endpoint is capped at
  5,000 entries. `list_files` and `get_project_overview` report
  `truncated=true` and a warning when this limit is reached. Further pagination
  cannot retrieve entries beyond the cap — narrow the requested `path` instead.
- **Automatic context expansion limits.** When search results include fetched
  source context, results beyond the configured result limit, file limit, or
  fetch concurrency are skipped. The `expansion` diagnostics field describes
  what was and was not expanded, including `expanded_context_bytes`. When
  auto-expansion exceeds ~50% of the page payload, `EXPANSION_BUDGET_HIGH` is
  emitted. Never assume every result includes expanded context.

If you add a new cap, you must also add a `truncated=true` indicator and a
`warning` with a narrowing suggestion. Silent truncation is a contract
violation.

---

## Tool Surfaces

The server exposes three tool surfaces via `OPENGROK_MCP_TOOL_SURFACE`. When
unset, the shipped default is **`compact`**. Set `full` to restore the
fine-grained tool list unchanged from prior releases. See
[`migration-compact-default.md`](migration-compact-default.md) for the default
change and call-shape migration.

| Surface | Tools | Call shape |
|---|---|---|
| **`compact`** (default) | `opengrok_projects`, `opengrok_search`, `opengrok_symbols`, `opengrok_read` | Flattened: `{ "operation": "<op>", …fields }` — no `payload` wrapper |
| **`full`** | `search_code`, `read_file`, `list_symbols`, … | One tool per operation; top-level typed fields |
| **`gateway`** (experimental) | `opengrok_discover`, `opengrok_call` | `{ "operation": "<gw-op>", "payload": {…} }` |

**Compact operation inventory** (capability-gated per operation; absent ops are
omitted from both schema enum and tool description):

| Tool | Operations |
|---|---|
| `opengrok_projects` | `list`, `files`, `overview` |
| `opengrok_search` | `code`, `read` |
| `opengrok_symbols` | `definitions`, `references`, `find`, `implementations`, `cross_project`, `list` |
| `opengrok_read` | `file`, `context` |

**Deliberate full-only divergence:** memory tools (`memory_*`) are not registered
on compact (stdio-only on full). This is the single intentional capability gap
between surfaces.

**Schema/description coherence:** each compact tool's `ListTools` description
lists only the operations present in that tool's discriminated input schema
(both derived from the same capability-gated operation set).

---

## Capability Gates

Tools are gated at startup by probing the backing OpenGrok instance. If a tool
is absent from the server's tool list, it means the server could not verify the
required OpenGrok capability at startup — not necessarily that the feature is
permanently unavailable.

Examples:

- File-read and compound-read tools require the `GetFileContext` capability.
- If `OPENGROK_MCP_PROBE_FILE` is not configured, the server uses a configured
  web fallback URL to establish the capability; actual failures surface on the
  first real read.
- Gateway mode (`OPENGROK_MCP_TOOL_SURFACE=gateway`) is experimental; its
  manifest and operation names may change before 1.0.

Do not treat a missing tool as a permanent failure. Check capability
configuration and retry after adjusting the probe target.

---

## Backward Compatibility

Stable defaults stay stable. Do not change a public default, rename a field,
or alter pagination or cursor semantics without:

1. A feature spec documenting the change.
2. A migration note explaining what agents must update.
3. A version bump consistent with the severity of the break.

This is grounded in constitution Principle V — see
[`.specify/memory/constitution.md`](../.specify/memory/constitution.md).

Additive changes (new optional inputs, new output fields, new warning triggers)
do not require a migration note but do require tests and docs updated in the
same change.

---

## Experimental Fields

Any field, tool, or behavior introduced as experimental must be explicitly
labeled as such in:

- The tool description (schema-level, visible to agents).
- The relevant `docs/` file.
- The environment variable or config name (e.g. `OPENGROK_MCP_TOOL_SURFACE=gateway`).

Experimental behavior may change between minor versions. It must not silently
alter stable tool behavior or defaults — the experimental path must be
explicitly opted into. If an experimental feature increases response size,
tool-call count, or automatic file fetching, it must define explicit limits,
defaults, and warnings before shipping.
