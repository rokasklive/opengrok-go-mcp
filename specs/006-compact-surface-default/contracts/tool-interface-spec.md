# Contract: Compact Tool-Interface Specification

**Feature**: 006-compact-surface-default | **Date**: 2026-06-24
**Audience**: an AI agent seeing the server cold. **Status**: design (draft descriptions).

Shaped with the **design-templates** discipline (every field is an obligation
tracing to a spec FR; WHAT here, not HOW). Sections emitted: Domain Scope &
Ownership (§1), Progressive Disclosure (§6), Tool Interface Spec (§7), plus the
error contract and CUJ test plan. The other design-template sections (boundary map,
scale projection, coordination) are **N/A** — single-process, single-agent, no
inter-agent boundaries.

## §1 Domain scope & ownership boundary

- **Domain**: the `compact` MCP tool surface (4 tools; memory tool omitted — clarified 2026-06-24), made default-eligible.
- **Encodes**: FR-001..FR-016 (consolidation, typed schemas, descriptions, parity,
  errors, contract preservation, gating, default flip, labeling).
- **This contract owns (WHAT)**: tool names, operation sets, each operation's typed
  input schema, the L2 descriptions, the error format, the default vs verbose output
  policy, and the migration map.
- **Does NOT own**:
  - *HOW to build it* (schema composition code, dispatch, coercion) → tasks.md /
    implementation (implementation-playbooks discipline).
  - *Whether the built surface complies* → `quickstart.md` go/no-go checklist
    (review-checklists discipline).
  - *Whether it performs* (success rate, tokens, equivalence) → `evals/`
    (evaluation-harness-designer discipline).
- **Source of truth**: every operation schema is generated from the existing Go
  input type in `data-model.md`. This contract does not redefine field semantics;
  it specifies the *surface shape* and the *agent-facing copy*.

## §6 Progressive disclosure

