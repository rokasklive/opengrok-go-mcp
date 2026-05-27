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
project snapshot (`cfg.Projects`), not a live `/projects/indexed` fetch. When the
source is `none` and no allowlist exists, the tool lists the configured default
project only. Pagination cursors remain deterministic over the snapshot for the
process lifetime.

**Pagination inputs:**

- `page_size` (integer) — controls how many results are returned per page.
  Servers apply a capped maximum; passing a value above the cap silently clamps
  to the cap.
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

**Individual result items** carry:

- `citation.url` (string) — a URL pointing to the matching file or symbol in
  the OpenGrok web UI. Always preserve this in answers so users can navigate
  to source.

**Additive-only rule:** new output fields may be added freely — they are
additive and do not break existing consumers. Never repurpose or rename an
existing output field without a spec, a migration note, and a version bump.

---

## Errors

Fail explicitly; never return a silent empty result where an error is correct.

Concrete error conditions that must fail with a clear message:

- **32 MiB response-body cap** — API and raw fallback responses exceeding this
  limit fail with an explicit error. Do not silently truncate or return partial
  data; report that the response exceeded the size limit and suggest narrowing
  the search or requesting a smaller file.
- **Malformed or mismatched cursors** — rejected with an explicit error, not
  silently discarded.
- **File-read failures** — if the server probed file access at startup but the
  actual read fails, surface the failure rather than returning an empty result.

Error messages must be actionable: tell the agent *why* the call failed and
what to try next. For wording guidance on agent-facing messages, see
[`docs/agent-ux.md`](agent-ux.md).

---

## Warnings

`warning` is a first-class string field on responses. Agents must read it — it
carries information that changes how results should be interpreted.

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
  what was and was not expanded. Never assume every result includes expanded
  context.

If you add a new cap, you must also add a `truncated=true` indicator and a
`warning` with a narrowing suggestion. Silent truncation is a contract
violation.

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
