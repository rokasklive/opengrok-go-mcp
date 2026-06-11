---
description: "Task list for Token Economy Eval"
---

# Tasks: Token Economy Eval

**Input**: Design documents from `/specs/005-token-economy-eval/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (user stories),
[research.md](./research.md), [data-model.md](./data-model.md),
[contracts/token-benchmark-contract.md](./contracts/token-benchmark-contract.md),
[quickstart.md](./quickstart.md)

**Tests**: Extend `evals/` with `tokens_test.go`, `surface_test.go`, `scenarios_test.go`,
and `token_benchmark_test.go`. Test-first for byte counting and adapters (plan Slice 1–2);
`TestTokenBenchmark` proves end-to-end (Slice 7). Contract eval `TestEvalSuite` must remain
green.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks in same phase)
- **[Story]**: `US1`–`US5` map to spec user stories by priority

## Story ↔ plan slice ↔ priority

| Story | Spec priority | Plan slice | Value |
|-------|---------------|------------|-------|
| US1 — Run token benchmark in CI | P1 | Slices 4–7 | **MVP** — reports published, no threshold gate |
| US2 — Compare surfaces on same scenario | P1 | Slices 5–6 | Cross-surface + cold/warm gateway tables |
| US3 — Identify obese tool schemas/responses | P2 | Slices 1, 6 | `schema_bytes_by_tool` + top offenders |
| US4 — Add scenarios without Go changes | P2 | Slices 3 | JSON scenario corpus |
| US5 — Diagnose response bloat composition | P3 | Slices 1, 6 | Text vs structured bytes in report |

**Build order**: Setup → Foundational (models, tokens, surface, harness) → **US4** (scenarios
JSON + loader — needed before full benchmark) → **US1** (runner + test) → **US2** (surface
comparison report) → **US3** (schema/top-offender report) → **US5** (composition columns) →
Polish.

---

## Phase 1: Setup

**Purpose**: Confirm scope, references, and directory layout

- [x] T001 Review [plan.md](./plan.md), [research.md](./research.md), and [contracts/token-benchmark-contract.md](./contracts/token-benchmark-contract.md) to confirm no server MCP contract changes
- [x] T002 Run baseline `go test ./evals/ -run TestEvalSuite -count=1` and confirm contract eval (004) is green before extending package
- [x] T003 Create `evals/testdata/scenarios/` directory for scenario JSON corpus

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Token models, byte counting, surface adapters, harness surface parameter —
blocks all user stories

**⚠️ CRITICAL**: Complete before benchmark runner work

- [x] T004 Implement `Scenario`, `ScenarioStep`, `SurfaceRun`, and `TokenBenchmarkResult` types in `evals/token_models.go` per [data-model.md](./data-model.md)
- [x] T005 [P] Implement byte counting helpers (`countListTools`, `countSchemaByTool`, `countCallToolRequest`, `countCallToolResponse` with text/structured split) in `evals/tokens.go`
- [x] T006 [P] Add cold/warm total helpers (`totalColdBytes`, `totalWarmBytes`, `estTokens`) in `evals/tokens.go` per data-model formulas
- [x] T007 [P] Add `evals/tokens_test.go` with fixed JSON fixtures proving byte counts and cold/warm math
- [x] T008 [P] Implement canonical-op → tool/args `Resolve(surface, op, args)` in `evals/surface.go` per [research.md](./research.md) R3 registry
- [x] T009 [P] Add `evals/surface_test.go` covering full, compact, and gateway mappings; assert `files.list` skipped on compact
- [x] T010 Extend `evals/harness.go` `Start` with `Options{ToolSurface string}` setting `OPENGROK_MCP_TOOL_SURFACE` on subprocess env (default `full` for existing `TestMain`)
- [x] T011 [P] Add harness surface smoke in `evals/harness_test.go` or extend existing test — `ListTools` differs between `full` and `compact` subprocess
- [x] T012 Add `evals/token_report.json`, `evals/token_report.md`, and `evals/token_report.baseline.json` to `.gitignore`

**Checkpoint**: Types, token math, adapters, and surface-aware harness compile and unit-test green

---

## Phase 3: User Story 4 — Add scenarios without Go changes (Priority: P2)

**Goal**: Load scenarios from JSON; four v1 scenario files on disk; malformed JSON fails fast.
(FR-012, SC-008 partial — loader before full corpus in benchmark)

**Independent Test**: `go test ./evals/ -run TestLoadScenarios -count=1` loads four scenarios;
invalid JSON fails in loader test without subprocess.

**Note**: Implemented before US1 so the benchmark has a real corpus; story label US4
reflects spec ownership of scenario authoring.

### Implementation for User Story 4

- [x] T013 [US4] Implement `loadScenarios(dir string)` in `evals/scenarios.go` — read `evals/testdata/scenarios/*.json`, validate unique `id`, non-empty `steps`, known `op` values
- [x] T014 [P] [US4] Add `evals/testdata/scenarios/symbol_investigation_granular.json` (definitions → read.file → references; PaymentProcessor fixtures)
- [x] T015 [P] [US4] Add `evals/testdata/scenarios/text_search_and_read.json` (search.code → read.file; Engine fixtures)
- [x] T016 [P] [US4] Add `evals/testdata/scenarios/file_exploration.json` (files.list → path.search → read.file; compact skips list)
- [x] T017 [P] [US4] Add `evals/testdata/scenarios/compound_symbol_investigation.json` (compound.find_symbol; PaymentProcessor)
- [x] T018 [US4] Add `evals/scenarios_test.go` proving loader loads four scenarios and rejects duplicate ids / empty steps

**Checkpoint**: Scenario corpus and loader ready for benchmark runner

---

## Phase 4: User Story 1 — Run token economy benchmark in CI (Priority: P1) 🎯 MVP

**Goal**: `go test ./evals/ -run TestTokenBenchmark` runs hermetic benchmark, writes
`token_report.json` and `token_report.md`, does not fail on byte thresholds. (FR-001, FR-005,
FR-009, FR-010, FR-014, SC-001, SC-006)

**Independent Test**: `go test ./evals/ -run TestTokenBenchmark -count=1` exits 0; reports
exist; `pgrep -f opengrok-go-mcp` empty after run.

### Implementation for User Story 1

- [x] T019 [US1] Implement `RunScenario(ctx, harness, scenario, surface)` in `evals/benchmark.go` — resolve steps, skip unavailable ops, `CallTool`, accumulate byte ledger per step
- [x] T020 [US1] Implement `RunBenchmark(ctx, moduleRoot, testdataDir)` in `evals/benchmark.go` — loop surfaces `full`, `compact`, `gateway`; start harness per surface; replay all loaded scenarios
- [x] T021 [US1] Record `list_tools_bytes` and per-step request/response bytes in `evals/benchmark.go` using `evals/tokens.go`
- [x] T022 [US1] Implement `WriteTokenReports(result TokenBenchmarkResult, dir string)` in `evals/token_report.go` writing `evals/token_report.json` (minimal fields first)
- [x] T023 [US1] Implement `TestTokenBenchmark` in `evals/token_benchmark_test.go` — hermetic run, assert reports exist, assert test passes regardless of byte totals (no threshold gate)
- [x] T024 [US1] Run `go test ./evals/ -run TestTokenBenchmark -v -count=1` and fix fixture/adapter mismatches until green (SC-001)
- [x] T025 [US1] After benchmark test, verify `pgrep -f opengrok-go-mcp` returns no processes (SC-003 subprocess hygiene)

**Checkpoint**: MVP — token benchmark runs end-to-end and publishes JSON artifact

---

## Phase 5: User Story 2 — Compare surfaces on the same scenario (Priority: P1)

**Goal**: Report enables side-by-side comparison of full, compact, gateway per scenario with
`total_cold_bytes`, `total_warm_bytes`, `list_tools_bytes`, `call_count`; gateway cold includes
`discover_bytes`, warm excludes it. (FR-007, SC-002, SC-007)

**Independent Test**: `token_report.md` contains scenario×surface table; gateway cold &gt;
warm when discover is non-zero; full/compact cold equals warm.

### Implementation for User Story 2

- [x] T026 [US2] Implement gateway `discover_bytes` via `opengrok_discover` in `evals/benchmark.go` before scenario replay on gateway surface
- [x] T027 [US2] Populate `total_cold_bytes` and `total_warm_bytes` on each `SurfaceRun` in `evals/benchmark.go` per [data-model.md](./data-model.md) formulas
- [x] T028 [US2] Extend `evals/token_report.md` renderer with scenario×surface comparison table (cold/warm, `list_tools_bytes`, `call_count`, `est_tokens_*`)
- [x] T029 [US2] Document gateway cold vs warm semantics in `evals/token_report.md` header (discover in cold only)
- [x] T030 [US2] Assert in `evals/token_benchmark_test.go` that result contains three surfaces × four scenarios (12 `SurfaceRun` rows minimum, plus skips documented)

**Checkpoint**: Cross-surface economics visible in markdown without reading JSON

---

## Phase 6: User Story 3 — Identify obese tool definitions and bloated responses (Priority: P2)

**Goal**: `schema_bytes_by_tool` from `ListTools` only; top-offender fields on each row.
(FR-006 partial, FR-008, SC-004)

**Independent Test**: `jq` on `token_report.json` shows `schema_bytes_by_tool` map,
`largest_tool_schema_name`, `largest_response_step` per run.

### Implementation for User Story 3

- [x] T031 [US3] Populate `schema_bytes_by_tool` on each `SurfaceRun` in `evals/benchmark.go` using `countSchemaByTool` from `evals/tokens.go` (ListTools only — no per-call mixing)
- [x] T032 [US3] Track and set `largest_response_bytes`, `largest_response_step`, `largest_tool_schema_name`, and `largest_tool_schema_bytes` on each `SurfaceRun` in `evals/benchmark.go`
- [x] T033 [US3] Add “top offenders” section per scenario×surface in `evals/token_report.md`
- [x] T034 [US3] Extend `evals/token_report.json` serialization to include full metric set from [contracts/token-benchmark-contract.md](./contracts/token-benchmark-contract.md)

**Checkpoint**: Report answers “which tool schema and which step dominated bytes?”

---

## Phase 7: User Story 5 — Diagnose response bloat composition (Priority: P3)

**Goal**: `response_text_bytes` and `response_structured_bytes` reported per row; markdown
explains code/text vs wrapper overhead; `est_tokens` labeled heuristic. (FR-006, SC-005)

**Independent Test**: Report columns include text and structured sums; markdown labels
`est_tokens_*` as bytes÷4 heuristic.

### Implementation for User Story 5

- [x] T035 [US5] Aggregate `response_text_bytes` and `response_structured_bytes` on each `SurfaceRun` in `evals/benchmark.go` (sum of per-step channels; `response_bytes` = sum of both)
- [x] T036 [US5] Add text vs structured columns to scenario×surface table in `evals/token_report.md`
- [x] T037 [US5] Add markdown note when structured bytes dominate text bytes (wrapper/metadata signal)
- [x] T038 [US5] Label `est_tokens_cold` and `est_tokens_warm` as heuristic (not model-exact) in `evals/token_report.md`

**Checkpoint**: Response bloat diagnosable without opening raw tool payloads

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Docs, coexistence with contract eval, full module verification

- [x] T039 [P] Add token benchmark section to `evals/README.md` (entrypoint, metrics, cold/warm, compact skip)
- [x] T040 [P] Update `AGENTS.md` testing section with `go test ./evals/ -run TestTokenBenchmark -count=1`
- [x] T041 Run quickstart commands from [quickstart.md](./quickstart.md) and fix doc drift if any
- [x] T042 Run `gofmt -w` on new/changed `evals/*.go` files
- [x] T043 Run `go test ./evals/ -count=1` — both `TestEvalSuite` and `TestTokenBenchmark` green
- [x] T044 Run `go test ./... -count=1` for full module regression
- [x] T045 [P] Optional: upload `evals/token_report.*` as CI artifact in `.github/workflows/ci.yml` (non-blocking)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup — **blocks all user stories**
- **US4 scenarios (Phase 3)**: Depends on Foundational — provides corpus for benchmark
- **US1 benchmark (Phase 4)**: Depends on Foundational + US4 loader/scenarios
- **US2 surface comparison (Phase 5)**: Depends on US1 runner (extends reports)
- **US3 top offenders (Phase 6)**: Depends on US1 ledger; can parallel with US2 after US1
- **US5 composition (Phase 7)**: Depends on US1 byte ledger; can parallel with US2/US3 after US1
- **Polish (Phase 8)**: Depends on US1–US5 desired for v1

### User Story Dependencies

| Story | Depends on | Independent test |
|-------|------------|------------------|
| US4 | Foundational | `TestLoadScenarios` |
| US1 | Foundational, US4 | `TestTokenBenchmark` |
| US2 | US1 | Report tables + gateway cold/warm |
| US3 | US1 | `schema_bytes_by_tool` in JSON |
| US5 | US1 | Text/structured columns in MD |

### Parallel Opportunities

- **Phase 2**: T005–T009, T007–T009, T011 can run in parallel after T004
- **Phase 3**: T014–T017 scenario JSON files in parallel after T013
- **Phase 6–7**: US3 and US5 report tasks can proceed in parallel after US1 completes
- **Polish**: T039, T040, T045 in parallel

### Parallel Example: Foundational

```bash
# After T004 models land, in parallel:
Task: "evals/tokens.go + tokens_test.go"
Task: "evals/surface.go + surface_test.go"
Task: "harness surface smoke test"
```

### Parallel Example: Scenario JSON (US4)

```bash
# After T013 loader API exists:
Task: "evals/testdata/scenarios/symbol_investigation_granular.json"
Task: "evals/testdata/scenarios/text_search_and_read.json"
Task: "evals/testdata/scenarios/file_exploration.json"
Task: "evals/testdata/scenarios/compound_symbol_investigation.json"
```

---

## Implementation Strategy

### MVP First (US4 loader + US1 benchmark)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: US4 (scenarios on disk)
4. Complete Phase 4: US1 (`TestTokenBenchmark` green, reports exist, no threshold gate)
5. **STOP and VALIDATE**: MVP delivers CI artifact

### Incremental Delivery

1. Setup + Foundational → token math and adapters proven
2. US4 → scenario corpus loadable
3. US1 → end-to-end benchmark (MVP)
4. US2 → cross-surface comparison tables + gateway cold/warm
5. US3 → schema obesity + largest response step
6. US5 → text vs structured diagnosis
7. Polish → docs and full `go test ./...`

### Parallel Team Strategy

1. Team completes Setup + Foundational together
2. One developer: US4 scenario JSON + loader
3. Another: US1 benchmark runner (after loader API)
4. After US1: split US2 (report tables), US3 (top offenders), US5 (composition columns)

---

## Notes

- Do not change `internal/mcpserver/` or MCP tool contracts for this feature
- `TestEvalSuite` must stay green — token benchmark is additive
- v1: no byte threshold CI failure — `TestTokenBenchmark` asserts artifacts, not ceilings
- Compact `files.list` skip must appear in `skipped_steps`, not silent omission
- `est_tokens` = integer `bytes / 4` — document as heuristic in markdown only
