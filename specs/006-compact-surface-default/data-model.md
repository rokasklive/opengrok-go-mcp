# Phase 1 Data Model: Compact Surface as Default

**Feature**: 006-compact-surface-default | **Date**: 2026-06-24

This feature adds no persistent data. The "model" here is the **contract surface**:
the consolidated compact tools, their operations, the typed schema each operation
exposes, and how each maps onto existing service behavior. Keeping every operation
traceable to an existing input type and service method is what guarantees the
compact surface stays a coherent view over the same behavior (Constitution I) with
a single source of truth (research D2).

## Entities

### Tool surface
The named set of MCP tools registered for a session: `full`, `compact`, `gateway`.
Selected by `OPENGROK_MCP_TOOL_SURFACE`; the shipped **default changes from `full`
to `compact`** (`config.Default()`), with `full` still selectable and unchanged.

### Compact tool
A consolidated MCP tool (`opengrok_*`) exposing several related operations behind
one name. Carries: a name, an L2 description naming every operation + when to use
each + gotchas, and a **discriminated input schema** (one branch per enabled
operation). Registered only when at least one of its operations has a verified
backing capability; a tool with no available operations is **not registered at all**
(no `ListTools` entry → no agent attention spent), matching the full surface.

### Operation
A named sub-action within a compact tool. Carries: a stable `operation` token, a
**typed input schema** (generated from its source input type), an L2 description,
and a capability gate. Dispatches to an existing service method. An operation is
present in a tool's schema **only when its capability gate is satisfied** (FR-013).

### Capability gate
A startup-probed OpenGrok capability (`cfg.Capabilities.*`) controlling availability.
Already enforced at registration (which tools appear) and per-operation at dispatch
(`unknownOperationError` lists only enabled operations). Unchanged in mechanism;
re-pointed at the new operation set.

### Migration mapping
The documented correspondence: prior-compact name/shape → new, and full tool name →
new compact tool+operation. Plus the default-change note and restore path. Lives in
`contracts/` and the user-facing migration note.

### Eval scenario (canonical)
A surface-agnostic scenario (`op` + `args`) in `evals/testdata/scenarios/` resolved
onto each surface by `evals/surface.go`. One scenario measures full and compact
alike; shared scenarios carry a cross-surface equivalence assertion.

### Eval baseline
Committed per-surface reference results under `evals/baselines/`. Compact gains its
own; CI compares against it and fails on a compact contract regression.

## Operation inventory

Every operation reuses an existing input type and service method (no new behavior;
only re-surfacing). This table is the build backbone and the equivalence map.

| Compact tool | operation | Source input type | Service method | Capability gate |
|---|---|---|---|---|
| `opengrok_projects` | `list` | `ListProjectsInput` | `ListProjects` | `ListProjects` |
| `opengrok_projects` | `files` | `ListFilesInput` | `ListFiles` | `ListFiles` |
| `opengrok_projects` | `overview` | `ProjectOverviewInput` | `GetProjectOverview` | `ListFiles` |
| `opengrok_search` | `code` | `SearchCodeInput` | `SearchCode` | `SearchCode` |
| `opengrok_search` | `read` | `SearchAndReadInput` | `SearchAndRead` | `SearchCode` + `GetFileContext` |
| `opengrok_symbols` | `definitions` | `SymbolSearchInput` | `SearchSymbolDefinitions` | `SearchSymbolDefinitions` |
| `opengrok_symbols` | `references` | `SymbolSearchInput` | `SearchSymbolReferences` | `SearchSymbolReferences` |
| `opengrok_symbols` | `find` | `FindSymbolAndReferencesInput` | `FindSymbolAndReferences` | `SearchSymbolDefinitions` + `SearchSymbolReferences` + `GetFileContext` |
| `opengrok_symbols` | `implementations` | `ImplementationSearchInput` | `SearchImplementations` | `SearchSymbolReferences` |
| `opengrok_symbols` | `cross_project` | `CrossProjectReferencesInput` | `SearchCrossProjectReferences` | `SearchSymbolReferences` |
| `opengrok_symbols` | `list` | `ListSymbolsInput` | `ListSymbols` | `ListSymbols` |
| `opengrok_read` | `file` | `FileContextInput` | `GetFileContext` (full read) | `GetFileContext` |
| `opengrok_read` | `context` | `FileContextInput` | `GetFileContext` (windowed) | `GetFileContext` |

Notes:
- `opengrok_search.read` and `opengrok_symbols.find` absorb the two operations of
  the removed `opengrok_compound` tool (research D1).
- `opengrok_search` no longer exposes `definitions`/`references`; all symbol/
  reference work lives in `opengrok_symbols` (overlap removal, FR-002).
- `overview` is gated on `ListFiles` (it is derived from directory/file listing),
  matching the full surface's `get_project_overview` gate.
- **Memory is omitted** from compact (clarified 2026-06-24, FR-014): no
  `opengrok_memory` tool is registered. Memory remains a full-surface, stdio-only
  capability pending a separate decision to sunset it. This is the single deliberate
  full↔compact divergence (Constitution I).

## Validation rules (per operation, enforced by schema + handler)

- `operation` is required and constrained to the enabled enum for that tool; an
  unknown value is rejected with an error naming the valid operations (FR-006).
- Each operation's required fields are declared in its schema branch and validated
  before dispatch (FR-004). Example: `opengrok_symbols.definitions` requires the
  bare symbol name field; `opengrok_read.context` requires `file_path` (and
  `line_number` to center a window).
- Capability-disabled operations are absent from the schema enum and rejected at
  dispatch (FR-013).
- Output fields, pagination cursors, `total_*`, truncation flags, warnings, and
  `citation.url` are produced by the **same** service methods as full, so they are
  identical across surfaces (FR-012) — the equivalence assertion (FR-021) checks
  exactly this.

## Response-state coverage (FR-015 / SC-009)

Each consolidated tool must let the agent distinguish: success, empty/zero-result
(distinct from error), partial/truncated (`truncated`+`next_cursor`), warning-carrying
(`warning` populated), error (`isError`/`Error` code), and unauthorized (capability/
transport error). These derive from the shared service layer; the compact wrappers
must not flatten or hide any of them.

## State transitions

None. All operations are read-only. Memory is omitted from the compact surface
(FR-014) and is unchanged on the full surface. No lifecycle/state machine is introduced.