- **Default output is the lean output.** Compact wrappers return exactly what the
  underlying full service returns — no added verbosity (anti-pattern #2/#7, L1).
- **Verbose / escape hatches carry over unchanged and stay discoverable**:
  `opengrok_symbols.list` keeps `include_snippets` (default lean for big sweeps);
  `opengrok_read`/`opengrok_search` keep their windowing and truncation. Every such
  control is in the operation's typed schema, so the agent can discover it without
  reading prose (no progressive *concealment*).
- **Pagination is the disclosure mechanism for large sets**: `next_cursor` +
  `truncated` + `total_*` are preserved verbatim (FR-012); the agent pages instead
  of receiving bloat.

## §7 Tool interface specification

Each tool below gives its **L2 description** (draft, agent-facing) and its
**operation schema** (discriminated by `operation`). Field lists name the
load-bearing fields per operation; the authoritative field set is generated from
the source input type (`data-model.md`) so it cannot drift. `*` = required.

### `opengrok_projects` — project & file navigation

> List indexed OpenGrok projects, list files in a project directory, or get a
> project overview. operation=list returns indexed projects (paginated; pass
> next_cursor). operation=files lists files under a path in a project (paginated).
> operation=overview returns a project's language breakdown, file/directory counts,
> and top-level entries — use it for "what languages does project X use?". Omit
> project unless the user named one; do not infer it from the local repo.

| operation | required / key fields | notes |
|---|---|---|
| `list` | — (optional `page_size`, `cursor`) | replaces `list_projects`; `total_projects` always returned |
| `files` | `project*`, `path` | replaces `list_files`; paginated |
| `overview` | `project*` | replaces `get_project_overview` |

### `opengrok_search` — text/path/history search (+ read)

> Search OpenGrok code with Apache Lucene. operation=code searches text, file paths
> (mode=path), or history (mode=history). operation=read does the same search and
> returns the file content around each match in one call (fewer round trips).
> QUERY SYNTAX: wrap multi-word queries in quotes for exact phrases
> ("extends PaymentProcessor"); bare multi-word queries are auto-quoted — set
> tokenized=true to search words independently. Inline syntax works: -path:legacy,
> +path:domain, defs:Name, date:[…] (history mode only; flagged in a warning if used
> elsewhere). For symbol definitions/references use opengrok_symbols, not this tool.

| operation | required / key fields | notes |
|---|---|---|
| `code` | `query*` (opt `mode`, `path_prefix`, `path_exclude`, `file_type`, `tokenized`) | text/path/history; **no** symbol def/ref here (moved to symbols) |
| `read` | `query*` (+ same search fields, + context window) | absorbs `search_and_read`; same query interface as `code` |

### `opengrok_symbols` — symbols, structure & references (single home for reference-finding)

> Work with ctags symbols and references — the one place for "where is X defined /
> who references X". Pass a bare symbol name (PaymentProcessor), not quoted.
> operation=definitions finds definitions; operation=references finds references;
> operation=find returns a definition with surrounding context plus its references in
> one call; operation=implementations finds candidate implementations (best-effort —
> OpenGrok has no language-semantic implementation map); operation=cross_project finds
> references across projects, grouped by project; operation=list lists definitions in a
> path, optionally filtered by kind (class/interface/function/…) for structural,
> architect-style queries. Results are full-text/ctags-backed, not an AST/call graph.

| operation | required / key fields | notes |
|---|---|---|
| `definitions` | `symbol*` | replaces `search_symbol_definitions` |
| `references` | `symbol*` | replaces `search_symbol_references` (and prior compact `search.references`) |
| `find` | `symbol*` | absorbs `find_symbol_and_references`; definition+context+paginated refs |
| `implementations` | `symbol*` | best-effort; carries the heuristic warning |
| `cross_project` | `symbol*` | replaces `search_cross_project_references`; grouped by project |
| `list` | (opt `path_prefix`, `kind`, `include_snippets`) | replaces `list_symbols`; lean-by-default for big sweeps |

### `opengrok_read` — read a known file

> Read a file you already located. operation=file returns full content (up to the
> per-call line cap; pass next_cursor if truncated; total_lines always returned).
> operation=context returns a line window around line_number (size with before/after).
> Use project + file_path from a search result. Do not WebFetch display_url/raw_url —
> this tool sends configured auth and falls back to /raw. Include citation.url when
> you answer about the file.

| operation | required / key fields | notes |
|---|---|---|
| `file` | `project*`, `file_path*` | full read; paginated by lines |
| `context` | `project*`, `file_path*`, `line_number*` (opt `before`, `after`) | line window |

> **Memory is intentionally omitted from the compact surface** (FR-014, clarified
> 2026-06-24): no `opengrok_memory` tool is registered. Memory remains a full-surface,
> stdio-only capability pending a separate decision to sunset it. This is the single
> deliberate full↔compact divergence.

## Error contract (FR-006 / L2)

Every error is actionable and distinguishable from an empty result:
- **Unknown operation** → `Error{code: unknown_operation, message: "Unknown
  operation \"X\"; enabled operations: a, b, c."}` (schema enum also rejects it
  pre-dispatch).
- **Missing/invalid required field** → schema validation error naming the field and
  the operation it belongs to.
- **Capability-gated** → operation absent from the enum; at dispatch, an error that
  states the backing capability was not verified.
- Error shape mirrors the full surface; the equivalence assertion checks error
  parity on shared negative cases.

## CUJ test plan (design-templates §7 C2.d — WHICH journeys, not HOW)

Critical user journeys that MUST be exercised on compact (run by `evals/`, see
`quickstart.md` for HOW):

1. **Locate + read**: search code → read context around a hit (`opengrok_search`
   → `opengrok_read`). Baseline: full surface equivalent.
2. **Symbol investigation**: find a definition + its references in one call
   (`opengrok_symbols.find`). Baseline: full `find_symbol_and_references`.
3. **Structural sweep**: list classes under a path (`opengrok_symbols.list`,
   `include_snippets=false`). Baseline: full `list_symbols`.
4. **Project orientation**: project overview + file listing (`opengrok_projects`
   `.overview`/`.files`). **New parity coverage** — no prior compact baseline.
5. **Negative**: invalid `operation` and missing required field return actionable
   errors.
6. **Schema-only construction**: build a valid call from the typed schema without
   reading the description (proves FR-004/SC-004).

Pre/post baselines: each journey's full-surface result is the equivalence baseline
(FR-021); journeys 4 and 6 are compact-specific (no full analog) and assert on the
compact output directly.

## Versioning / migration strategy (design-templates §7 C2.e)

- **Strategy = Test** (CUJ equivalence) + **migration note**, not version-pinning:
  compact is non-default (effectively experimental) today, so its names are not yet
  a frozen contract. The flip makes them stable going forward.
- Removed/renamed names are listed in `contracts/migration-map.md`; the user-facing
  migration note records the default change and restore path.
