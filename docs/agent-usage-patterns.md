# Agent Usage Patterns

**Document status:** live — this file is tracked by git and reflects the
current stable surface. If you are iterating on a pre-release branch, check for
uncommitted changes or a newer version in the working tree before treating this
as authoritative.

This document describes effective patterns for AI agents using the
opengrok-go-mcp server. These patterns apply to both compact and full tool
surfaces — adjust the tool names as needed.

---

## 1. Symbol Investigation: find → read context → follow references

The most common pattern for understanding a piece of code.

**Step 1 — locate the symbol.**

Using compact tools:

```json
{
  "operation": "definitions",
  "payload": { "symbol": "ValidateEmail", "project": "platform" }
}
```

Using full tools:

`search_symbol_definitions` with `symbol: "ValidateEmail"`.

Returns the definition location with surrounding context lines, file path, and a
citation URL.

**Step 2 — read the definition file.**

If the snippet isn't enough, read the full file (or a broader window) around the
definition:

```json
{
  "operation": "file",
  "payload": { "file_path": "src/validation/email.go" }
}
```

Or `read_file` in full mode.

Alternatively, use `get_file_context` (or `operation=context`) with the exact
`line_number` from the search result to get a targeted window.

**Step 3 — follow references.**

Find all usages of the symbol across the project:

```json
{
  "operation": "references",
  "payload": { "symbol": "ValidateEmail", "project": "platform" }
}
```

Or `search_symbol_references` in full mode.

Each reference result includes the file path, line number, snippet, and citation
URL. Include `citation.url` when answering about a specific symbol so the user
can navigate to the source.

**Step 3b — find implementations.**

For interfaces and abstract types, search for candidate implementations:

```json
{
  "operation": "implementations",
  "payload": { "symbol": "Notifier" }
}
```

Results are best-effort reference matches, not guaranteed implementations.

---

## 2. Structural Exploration: project overview → list symbols → drill into files

Useful for understanding a project's architecture before diving into specific
code.

**Step 1 — get the project overview.**

```json
{
  "project": "platform"
}
```

Returns total files, total directories, and top-level entries. The
`total_directories` and `total_files` fields give a sense of project scale.

**Step 2 — list symbols by kind.**

Target structural elements — classes, interfaces, functions — within a path
prefix:

```json
{
  "project": "platform",
  "kind": "interface",
  "path_prefix": "src/api/"
}
```

Gives you a bird's-eye view of the API surface. Use `kind=class` for class
hierarchies, `kind=function` for utility modules, `kind=method` for API
handlers.

Set `include_snippets=false` for broad sweeps to reduce token cost.

This tool is available in full mode as `list_symbols`. In compact mode, use
`opengrok_symbols` with `operation=list`.

**Step 3 — read interesting files.**

Once you've identified files of interest, read them directly:

```json
{
  "operation": "file",
  "payload": { "file_path": "src/api/handler.go" }
}
```

Or in full mode, use `read_file`.

For very large files, use paginated reads: pass `next_cursor` from the response
to retrieve subsequent pages.

---

## 3. Broad Search → Narrow Query

When a query returns too many results, the server returns a `warning` field
(at `total_hits > 500` for search tools) with guidance on narrowing.

**The warning looks like:**

> `Query returned 873 hits. Consider narrowing with path_prefix, file_type, or a more specific query.`

**Narrowing strategies (in order of effectiveness):**

1. **Add `path_prefix`.** Restrict to a specific directory tree:
   `path_prefix: "src/auth/"` or `path_prefix: "internal/middleware/"`.

2. **Add `file_type`.** Filter by file extension:
   `file_type: ".go"`, `file_type: ".js"`, `file_type: ".tsx"`.

3. **Use a more specific `query`.** If you searched for `"error"`, try
   `"error handling middleware"` or `"ErrNotFound"` or
   `"func.*[Ee]rror"`.

4. **Use `mode=path` for path-based search.** When you know part of a file
   path, searching by path mode narrows results to matching paths:
   ```json
   {
     "query": "auth",
     "mode": "path",
     "path_prefix": "src/"
   }
   ```

5. **Limit by project.** When searching across multiple projects, pass an
   explicit `project` to scope to a single project. Use `projects` (array) for
   a specific subset. The `total_hits` in the response always reflects the
   true count — use it to decide whether more narrowing is needed.

When `total_hits > 500`, the `warning` field is always present. Treat it as a
signal to iterate on the query rather than consuming more cursor pages.

---

## 4. Pagination: handling large result sets

