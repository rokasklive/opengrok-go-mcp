---
description: "Task list for MCP Eval Harness"
---

# Tasks: MCP Eval Harness

**Input**: Design documents from `/specs/004-mcp-eval-harness/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (user stories),
[research.md](./research.md), [data-model.md](./data-model.md),
[contracts/eval-harness-contract.md](./contracts/eval-harness-contract.md),
[quickstart.md](./quickstart.md)

**Tests**: New `evals/` package — proving tests are `evals_test.go` (`TestEvalSuite`) plus
focused tests where helpful (`backend_test.go`, `loader_test.go`, `report_test.go`). Harness
skeleton + hermetic backend before full seed corpus (test-first per plan).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks in same phase)
- **[Story]**: `US1` (P1, end-to-end CI suite), `US2` (P2, JSON-only case authoring),
  `US3` (P3, baseline delta reports)

## Story ↔ plan slice ↔ priority

| Story | Spec priority | Plan slice | Value |
|-------|---------------|------------|-------|
| US1 — Run end-to-end eval suite in CI | P1 | Slices 2–6 | **MVP** — stdio subprocess + hermetic backend green |
| US2 — Add cases without Go changes | P2 | Slice 1 loader + validation | Dataset-driven scale |
| US3 — Compare reports across runs | P3 | Slice 5 delta reporting | PR regression visibility |

**Build order**: Setup → Foundational (models + fixtures) → **US1** (backend → harness → runner →
suite) → **US2** (loader hardening + docs) → **US3** (baseline deltas) → Polish.

---

## Phase 1: Setup

**Purpose**: Confirm scope, references, and package skeleton before implementation

- [x] T001 Review [plan.md](./plan.md), [research.md](./research.md), and [contracts/eval-harness-contract.md](./contracts/eval-harness-contract.md) to confirm no server contract changes and hermetic-default CI path
- [x] T002 Run baseline `go test ./... -count=1` and confirm `github.com/modelcontextprotocol/go-sdk` v1.4.x in `go.mod` matches server usage
- [x] T003 Create `evals/` package directory and empty `evals/evals_test.go` package declaration so `go test ./evals/` resolves

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared types, seed fixtures, and package layout — blocks all user stories

**⚠️ CRITICAL**: Complete before US1 harness wiring

- [x] T004 Implement `EvalCase`, `Expected`, `ResultCheck`, `EvalResult`, and `SuiteResult` types in `evals/models.go` per [data-model.md](./data-model.md)
- [x] T005 [P] Copy `.agents/skills/mcp-eval-harness/test_data_pack/evalcases/projects.json` to `evals/testdata/list_projects.json` (rename to match one-file-per-tool convention)
- [x] T006 [P] Copy `.agents/skills/mcp-eval-harness/test_data_pack/evalcases/search_code.json` to `evals/testdata/search_code.json`
- [x] T007 [P] Copy `.agents/skills/mcp-eval-harness/test_data_pack/evalcases/read_file.json` to `evals/testdata/read_file.json`
- [x] T008 [P] Copy `.agents/skills/mcp-eval-harness/test_data_pack/evalcases/symbols.json` to `evals/testdata/search_symbols.json`
- [x] T009 [P] Copy `.agents/skills/mcp-eval-harness/test_data_pack/opengrok/*.json` to `evals/testdata/opengrok/`
- [x] T010 Copy `.agents/skills/mcp-eval-harness/test_data_pack/manifest.json` to `evals/testdata/manifest.json`
- [x] T011 Implement `loadCases(dir string)` in `evals/runner.go` — read all `evals/testdata/*.json` (exclude `opengrok/` subdirectory), merge into case list

**Checkpoint**: Types and fixtures on disk; loader compiles

---

## Phase 3: User Story 1 — Run end-to-end eval suite in CI (Priority: P1) 🎯 MVP

**Goal**: `go test ./evals/` builds binary, starts hermetic backend, spawns stdio subprocess once,
runs seed corpus, writes reports, fails on judged case failure, skips gated tools. (FR-001–FR-008,
FR-010–FR-012, SC-001, SC-003, SC-005, SC-006)

**Independent Test**: `go test ./evals/ -run TestEvalSuite -count=1` green with no live OpenGrok;
`pgrep -f opengrok-go-mcp` empty after run; `evals/report.md` and `evals/report.json` exist.

### Hermetic backend (Slice 2)

- [x] T012 [US1] Implement manifest-driven `httptest` router in `evals/backend.go` per skill `backend-strategies.md` and `test_data_pack/README.md`
- [x] T013 [US1] Implement `startBackend(manifestPath string)` in `evals/backend.go` returning base URL, web URL, teardown, and subprocess env map (`OPENGROK_MCP_BASE_URL`, `OPENGROK_MCP_WEB_BASE_URL`, `OPENGROK_MCP_PROJECTS`, `OPENGROK_MCP_DEFAULT_PROJECT`, `OPENGROK_MCP_PROBE_FILE`, `OPENGROK_MCP_CURSOR_SECRET`)
- [x] T014 [P] [US1] Add `evals/backend_test.go` proving `/projects/indexed` and startup probe `/search` return HTTP 200 with fixtures from `evals/testdata/`

### Subprocess harness (Slice 3)

- [x] T015 [US1] Implement `buildBinary(ctx context.Context)` in `evals/harness.go` — `go build -o` temp `opengrok-go-mcp` from `cmd/opengrok-go-mcp`
- [x] T016 [US1] Implement `newHarness(ctx, binaryPath, env map[string]string)` in `evals/harness.go` using `mcp.CommandTransport` and `exec.Command`
- [x] T017 [US1] Call `ListTools` once in `evals/harness.go` and store registered tool name set for skip logic (FR-005)
- [x] T018 [US1] Implement teardown in `evals/harness.go` — `session.Close()`, transport cleanup, no orphan subprocess (FR-002, SC-003)

### Runner and asserts (Slice 4)

- [x] T019 [US1] Implement `RunCase(ctx, session, case EvalCase, registered map[string]bool)` in `evals/runner.go` — skip when tool ∉ registered; direct `CallTool` (FR-006)
- [x] T020 [US1] Parse `StructuredContent` or JSON `TextContent` into `map[string]any` in `evals/runner.go`
- [x] T021 [US1] Implement `evalCheck` for `no_error`, `has_results`, `field_present`, and `latency_ms` in `evals/assert.go` with dotted-path resolver (arrays: first element)
- [x] T022 [US1] Implement `RunSuite(ctx, harness, cases []EvalCase)` in `evals/runner.go` aggregating `[]EvalResult`

### Metrics and basic reports (Slice 5 partial)

- [x] T023 [US1] Implement `Aggregate(results []EvalResult)` in `evals/metrics.go` — pass/fail/skip counts, mean score, Coverage@K, per-tool scores, latency p50/p95 (SC-004, SC-005)
- [x] T024 [US1] Implement `WriteReports(result SuiteResult, dir string)` in `evals/report.go` writing `evals/report.md` and `evals/report.json` with per-tool table and failed case IDs (FR-008, SC-006)

### Full suite test (Slice 6)

- [x] T025 [US1] Implement `TestMain` in `evals/evals_test.go` — build → `startBackend` → harness → `loadCases` → `RunSuite` → `WriteReports` → teardown
- [x] T026 [US1] Implement `TestEvalSuite` in `evals/evals_test.go` asserting `SuiteResult.Failed == 0` for judged cases (or documented threshold)
- [x] T027 [US1] Run `go test ./evals/ -run TestEvalSuite -v -count=1` and fix seed case or fixture mismatches until green (SC-001)
- [x] T028 [US1] After `go test ./evals/ -count=1`, verify `pgrep -f opengrok-go-mcp` returns no processes (SC-003)

**Checkpoint**: MVP complete — hermetic eval suite runs via `go test ./evals/`

---

## Phase 4: User Story 2 — Add and maintain cases without Go changes (Priority: P2)

**Goal**: Contributors add cases by editing JSON only; malformed testdata fails fast with clear
errors. (FR-003, FR-004, SC-002)

**Independent Test**: Add a new case to `evals/testdata/search_code.json`; re-run suite without
Go changes; new case appears in report. Malformed JSON fails in loader test before subprocess.

### Implementation for User Story 2

- [x] T029 [US2] Harden `loadCases` in `evals/runner.go` — reject duplicate `id`, empty `tool`, empty `result_checks`, invalid check `type` with explicit error messages (FR-003)
- [x] T030 [US2] Ensure `loadCases` runs in `TestMain` before subprocess spawn so validation errors fail fast (spec edge case: startup failure vs load failure)
- [x] T031 [US2] Add `TestLoadCasesValidation` in `evals/loader_test.go` — malformed JSON and duplicate `id` fail with clear errors without starting subprocess
- [x] T032 [US2] Add one new eval case to `evals/testdata/search_code.json` demonstrating JSON-only addition; verify it runs in next `TestEvalSuite` (SC-002)
- [x] T033 [US2] Add `evals/README.md` documenting case JSON schema, check types, and one-file-per-tool layout per [quickstart.md](./quickstart.md)

**Checkpoint**: Case authoring documented; loader validation independent of harness

---

## Phase 5: User Story 3 — Compare eval reports across runs (Priority: P3)

**Goal**: Second suite run shows per-tool pass-rate and latency deltas when baseline JSON exists.
(FR-009, SC-004)

**Independent Test**: Copy `evals/report.json` to `evals/report.baseline.json`; run suite again;
`evals/report.md` includes Δ lines per tool.

### Implementation for User Story 3

- [x] T034 [US3] Implement `ReadBaseline(path string)` in `evals/report.go` loading prior `SuiteResult` from `evals/report.baseline.json` (or configurable path)
- [x] T035 [US3] Extend `WriteReports` in `evals/report.go` to include per-tool pass-rate Δ and latency Δ when baseline present; omit delta section when absent (FR-009)
- [x] T036 [US3] Add `TestReportDelta` in `evals/report_test.go` with synthetic `SuiteResult` pairs proving delta markdown sections render correctly

**Checkpoint**: Baseline comparison works for PR review

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Repo hygiene, docs map, full verification

- [x] T037 [P] Add `evals/report.json`, `evals/report.md`, and optional `evals/report.baseline.json` to `.gitignore` if reports should not be committed
- [x] T038 [P] Update `docs/README.md` documentation map row for eval harness maintainer docs (or mark N/A with pointer to `evals/README.md`)
- [x] T039 [P] Optional: add `TestEvalSuiteLive` in `evals/evals_test.go` gated on `OPENGROK_MCP_LIVE_EVAL=1` — skip by default (out of CI path)
- [x] T040 Run [quickstart.md](./quickstart.md) commands and fix any drift in paths or flags
- [x] T041 Run `gofmt -w` on all `evals/*.go` files
- [x] T042 Run `go test ./... -count=1` — full module green including `evals/` (FR-010)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Setup — **blocks all user stories**
- **User Story 1 (Phase 3)**: Depends on Foundational — **MVP**
- **User Story 2 (Phase 4)**: Depends on US1 `loadCases` and `TestEvalSuite` existing
- **User Story 3 (Phase 5)**: Depends on US1 `WriteReports` and `metrics.go`
- **Polish (Phase 6)**: Depends on US1 minimum; full polish after US2/US3

### User Story Dependencies

- **US1 (P1)**: After Foundational — no dependency on US2/US3
- **US2 (P2)**: After US1 runner/load path — independently testable via `loader_test.go`
- **US3 (P3)**: After US1 reports — independently testable via `report_test.go`

### Within User Story 1

```text
T012–T014 backend → T015–T018 harness → T019–T022 runner/assert → T023–T024 metrics/report → T025–T028 TestEvalSuite
```

### Parallel Opportunities

- **Phase 2**: T005–T010 (fixture copies) in parallel after T004
- **Phase 3**: T014 `backend_test.go` parallel with harness once backend API stable
- **Phase 3**: Seed case files already copied in Phase 2; tuning in T027 may edit multiple JSON files in parallel
- **Phase 6**: T037–T039 documentation/gitignore in parallel

---

## Parallel Example: Foundational fixtures

```bash
# After T004 models.go:
cp test_data_pack/evalcases/projects.json    → evals/testdata/list_projects.json
cp test_data_pack/evalcases/search_code.json → evals/testdata/search_code.json
cp test_data_pack/evalcases/read_file.json   → evals/testdata/read_file.json
cp test_data_pack/evalcases/symbols.json     → evals/testdata/search_symbols.json
cp test_data_pack/opengrok/*                 → evals/testdata/opengrok/
cp test_data_pack/manifest.json              → evals/testdata/manifest.json
```

---

## Parallel Example: User Story 1 backend + tests

```bash
# After T012 manifest router exists:
Task T014: evals/backend_test.go
Task T015: evals/harness.go buildBinary (different file, no conflict)
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (T012–T028)
4. **STOP and VALIDATE**: `go test ./evals/ -count=1` green; no orphan processes
5. Optional: commit MVP before US2/US3

### Incremental Delivery

1. Setup + Foundational → fixtures and types ready
2. US1 → full hermetic suite (**MVP**)
3. US2 → loader validation + contributor README
4. US3 → baseline deltas for PR review
5. Polish → gitignore, docs map, full `go test ./...`

### Parallel Team Strategy

1. One developer: US1 sequential (backend → harness → suite)
2. After US1 green: second developer can do US2 (`loader_test.go`, README) while third does US3 (`report_test.go`)

---

## Notes

- Do **not** change `internal/mcpserver/` tool behavior for this feature (FR-011)
- Direct-call mode only — no LLM tool-selection metrics in v1 headline report
- Hermetic `httptest` is default; live OpenGrok is optional polish (T039)
- Field names in case JSON must match `internal/mcpserver/types.go` JSON tags
- Commit after each slice or logical task group
