---
description: "Task list for Compact Surface as Default"
---

# Tasks: Compact Surface as Default

**Input**: Design documents from `specs/006-compact-surface-default/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test-first (Constitution III). Proving tests are written before the
implementation they cover and confirmed to fail against old behavior.

**Phase order note**: Phases follow the plan's **safety-first build sequence**
(research D4 / implementation-playbooks), not raw story priority. The default flip
(US4) is sequenced **after** the eval equivalence work (US5) because the flip is the
optimization and must be gated on proven parity + equivalence — flipping first is the
AP#3 "silent interface change" failure. Order: US1 → US2 → US3 → US5 → US4.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete dependencies)
- **[Story]**: US1–US5 (foundational/setup/polish tasks carry no story label)

## Path Conventions

- Core MCP behavior: `internal/mcpserver/`
- Config: `internal/config/`
- Eval harness: `evals/`
- Docs: `README.md`, `docs/`, `CHANGELOG.md`

---

## Phase 1: Setup

**Purpose**: Baseline and de-risk the keystone schema decision before building.

- [X] T001 [P] Establish a green baseline:
- [X] T002 Spike

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The discriminated-schema mechanism and input handling that BOTH P1
stories build on.

**⚠️ CRITICAL**: No user-story work begins until this phase is complete.

- [X] T003 Add proving test in `internal/mcpserver/compact_schema_test.go` that the composer produces a valid discriminated schema from a set of operation input types (required fields/enums present per branch) and that the SDK validator accepts a valid call and rejects an unknown operation / missing required field.
- [X] T004 Implement the discriminated-schema composer in `internal/mcpserver/compact_schema.go`: given a compact tool's enabled operations, generate each branch with `jsonschema.For[T]()` over the operation's input type and compose a top-level schema requiring `operation` (enum of enabled ops) discriminated per research D2; return a `*jsonschema.Schema` for `mcp.Tool.InputSchema`.
- [X] T005 [P] Adjust compact input handling in `internal/mcpserver/types.go` for flattened `{operation, …fields}` decode (read `operation`, decode the remaining args into the operation's existing input type), removing the `payload` wrapper assumption.
- [X] T006 [P] Extend the scalar coercer in `internal/mcpserver/coerce.go` to register, per compact tool, the union of scalar fields across its operations (research D3), with a proving case in `internal/mcpserver/coerce_test.go` for a string-encoded scalar (e.g. `tokenized:"true"`).

**Checkpoint**: schema composition + flattened decode + coercion ready.

---

## Phase 3: User Story 1 - Consolidated, non-overlapping surface (Priority: P1) 🎯 MVP

**Goal**: 4 tools (`opengrok_projects`, `opengrok_search`, `opengrok_symbols`,
`opengrok_read`), each owning one job; all symbol/reference work in `opengrok_symbols`;
`opengrok_compound` and `opengrok_memory` removed; actionable errors.

**Independent Test**: Inspect the compact tool list — each task in the suite maps to
exactly one tool+operation, no operation is duplicated across tools, and an unknown
operation / missing field returns an actionable error.

### Tests for User Story 1 (write first)

- [X] T007 [P] [US1] In `internal/mcpserver/register_test.go`, assert the compact surface registers exactly `{opengrok_projects, opengrok_search, opengrok_symbols, opengrok_read}` (no `opengrok_compound`, no `opengrok_memory`) and that no operation token appears in more than one tool (no overlap, FR-001/002, SC-002).
- [X] T008 [P] [US1] In `internal/mcpserver/compact_test.go`, assert each operation routes to the correct service method (e.g. `opengrok_symbols.find` → `FindSymbolAndReferences`, `opengrok_search.read` → `SearchAndRead`) and that an unknown operation and a missing required field return actionable errors naming valid operations / the missing field (FR-006).

### Implementation for User Story 1

- [X] T009 [US1] Rewrite `internal/mcpserver/register_compact.go` to register the 4 consolidated tools with names + per-operation enums from `contracts/tool-interface-spec.md`, wiring `InputSchema` via the T004 composer; remove `opengrok_compound` and `opengrok_memory` registration.
- [X] T010 [US1] Rewrite dispatch in `internal/mcpserver/compact.go`: `CompactSearch` (code, read), `CompactSymbols` (definitions, references, find, implementations, cross_project, list), `CompactRead` (file, context); flattened per-operation decode; `unknownOperationError` lists enabled operations.
- [X] T011 [US1] Update compact input types in `internal/mcpserver/types.go` to match the flattened operations; delete the obsolete `CompactCompoundInput`/`CompactMemoryInput` envelope types.
- [X] T012 [US1] Run `go test ./internal/mcpserver/ -run 'Compact|Register' -count=1`.

**Checkpoint**: a coherent, non-overlapping compact surface (selectable via `OPENGROK_MCP_TOOL_SURFACE=compact`).

---

## Phase 4: User Story 2 - Typed per-operation schemas (Priority: P1)

**Goal**: Each operation exposes a typed schema (required fields, types, enums)
discoverable from introspection alone; calls constructible from schema; scalars coerced.

**Independent Test**: Introspect each tool's input schema and confirm every operation's
required fields/types/enums are present (none prose-only); construct a valid call from
the schema without reading the description.

**Depends on**: US1 (integrates with the registered tools).

### Tests for User Story 2 (write first)

- [X] T013 [P] [US2] In `internal/mcpserver/compact_schema_test.go`, for every tool+operation assert the introspected schema branch declares the operation's required fields, types, and enum values, cross-checked against the source input type (zero prose-only required fields, FR-004/SC-004).
- [X] T014 [P] [US2] In `internal/mcpserver/compact_test.go`, add a schema-only construction case (build a call from the schema branch, assert accepted) and a scalar-coercion case for a flattened operation field.

### Implementation for User Story 2

- [X] T015 [US2] Ensure `internal/mcpserver/register_compact.go` sets the T004 discriminated schema on every compact tool and that each operation branch reuses the live input type (single source of truth, no drift from full).
- [X] T016 [US2] Author the L2 operation descriptions in `internal/mcpserver/register_compact.go` from `contracts/tool-interface-spec.md` (purpose, when-to-use, gotchas; at ≥ full depth, FR-005).
- [X] T017 [US2] Run `go test ./internal/mcpserver/ -run 'Compact|Schema' -count=1`.

**Checkpoint**: compact is non-overlapping AND schema-complete — the MVP polished surface.

---

## Phase 5: User Story 3 - Capability parity + registration gating (Priority: P2)

**Goal**: Add `opengrok_projects.files`/`.overview` (close the parity gaps); enforce
tool-level + operation-level capability gating; confirm memory is omitted.

**Independent Test**: Every full code-intelligence capability maps to a compact op;
project file-listing and overview work; with a capability disabled its tool/ops are not
registered; no `opengrok_memory` tool is ever present.

**Depends on**: US1 (the `opengrok_projects` tool exists).

### Tests for User Story 3 (write first)

- [X] T018 [P] [US3] In `internal/mcpserver/register_test.go`, assert tool-level gating: with `ListProjects`+`ListFiles` capabilities off, `opengrok_projects` is absent from `ListTools` (FR-013/SC-013); with `ListSymbols` off, `opengrok_symbols.list` is absent from the enum but other symbol ops remain; assert `opengrok_memory` is never registered (FR-014).
- [X] T019 [P] [US3] In `internal/mcpserver/compact_test.go`, assert `opengrok_projects.files` returns a file listing and `opengrok_projects.overview` returns a language breakdown equivalent to full `list_files`/`get_project_overview`; add a parity-coverage assertion mirroring `contracts/migration-map.md` (every full code-intelligence tool has a compact target; memory excepted).

### Implementation for User Story 3

- [X] T020 [US3] Add `files` and `overview` operations to `opengrok_projects` in `internal/mcpserver/register_compact.go` and dispatch in `internal/mcpserver/compact.go` (`CompactProjects` → `ListFiles`/`GetProjectOverview`; gate `overview` on `ListFiles`).
- [X] T021 [US3] Implement tool-level registration gating in `internal/mcpserver/register_compact.go`: register a compact tool only when ≥1 of its operations has a verified capability, and build each tool's operation enum from the available operations (FR-013).
- [X] T022 [US3] Run `go test ./internal/mcpserver/ -count=1`.

**Checkpoint**: compact has full code-intelligence parity and gates like full.

---

## Phase 6: User Story 5 - Compact as a first-class measured surface (Priority: P3)

**Goal**: Parameterize the contract suite onto compact, add compact-specific cases,
assert cross-surface equivalence, commit a compact baseline that gates CI.

**Independent Test**: `go test ./evals/` runs every contract scenario on compact;
cross-surface equivalence holds; the compact baseline exists and a forced compact
regression fails CI.

**Depends on**: US1–US3 (the new compact tools/ops must exist to resolve onto).

### Tests / harness for User Story 5

- [X] T023 [P] [US5] Update `evals/surface.go` `resolveCompact` to emit the new tools/operations with flattened args, add the `files.list`→`opengrok_projects.files` mapping (remove the "no compact equivalent" skip), and add `projects.overview`; update `evals/surface_test.go` (FR-023/SC-012).
- [X] T024 [US5] Parameterize `TestEvalSuite` in `evals/evals_test.go` (and `evals/harness.go`) to run each contract scenario on both `full` and `compact` via the resolver — no scenario remains full-only (FR-019/SC-010).
- [X] T025 [P] [US5] Add compact-specific eval cases under `evals/testdata/`: `opengrok_projects.overview`, `opengrok_projects.files`, an invalid-`operation` error case, and a typed-schema-construction case (FR-020).
- [X] T026 [US5] Implement the cross-surface equivalence assertion in the harness (`evals/assert.go` or new `evals/equivalence.go`): compare decoded `*Output` fields (hits, `citation.url`, cursors/`total_*`, warnings) across surfaces and fail on divergence (FR-021/SC-011).
- [X] T027 [US5] Refresh and commit the compact baseline via `./scripts/update-eval-results.sh` (`evals/baselines/`), wire CI to fail on a compact contract regression, and update `evals/README.md` (compact = first-class measured surface; token deltas non-gating) (FR-022).
- [X] T028 [US5] Run `go test ./evals/ -count=1` and `go test ./evals/ -run TestTokenBenchmark -count=1`.

**Checkpoint**: compact equivalence + parity are enforced by the harness — the gate that authorizes the flip.

---

## Phase 7: User Story 4 - Compact becomes the default (Priority: P2)

**Goal**: Flip the shipped default to compact; keep `full` selectable + byte-for-byte
stable; ship the migration note and docs; drop compact's experimental framing.

**Independent Test**: Start with no surface env var → compact tools registered; set
`OPENGROK_MCP_TOOL_SURFACE=full` → full tools registered, unchanged.

**⚠️ Gated**: The flip task (T030) MUST NOT land until Phase 6 (US5) is green —
parity + equivalence proven (plan D4, safety-first).

### Tests for User Story 4 (write first)

- [X] T029 [P] [US4] In `internal/config/config_test.go`, assert `Default().ToolSurface == compact`, that `OPENGROK_MCP_TOOL_SURFACE=full` still yields full, and (in `internal/mcpserver/register_test.go`) that the full surface registration is unchanged from baseline (FR-009/010, SC-007).

### Implementation for User Story 4

- [X] T030 [US4] Flip the default in `internal/config/config.go` `Default()`: `ToolSurface: ToolSurfaceFull` → `ToolSurfaceCompact` (gated on Phase 6 green).
- [X] T031 [P] [US4] Write the migration note (new `docs/` page + `CHANGELOG.md`) from `contracts/migration-map.md`: default change, restore path (`OPENGROK_MCP_TOOL_SURFACE=full`), and prior-compact/full → new compact mapping (FR-011).
- [X] T032 [US4] Remove "experimental"/"non-default" framing of compact in tool descriptions (`internal/mcpserver/register_compact.go`) and config commentary; keep gateway labeled experimental (FR-016).
- [X] T033 [P] [US4] Update `README.md`, `docs/configuration.md`, `docs/tool-contracts.md`, `docs/limitations.md`, and `docs/agent-usage-patterns.md` for compact-as-default (incl. memory-omitted-from-compact note).
- [X] T034 [US4] Run `go test ./internal/config/ ./internal/mcpserver/ -count=1`.

**Checkpoint**: compact is the shipped default; full remains a stable opt-in.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T035 [P] Documentation reconciliation gate: walk every row of `docs/README.md` (source-of-truth map) and update the single home of each affected concern (or mark N/A); do not restate canon across docs.
- [X] T036 Fresh-subagent UX validation (Constitution I / quickstart G17): dispatch a fresh lightweight/mid-tier subagent with a realistic task and minimal context; capture first-use findings on tool selection, schema legibility, descriptions, warnings, errors.
- [X] T037 Run an agent-ergonomics review of the compact surface (agent-ergonomics-inspector); confirm no Critical findings and Tool & Interface / Economic scores ≥ full; attach the report (SC-006 / quickstart G18).
- [X] T038 [P] Run `gofmt -w` on changed Go files and `git diff --check`.
- [X] T039 Execute the `quickstart.md` go/no-go gate (G1–G18) and the real-instance check against `https://opengrok.home/` (FR-017).
- [X] T040 Run `go test ./...` (full verification, Constitution III).

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (P1)** → no deps.
- **Foundational (P2)** → after Setup; **blocks all stories** (composer + decode + coercion).
- **US1 (Phase 3)** → after Foundational. MVP core.
- **US2 (Phase 4)** → after US1 (integrates with the registered tools).
- **US3 (Phase 5)** → after US1 (extends `opengrok_projects`, adds gating).
- **US5 (Phase 6)** → after US1–US3 (resolves onto the new tools/ops).
- **US4 (Phase 7)** → after US5 (**flip gated on proven equivalence/parity**, plan D4).
- **Polish (Phase 8)** → after US4.

