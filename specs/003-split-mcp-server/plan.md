# Implementation Plan: Split MCP Server Monolith

**Branch**: `003-split-mcp-server` | **Date**: 2026-06-10 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-split-mcp-server/spec.md`

## Summary

Decompose `internal/mcpserver/server.go` (~2730 lines, ~80 functions) into cohesive files within
the **same** `mcpserver` package: thin registration (`register_*.go`), per-tool-family handlers,
shared search pipeline (`search_core.go`, `results.go`), and split tests mirroring those files.
**Zero intentional MCP contract changes** — behavioral equivalence verified by existing tests
plus per-slice `go test ./internal/mcpserver/` and full `go test ./...`.

Delivery is **seven incremental slices** (registration → projects → search → symbols/read →
compact/memory → remove `server.go` stub → split `server_test.go`). See [research.md](./research.md)
for rationale on single-package layout and slice ordering.

## Technical Context

**Language/Version**: Go 1.25.0

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.0; existing internal
packages (`config`, `cursor`, `links`, `opengrok`).

**Storage**: Unchanged — process-scoped `Service`, `MemoryBank`, no new persistence.

**Testing**: `go test ./internal/mcpserver/` (primary); `go test ./...` (merge gate). Target
focused test files per concern after Slice G.

**Target Platform**: Unchanged — stdio MCP and loopback HTTP via `cmd/opengrok-go-mcp`.

**Project Type**: Go CLI / MCP server; refactor scope is `internal/mcpserver` only.

**Performance Goals**: No change to request latency, expansion concurrency, or page sizes.
Refactor must not add per-call allocations or duplicate backend fetches.

**Constraints**: Preserve MCP schemas, warnings, cursors, citations, capability gates, and
surface parity. `cmd` import surface: `NewMCPServer`, `Backend`, `NewCachingBackend` only.
No new env vars or DI frameworks.

**Scale/Scope**: ~7000 lines in `mcpserver` package today; goal is no single handler file
>1000 lines and `server_test.go` split into ≤8 test files.

## Constitution Check

*GATE: passed at planning. Re-check after implementation.*

- **MCP Contract**: **No intentional changes.** Registration moves must preserve exact tool
  names, descriptions, schemas, and handler bindings. Equivalence contract in
  [contracts/behavior-equivalence-contract.md](./contracts/behavior-equivalence-contract.md).
  All surfaces (`full`, `compact`, `gateway`) remain coherent views over the same `Service`
  methods.
- **OpenGrok Semantics**: Unchanged — search modes, heuristics, truncation, and warnings move
  with code but must not be edited for product reasons in this feature.
- **Test Evidence**: See **Test Plan**. Refactor slices are **not** test-first for behavior
  (no new behavior); order is **move → compile → run package tests → run full suite**. Optional
  golden tests added in Slice C only if needed to guard search pipeline moves. Existing
  `server_test.go` assertions are the primary contract proof until Slice G splits them.
- **Agent UX Validation**: Recommended before final merge: fresh-session task on fixed fixture —
  *"List projects, run full-text search, read definition file with citation."* Compare to
  pre-refactor baseline; expect identical tool list and outputs. Not required for every
  intermediate slice.
- **Security**: No change — secrets remain in `config`; memory HTTP gating stays in registration.
  Refactor must not add logging of tokens or file paths beyond existing behavior.
- **Compatibility and Docs**: **No** operator or agent doc changes required for equivalence.
  Add `internal/mcpserver/README.md` for contributors (internal only). Optional one-line
  pointer in `AGENTS.md` repository map after README exists.
- **Experimental Surface**: None — gateway labeling unchanged; no new experimental config.
- **Resource Bounds**: Unchanged — warn thresholds, page sizes, expansion budgets, file page
  size (500 lines) stay in moved code without value changes.

No constitution violations → **Complexity Tracking is empty**.

## Project Structure

### Documentation (this feature)

```text
specs/003-split-mcp-server/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── behavior-equivalence-contract.md
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/opengrok-go-mcp/
  main.go                # unchanged imports: NewMCPServer, Backend, NewCachingBackend

internal/mcpserver/
  server.go              # DELETE or shrink after slices A–F
  server_test.go         # split into *_test.go in Slice G
  service.go             # NEW — Service, Backend, Error (Slice A)
  register.go            # NEW — NewMCPServer (Slice A)
  register_full.go       # NEW (Slice A)
  register_compact.go    # NEW (Slice A)
  register_gateway.go    # NEW (Slice A)
  resources.go           # NEW (Slice A)
  helpers.go             # NEW (Slice B)
  projects.go            # NEW (Slice B)
  search_handlers.go     # NEW (Slice C)
  search_core.go         # NEW (Slice C)
  results.go             # NEW (Slice C)
  symbols.go             # NEW (Slice D)
  filecontext.go         # NEW (Slice D)
  compound.go            # NEW (Slice D)
  compact.go             # NEW (Slice E)
  memory_handlers.go     # NEW (Slice E)
  README.md              # NEW — contributor file map
  types.go               # existing
  pagination.go          # existing
  warnings.go            # existing
  coerce.go              # existing — addTool stays here
  query.go               # existing
  memory_bank.go         # existing
  cache_backend.go       # existing
  projects_test.go       # NEW (Slice G)
  search_test.go         # NEW (Slice G)
  symbols_test.go        # NEW (Slice G)
  filecontext_test.go    # NEW (Slice G)
  compact_test.go        # NEW (Slice G)
  gateway_test.go        # NEW (Slice G)
  register_test.go       # NEW (Slice G)
  resources_test.go      # NEW (Slice G)
  integration_test.go    # optional thin cross-surface tests
