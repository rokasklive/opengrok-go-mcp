---
description: "Task list for Split MCP Server Monolith"
---

# Tasks: Split MCP Server Monolith

**Input**: Design documents from `/specs/003-split-mcp-server/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (user stories),
[research.md](./research.md), [data-model.md](./data-model.md),
[contracts/behavior-equivalence-contract.md](./contracts/behavior-equivalence-contract.md),
[quickstart.md](./quickstart.md)

**Tests**: Non-functional refactor — no new behavior. Verify with existing tests after each
slice (`go test ./internal/mcpserver/`). Do not weaken assertions. Optional golden JSON tests
only in Slice C if search moves need extra guard.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks in same phase)
- **[Story]**: `US1` (P1, behavioral equivalence), `US2` (P2, localized file layout),
  `US3` (P3, focused test files)

## Story ↔ plan slice ↔ priority

| Story | Spec priority | Plan slice | Value |
|-------|---------------|------------|-------|
| US1 — Preserve agent-facing MCP behavior | P1 | Verify after each slice + final | **MVP gate** — blocks merge on any drift |
| US2 — Localized maintenance (file split) | P2 | Slices A–F | Core refactor deliverable |
| US3 — Focused tests per concern | P3 | Slice G | SC-004 faster scoped feedback |

**Build order**: Foundational → **US2 Slice A** → US1 verify → **US2 B** → US1 verify → … →
**US2 F** → **US3 G** → Polish. US1 is not a separate code phase; it gates every slice.

---

## Phase 1: Setup

**Purpose**: Confirm scope and green baseline before moves

- [x] T001 Review [plan.md](./plan.md), [research.md](./research.md), and [contracts/behavior-equivalence-contract.md](./contracts/behavior-equivalence-contract.md) to confirm move-only scope and slice order before coding
- [x] T002 Run baseline `go test ./internal/mcpserver/ -count=1` and `go test ./...` — record pass as pre-refactor gate (SC-001)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Inventory and sequencing — no production logic moves yet

**⚠️ CRITICAL**: Complete before Slice A

- [x] T003 Map `internal/mcpserver/server.go` functions to target files per [data-model.md](./data-model.md) and [quickstart.md](./quickstart.md) (checklist in PR or task notes)
- [x] T004 [P] Confirm `cmd/opengrok-go-mcp/main.go` import surface stays `NewMCPServer`, `Backend`, `NewCachingBackend` only — no new public APIs in `internal/mcpserver/`

**Checkpoint**: Foundation ready — Slice A can begin

---

## Phase 3: User Story 2 — Slice A: Registration and resources (Priority: P2)

**Goal**: Extract `NewMCPServer`, all `register_*` functions, gateway registry, and MCP resources
from `internal/mcpserver/server.go` into dedicated files without logic edits. (FR-006, FR-010)

**Independent Test**: `TestCompactSurfaceDoesNotExposeContentOrMemoryWithoutCapabilities`,
`TestGatewayRegistry*` family pass unchanged.

### Implementation for Slice A

- [x] T005 [US2] Move `Service`, `Backend`, `Error`, `IsCode`, `NewService`, `listFilesWithMetadata` to `internal/mcpserver/service.go` from `internal/mcpserver/server.go` (move only)
- [x] T006 [P] [US2] Move `NewMCPServer` and surface switch to `internal/mcpserver/register.go` from `internal/mcpserver/server.go`
- [x] T007 [P] [US2] Move `registerFullTools` to `internal/mcpserver/register_full.go` from `internal/mcpserver/server.go`
- [x] T008 [P] [US2] Move `registerCompactTools` and `compactInputSchema` to `internal/mcpserver/register_compact.go` from `internal/mcpserver/server.go`
- [x] T009 [P] [US2] Move `buildGatewayRegistry`, `registerGatewayTools`, `gatewayOperation`, `memoryToolsEnabled` to `internal/mcpserver/register_gateway.go` from `internal/mcpserver/server.go`
- [x] T010 [P] [US2] Move resource handlers and URI parsers (`projectsResource`, `projectResource`, `fileResource`, `jsonResource`, `parseProjectResourceURI`, `parseLineFragment`) to `internal/mcpserver/resources.go` from `internal/mcpserver/server.go`
- [x] T011 [US2] Run `gofmt -w` on new files; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice A)

- [x] T012 [US1] Verify registration gating: `go test ./internal/mcpserver/ -run 'CompactSurface|GatewayRegistry' -count=1` — all pass with unchanged expectations (FR-001, SC-005)

**Checkpoint**: Registration lives outside monolith; tool names/schemas unchanged

---

## Phase 4: User Story 2 — Slice B: Projects and shared helpers (Priority: P2)

**Goal**: Extract project/file tools and cross-cutting helpers. (FR-006, FR-007)

**Independent Test**: List projects/files/overview tests pass; project resolution helpers
single-sourced in `helpers.go`.

### Implementation for Slice B

- [x] T013 [US2] Move shared helpers (`validateResponseMode`, `cursorValue`, `invalidCursorError`, `pageSize`, `includeLinks`, `shouldExpandContext`, `resolveBudgetTier`, `nextCursor`, project resolution, citation helpers) to `internal/mcpserver/helpers.go` from `internal/mcpserver/server.go`
- [x] T014 [US2] Move `ListProjects`, `ListFiles`, `GetProjectOverview`, `detectLanguage`, `projectSourceLabel`, `firstProject` to `internal/mcpserver/projects.go` from `internal/mcpserver/server.go`
- [x] T015 [US2] Run `gofmt -w` changed files; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice B)

- [x] T016 [US1] Verify project tools: `go test ./internal/mcpserver/ -run 'ListProjects|ListFiles|ProjectOverview' -count=1` — unchanged outputs (FR-002)

**Checkpoint**: Project family localized; helpers not duplicated

---

## Phase 5: User Story 2 — Slice C: Search pipeline (Priority: P2)

**Goal**: Extract search entrypoints and shared search/result shaping. (FR-006, FR-007)

**Independent Test**: Search, symbol search, cursor, and pagination tests pass.

### Implementation for Slice C

- [x] T017 [US2] Move search tool entrypoints (`SearchCode`, symbol searches, `SearchCrossProjectReferences`, `SearchImplementations`) to `internal/mcpserver/search_handlers.go` from `internal/mcpserver/server.go`
- [x] T018 [US2] Move `search()`, `searchRequest` type, `emptySearchOutput`, sort/max-hits helpers to `internal/mcpserver/search_core.go` from `internal/mcpserver/server.go`
- [x] T019 [US2] Move `results()`, expansion, `compactResults`, `applyMaxHitsPerFile`, `applySort`, `maybeExpandResults`, `expandResultContexts*` to `internal/mcpserver/results.go` from `internal/mcpserver/server.go`
- [x] T020 [P] [US1] Optional: add golden JSON fixture tests in `internal/mcpserver/testdata/` and proving tests in `internal/mcpserver/search_test.go` per [behavior-equivalence-contract.md](./contracts/behavior-equivalence-contract.md) if manual review shows risk
- [x] T021 [US2] Run `gofmt -w` changed files; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice C)

- [x] T022 [US1] Verify search contract: `go test ./internal/mcpserver/ -run 'Search|Cursor|Pagination' -count=1` — warnings, cursors, citations unchanged (FR-002, FR-003)

**Checkpoint**: Single search pipeline; no duplicate result shaping

---

## Phase 6: User Story 2 — Slice D: Symbols, file context, compound (Priority: P2)

**Goal**: Extract symbol listing, file read/context, and compound tools. (FR-008)

**Independent Test**: Symbol, `read_file`, `get_file_context`, compound tool tests pass.

### Implementation for Slice D

- [x] T023 [US2] Move `ListSymbols`, `validateSymbolReferenceCursor` to `internal/mcpserver/symbols.go` from `internal/mcpserver/server.go`
- [x] T024 [US2] Move `GetFileContext`, `windowedFileContext`, `pagedFileContext`, line helpers, and `read_file` handler (from inline closure) to `internal/mcpserver/filecontext.go` from `internal/mcpserver/server.go`
- [x] T025 [US2] Move `SearchAndRead`, `FindSymbolAndReferences` to `internal/mcpserver/compound.go` from `internal/mcpserver/server.go`
- [x] T026 [US2] Run `gofmt -w` changed files; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice D)

- [x] T027 [US1] Verify read/symbol/compound tools: `go test ./internal/mcpserver/ -run 'Symbol|FileContext|read_file|SearchAndRead|FindSymbol' -count=1` (FR-002)

**Checkpoint**: Read and symbol families localized

---

## Phase 7: User Story 2 — Slice E: Compact and memory handlers (Priority: P2)

**Goal**: Extract compact wrappers and direct memory tool methods. (FR-005, FR-006)

**Independent Test**: Compact payload tests and memory HTTP gating tests pass.

### Implementation for Slice E

- [x] T028 [US2] Move `CompactSearch`, `CompactSymbols`, `CompactRead`, `CompactCompound`, `compact*Operations`, `unknownOperationError` to `internal/mcpserver/compact.go` from `internal/mcpserver/server.go`
- [x] T029 [US2] Move `MemorySet/Get/List/Delete/Clear`, `CompactMemory`, `compactMemoryOperations` to `internal/mcpserver/memory_handlers.go` from `internal/mcpserver/server.go`
- [x] T030 [US2] Run `gofmt -w` changed files; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice E)

- [x] T031 [US1] Verify compact and memory: `go test ./internal/mcpserver/ -run 'Compact|Memory' -count=1` — including HTTP memory disabled (FR-005)

**Checkpoint**: All handlers moved out of monolith body

---

## Phase 8: User Story 2 — Slice F: Remove monolith and contributor map (Priority: P2)

**Goal**: Delete or empty `server.go`; add contributor README. (SC-002)

**Independent Test**: No `server.go` handler/registrar symbols remain; package docs map tool families to files.

### Implementation for Slice F

- [x] T032 [US2] Delete `internal/mcpserver/server.go` or reduce to zero duplicate symbols — ensure no dead registration paths (FR-011)
- [x] T033 [US2] Add `internal/mcpserver/README.md` contributor file map per [quickstart.md](./quickstart.md)
- [x] T034 [US2] Run `gofmt -w internal/mcpserver/`; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice F)

- [x] T035 [US1] Full equivalence gate: `go test ./... -count=1` — all tests pass without weakened assertions (SC-001, SC-005)

**Checkpoint**: Monolith removed; full suite green

---

## Phase 9: User Story 3 — Slice G: Split test files (Priority: P3)

**Goal**: Partition `server_test.go` by concern for faster scoped runs. (FR-009, SC-004)

**Independent Test**: `go test ./internal/mcpserver/ -run Search -count=1` completes in under half
the wall-clock time of full package test on the same machine.

### Implementation for Slice G

- [x] T036 [US3] Extract shared test fixtures (`fakeBackend`, helpers) to `internal/mcpserver/testutil_test.go` from `internal/mcpserver/server_test.go`
- [x] T037 [P] [US3] Move registration and gateway tests to `internal/mcpserver/register_test.go` and `internal/mcpserver/gateway_test.go` from `internal/mcpserver/server_test.go`
- [x] T038 [P] [US3] Move search-related tests to `internal/mcpserver/search_test.go` from `internal/mcpserver/server_test.go`
- [x] T039 [P] [US3] Move project tests to `internal/mcpserver/projects_test.go` from `internal/mcpserver/server_test.go`
- [x] T040 [P] [US3] Move symbol and file-context tests to `internal/mcpserver/symbols_test.go` and `internal/mcpserver/filecontext_test.go` from `internal/mcpserver/server_test.go`
- [x] T041 [P] [US3] Move compact tests to `internal/mcpserver/compact_test.go` from `internal/mcpserver/server_test.go`
- [x] T042 [P] [US3] Move resource tests to `internal/mcpserver/resources_test.go` from `internal/mcpserver/server_test.go`
- [x] T043 [US3] Remove or thin `internal/mcpserver/server_test.go`; add `internal/mcpserver/integration_test.go` only if cross-surface tests need a shared home
- [x] T044 [US3] Run `gofmt -w internal/mcpserver/`; `go test ./internal/mcpserver/ -count=1`

### User Story 1 gate (Slice G)

- [x] T045 [US1] Re-run `go test ./... -count=1` after test split — same pass count, no weakened assertions
- [x] T046 [US3] Measure scoped vs full package test time; document in PR notes for SC-004

**Checkpoint**: Focused test files; US3 independently validated

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Docs, agent UX, final verification

- [x] T047 [P] Update `AGENTS.md` repository map bullet for `internal/mcpserver/` to reference `internal/mcpserver/README.md` layout
- [x] T048 [P] Documentation reconciliation: walk `docs/README.md` — mark agent-facing docs N/A (no contract changes) or note internal-only README
- [x] T049 Dispatch fresh-session or mid-tier subagent with realistic task (*list projects, search symbol, read file with citation*) on fixed fixture; confirm identical tool list and outputs vs baseline (plan Agent UX Validation)
- [x] T050 Run `gofmt -w` on all changed Go files under `internal/mcpserver/`
- [x] T051 Run `go test ./...` final merge gate (SC-001)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup — **blocks all slices**
- **US2 Slices A–F (Phases 3–8)**: Sequential — each slice depends on previous slice compiling and testing green
- **US1 gates**: Run after each slice (T012, T016, T022, T027, T031, T035, T045)
- **US3 Slice G (Phase 9)**: Depends on Slice F complete
- **Polish (Phase 10)**: Depends on Slice G

### User Story Dependencies

- **US1 (P1)**: Gates every slice; not a separate code phase
- **US2 (P2)**: Slices A→F sequential; delivers file layout (SC-002, SC-003)
- **US3 (P3)**: After US2 complete; delivers test ergonomics (SC-004)

### Parallel Opportunities

- **Slice A**: T006–T010 [P] (different new files, after T005 `service.go` exists)
- **Slice C**: T020 optional golden tests parallel to T021 if added
- **Slice G**: T037–T042 [P] (different test files, after T036 `testutil_test.go`)
- **Polish**: T047, T048 [P]

### Parallel Example: Slice A

```bash
# After T005 (service.go) lands:
# Move registration files in parallel:
T007 register_full.go
T008 register_compact.go
T009 register_gateway.go
T010 resources.go
# Then T011 go test, T012 US1 gate
```

### Parallel Example: Slice G

```bash
# After T036 testutil_test.go:
go test ./internal/mcpserver/ -run 'Register|Gateway'   # register_test + gateway_test
go test ./internal/mcpserver/ -run Search              # search_test
go test ./internal/mcpserver/ -run 'ListProjects|ListFiles'  # projects_test
```

---

## Implementation Strategy

### MVP First (US1 through Slice A)

1. Complete Phase 1–2 (baseline + inventory)
2. Complete Slice A (registration extraction)
3. **STOP and VALIDATE**: T012 US1 gate — registration tests green
4. Continue B→F only if gate passes

### Incremental Delivery

1. One slice = one reviewable PR (or commit series)
2. Never merge a slice with failing `go test ./internal/mcpserver/`
3. Slice G (tests) optional as separate PR after F if F is already large

### Suggested PR sequence

| PR | Tasks | Story |
|----|-------|-------|
| PR1 | T001–T012 | Setup + Slice A |
| PR2 | T013–T016 | Slice B |
| PR3 | T017–T022 | Slice C |
| PR4 | T023–T027 | Slice D |
| PR5 | T028–T031 | Slice E |
| PR6 | T032–T035 | Slice F |
| PR7 | T036–T046 | Slice G |
| PR8 | T047–T051 | Polish |

---

## Notes

- **Move only**: no tool description, schema, warning, or default edits in refactor PRs
- If a move requires a logic fix, split into move commit then fix commit with proving test
- `[P]` within a slice assumes prior task in that slice that creates shared types is done
- `server.go` line count target: zero handler/registrar code after Slice F (SC-002)
- Commit after each slice or PR boundary