### Critical gate

- T030 (the default flip) is blocked until T024+T026+T027 (parameterized suite +
  equivalence assertion + compact baseline) are green. This is the safety-first
  ordering — do not flip the default on an unproven surface.

### Parallel opportunities

- Setup: T001 ∥ (T002 after).
- Foundational: T005 ∥ T006 (after T003/T004).
- Per story, the test tasks marked [P] run together (different `_test.go` files):
  T007∥T008, T013∥T014, T018∥T019.
- US5: T023 ∥ T025; US4: T031 ∥ T033 ∥ T029.

---

## Parallel Example: User Story 1 tests

```bash
# Write the two proving tests together (different files), confirm they fail:
Task: "T007 register_test.go — assert 4-tool set, no overlap, no compound/memory"
Task: "T008 compact_test.go — assert operation routing + actionable errors"
```

---

## Implementation Strategy

### MVP (polished compact surface, default not yet flipped)

1. Phase 1 Setup → 2. Phase 2 Foundational → 3. Phase 3 US1 → 4. Phase 4 US2.
   **STOP & VALIDATE**: a consolidated, typed, non-overlapping compact surface usable
   via `OPENGROK_MCP_TOOL_SURFACE=compact`. This is the eligible-but-not-yet-default MVP.

### Incremental delivery

- + US3 → capability parity + gating (compact can do everything full can, minus memory).
- + US5 → eval harness proves equivalence/parity (the gate).
- + US4 → flip the default (safe, because US5 is green) + migration/docs.
- + Polish → fresh-subagent probe, ergonomics review, full verification, go/no-go gate.

### Notes

- [P] = different files, no incomplete dependency. `register_compact.go` and
  `compact.go` are touched across US1–US4, so those tasks are sequential, not parallel.
- Test-first: confirm each proving test fails against old behavior before implementing.
- Commit after each task or logical group (no co-author trailer).
- Do not flip the default (T030) until the equivalence gate (Phase 6) is green.