Multiple tools return paginated results via a `next_cursor` field.

**Cursor contract:**

- Pass the literal `next_cursor` value from one response into the next request
  as `cursor`.
- The cursor encodes the original query context (query, mode, project, offset,
  page size). If the server has `OPENGROK_MCP_CURSOR_SECRET` set, cursors are
  HMAC-signed to prevent tampering.
- When `next_cursor` is absent or empty, there are no more pages.

**Example — paginating through search results:**

```json
// First page
{ "query": "validateEmail", "page_size": 20 }
// → response includes "next_cursor": "eyJvZmZzZXQiOjIwfQ.Kd7..."

// Second page
{ "query": "validateEmail", "page_size": 20, "cursor": "eyJvZmZzZXQiOjIwfQ.Kd7..." }
```

The same pattern applies to `list_projects`, `list_files`, `read_file` (full
mode), and all search tools.

**When `/projects/indexed` is restricted:** call `list_projects` first. By default
the server scrapes the web UI project picker at startup unless
`OPENGROK_MCP_DISABLE_PROJECT_SCRAPE=true`. The result is a startup snapshot —
restart the server after projects change on the OpenGrok instance.

**When to stop paginating:**

- The agent has found the specific information it needs.
- The query returned a `warning` about high hit counts — narrow the query
  instead of paginating through hundreds of results.
- The response contains `truncated: true` (file listing safety caps).

---

## 5. Cross-project search

When the server has multiple configured projects, you can search across all of
them or target specific subsets.

**Search across all projects:**

```json
{
  "query": "func Handler",
  "allow_all_projects": true
}
```

Results include a `project` field to identify the source project. Cross-project
attribution is heuristic — when paths don't match any queried project, the
server falls back to the default project.

**Search references across projects:**

```json
{
  "symbol": "Config",
  "projects": ["platform", "api-gateway"]
}
```

This is available as `search_cross_project_references` in full mode or
`opengrok_symbols` with `operation=cross_project_references` in compact mode.

Results are grouped by project. If attribution is uncertain, a
`attribution_uncertain` field (full mode) or `warning` field (compact mode) is
set.

---

## 6. Compound operations

When file context is available, two compound operations combine search and
read in a single call, reducing round-trips:

- **`search_and_read`** (full) / `operation=search_and_read` (compact): search
  then automatically fetch file context around each hit. Set
  `expand_context=false` in the payload if you only need search results.

- **`find_symbol_and_references`** (full) / `operation=find_symbol_and_references`
  (compact): finds a symbol's definition and all references in one call.
  Returns a combined result with both the definition site and reference
  locations, each with file context.

These tools are gated by the `GetFileContext` capability — they're available
only when the server probed it successfully at startup. Use individual
search + read calls as the fallback.

---

## 7. Structural queries (subclasses / implementers / call graphs)

OpenGrok finds *definitions*, not *relationships*. Don't full-text search for
`extends Foo` — you'll get fields, parameters, and comments, not just subclasses.

Two-step workflow:

1. **Scope with OpenGrok.** `list_symbols(kind="class", path_prefix=...)` or
   `search_symbol_definitions` to locate the package(s) involved. Read
   `has_more` / `total_pages` to know whether you've seen everything; when a
   `kind` filter is active, remember `total_hits` is the pre-filter count.
2. **Match precisely with a local AST tool.** Run ast-grep (or similar) scoped to
   the paths from step 1, e.g. `class $NAME extends BaseController`.

This replaces the "search → truncated output → shell grep → cross-reference →
repeat" loop with two deterministic calls. `search_implementations` remains
best-effort (textual references, not semantic implementers) for the same reason.

---

## Notes for agent implementors

- **Citation URLs** (`citation.url` on search and file results) point to the
  OpenGrok web UI. Include them when presenting answers to let users navigate
  to source.
- **The `total_hits` field** is returned on every search response. Use it to
  gauge result volume *before* paginating.
- **Memory tools** (`memory_set`, `memory_get`, etc.) are process-scoped and
  available only on stdio transport. They are not exposed over HTTP because
  HTTP is inherently multi-client. Use them to retain context across
  invocations when the agent supports it.
- **Gateway mode** (`OPENGROK_MCP_TOOL_SURFACE=gateway`) exposes only two
  tools: `opengrok_discover` (list available operations) and `opengrok_call`
  (dispatch any operation by name). Use `discover` first to learn what's
  available, then call by name. This is useful for agents that prefer a
  dynamic discovery contract over a static tool list.