```

**Structure Decision**: Single `internal/mcpserver` package with file-per-concern layout;
registration separated from handlers; search pipeline centralized in `search_core.go` +
`results.go`. No new packages under `internal/`.

## Implementation Design

### Slice A — Registration and resources

Move without logic edits:

- `NewMCPServer`, `registerFullTools`, `registerCompactTools`, `registerGatewayTools`,
  `buildGatewayRegistry`, `registerResources`, `compactInputSchema`, `memoryToolsEnabled`
- Resource handlers and URI parsers (`projectsResource`, `parseProjectResourceURI`, etc.)
- `Service` struct, `NewService`, `Backend`, `Error`, `IsCode`, `listFilesWithMetadata`

**Exit criteria**: `server.go` no longer contains registration or resource blocks; `go test`
green.

### Slice B — Projects and shared helpers

Move:

- `ListProjects`, `ListFiles`, `GetProjectOverview`, `detectLanguage`, project helpers
- `resolveProjects`, `resolveSearchProjects`, `validateConfiguredProjects`, `projectSourceLabel`,
  `firstProject`, `pageSize`, `includeLinks`, `shouldExpandContext`, `resolveBudgetTier`,
  `nextCursor`, `validateResponseMode`, `cursorValue`, `invalidCursorError`, citation helpers

**Exit criteria**: Project tools compile from `projects.go`; helpers single-sourced.

### Slice C — Search pipeline

Move:

- `SearchCode`, symbol search entrypoints, `SearchCrossProjectReferences`,
  `SearchImplementations`, `search()`, `searchRequest` type if present
- `results()`, `compactResults`, `applyMaxHitsPerFile`, `applySort`, expansion functions,
  `maybeExpandResults`, `emptySearchOutput`

**Optional**: Add golden JSON tests per [behavior-equivalence-contract](./contracts/behavior-equivalence-contract.md).

**Exit criteria**: No search result shaping left in `server.go` except imports.

### Slice D — Symbols, file context, compound

Move:

- `ListSymbols`, `validateSymbolReferenceCursor`
- `GetFileContext`, `windowedFileContext`, `pagedFileContext`, `fileContextLines`,
  `extractWindow`, `fileLines`, inline `read_file` closure → named handler in `filecontext.go`
- `SearchAndRead`, `FindSymbolAndReferences`

### Slice E — Compact and memory handlers

Move:

- `CompactSearch`, `CompactSymbols`, `CompactRead`, `CompactCompound`, `CompactMemory`
- `compact*Operations()` helpers, `unknownOperationError`
- Direct `MemorySet/Get/List/Delete/Clear` methods

### Slice F — Remove monolith file

- Delete `server.go` if empty or rename leftover to avoid stale duplicate symbols
- Run `gofmt` on all touched files
- Add `internal/mcpserver/README.md` with file map from [quickstart.md](./quickstart.md)

### Slice G — Test file split

Partition `server_test.go` by concern into files listed in [data-model.md](./data-model.md).
Use `t.Run` names preserved where possible. Keep one thin integration test file if cross-surface
tests need shared fixtures.

**Mechanics**:

- Move test helpers (`fakeBackend`, etc.) to `testutil_test.go` or top of `integration_test.go`
- Verify: `go test ./internal/mcpserver/ -count=1` timing improves for filtered runs (SC-004)

## Test Plan

| ID | Slice | Command / test area | Proves |
|----|-------|---------------------|--------|
| T1 | A | `TestCompactSurfaceDoesNotExposeContentOrMemoryWithoutCapabilities` | Registration gating unchanged |
| T2 | A | `TestGatewayRegistry*` family | Gateway manifest ↔ call parity |
| T3 | B | List projects/files/overview tests | FR-002 project tools |
| T4 | C | Search code/symbol tests | Search output + warnings |
| T5 | C | Cursor/pagination tests in search | FR-003 invalid cursor |
| T6 | D | `get_file_context` / `read_file` tests | File context + pagination |
| T7 | E | `TestCompactSearchAcceptsObjectPayload` etc. | Compact wrappers |
| T8 | E | Memory HTTP disabled tests | FR-005 |
| T9 | All | `go test ./...` | SC-001 full suite |
| T10 | G | `go test ./internal/mcpserver/ -run Search -count=1` | SC-004 focused feedback |

Run order per slice: move code → `go test ./internal/mcpserver/` → `go test ./...`. Do not
weaken assertions to green the build.

## Agent UX Validation Plan

Before final merge (after Slice F or G):

- **Fixture**: Existing `fakeBackend` or staging OpenGrok with stable data.
- **Task**: List projects → search definitions for known symbol → `read_file` with citation in
  answer.
- **Surfaces**: Spot-check `full`; one compact wrapper call; gateway discover + one operation.
- **Pass**: Identical tool list, schemas, and response fields vs pre-refactor branch snapshot.
- **Fail action**: Block merge; treat as contract regression (see equivalence contract).

Intermediate slices may skip agent UX check if `go test` is comprehensive for moved code.

## Complexity Tracking

> Empty — no constitution violations.

## Quickstart Reference

See [quickstart.md](./quickstart.md) for contributor navigation and slice workflow.

## Contracts Reference

See [contracts/behavior-equivalence-contract.md](./contracts/behavior-equivalence-contract.md)
for non-regression requirements.

## Next Step

Run **`/speckit-tasks`** to generate dependency-ordered `tasks.md`.
