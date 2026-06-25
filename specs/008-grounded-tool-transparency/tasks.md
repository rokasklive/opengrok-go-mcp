---
description: "Task list for Grounded, Test-Backed Tool Transparency (008)"
---

# Tasks: Grounded, Test-Backed Tool Transparency

**Input**: Design documents from `/specs/008-grounded-tool-transparency/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: REQUIRED. This feature's premise is test-backed claims, and constitution
Principle III mandates proving tests for behavioral changes. Every slice is test-first.

**Phase order note**: Phases follow the **safety-first build order documented in plan.md**,
not strict priority — the registry must exist before its consumers, and the highest-risk
contract change (errors) is proven before the visible one (descriptions). Story priority is
shown per phase. This deviation is constitution-sanctioned (Principle III: "if another
sequence is clearer, the plan MUST document why").

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependencies)
- **[Story]**: US1–US5 from spec.md

---

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Capture the OpenGrok `help.jsp` supported-syntax section into `evals/testdata/help-syntax.snapshot.md` as the pinned ground-truth authoring source (per research.md R3)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The claim registry is the single source of truth that BOTH US1 (descriptions)
and US2 (conformance) consume. No description or conformance work begins until it exists.

**⚠️ CRITICAL**: Blocks US1 and US2.

- [X] T002 [P] Define the `Claim` struct and registry accessors (fields per data-model.md Entity 1: id, category, support_status, condition, agent_claim_text, example, applies_to, disclosure_location, ground_truth_source, conformance_test_ref, positive_assertion, negative_control, gate) in `internal/mcpserver/claims.go`
- [X] T003 Seed the registry from `contracts/claim-registry.md` (supported/conditional/unsupported/limitation claims, grounded in the T001 snapshot) in `internal/mcpserver/claims.go`
- [X] T004 Add the claim-check registration helper and the always-on **bijection test** (every claim with `gate≠none` names a resolvable `conformance_test_ref`; once checks register, assert set-equality so an orphan claim OR orphan test fails the build) in `internal/mcpserver/claims_test.go`

**Checkpoint**: Registry exists; bijection guard active (initially flags claims lacking checks — those are filled by US2/US3/US5).

---

## Phase 3: User Story 5 - Regression locks (Priority: P3)

**Goal**: Pin the three already-correct behaviors the audit misreported, so they cannot
regress during later refactors.

**Independent Test**: Always-on (no backend) tests pass: `projects[]` accepted+applied, string
scalars coerced, project-less call resolves to the default.

**Why early (not P-order)**: cheap, backend-free, and they guard every subsequent refactor (plan.md build order step 1).

- [X] T005 [P] [US5] Regression test: a `projects:["a","b"]` array validates and is applied to scoping; register claim `projects-array` — `internal/mcpserver/regression_test.go`
- [X] T006 [P] [US5] Regression test: string-encoded scalar (`before:"10"`) is coerced to int and accepted; register claim `scalar-coercion` — `internal/mcpserver/coerce_test.go`
- [X] T007 [P] [US5] Regression test: `project` omitted + default configured resolves to the default with no discovery call; register claim `default-project` — `internal/mcpserver/helpers_test.go`

**Checkpoint**: Three claims closed in the bijection; existing behavior locked.

---

## Phase 4: User Story 3 - Specific, actionable errors (Priority: P2)

**Goal**: Replace the opaque `oneOf` with cause-specific structured errors carrying a
`suggestion`; keep response states distinct.

**Independent Test**: Each failure mode yields its named error class (not raw `-32602`); a
zero-result search is a labeled empty state.

### Tests (write first; T008–T013 confirm the opaque/old behavior fails)

- [X] T008 [P] [US3] Contract tests for the four validation classes — `UNKNOWN_OPERATION` (e.g. `opengrok_read` op=read names enabled ops), `MISSING_REQUIRED_FIELD`, `INVALID_FIELD_TYPE`, `UNKNOWN_FIELD` — assert a structured `ToolErrorBody` with `suggestion`, not `oneOf: did not validate`, in `internal/mcpserver/validation_test.go`
- [X] T009 [P] [US3] Contract test: malformed Lucene (HTTP 400 via fake client) maps to `QUERY_PARSER_FAILED` distinct from `UPSTREAM_HTTP_ERROR`, with corrective guidance, in `internal/mcpserver/tool_errors_test.go`
- [X] T010 [P] [US3] Contract test: zero-hit search returns `total_hits=0`, `IsError=false` (labeled empty), distinct from error, in `internal/mcpserver/search_test.go`

### Implementation

- [X] T011 [US3] Add `Suggestion` field and new codes (`UNKNOWN_OPERATION`, `MISSING_REQUIRED_FIELD`, `INVALID_FIELD_TYPE`, `UNKNOWN_FIELD`, `QUERY_PARSER_FAILED`) to `ToolErrorBody`/mapping in `internal/mcpserver/tool_errors.go`
- [X] T012 [US3] Implement the pre-validation middleware (checks operation → required → type → unknown-field, returns the first matching structured error before the SDK validator) in `internal/mcpserver/validation.go`; register it alongside the scalar-coercer middleware
- [X] T013 [US3] Map upstream Lucene 400 → `QUERY_PARSER_FAILED` with `/…/`/quote guidance in `internal/mcpserver/tool_errors.go`
- [X] T014 [US3] Run `go test ./internal/mcpserver/ -run 'Err|Validation|Empty'`

**Checkpoint**: The four classes + query-parser + empty-state are specific and tested.

---

## Phase 5: User Story 4 - Lean, honest default surface (Priority: P2)

**Goal**: Gate diagnostics off by default and remove schema slimming (must precede US1 —
descriptions need the full schema to render legal values).

**Independent Test**: No `diagnostics` block unless enabled; compact field docs equal full.

### Tests (write first)

- [X] T015 [P] [US4] Snapshot test: `diagnostics` absent by default, present iff `OPENGROK_MCP_DIAGNOSTICS=true`, in `internal/mcpserver/diagnostics_test.go`
- [X] T016 [P] [US4] Schema test: compact field descriptions equal full for `mode`/`sort`/`context_budget` (no slimming), in `internal/mcpserver/compact_schema_test.go`

### Implementation

- [X] T017 [US4] Add `Config.Diagnostics` + parse `OPENGROK_MCP_DIAGNOSTICS` (shared `ParseBool` convention, default off) in `internal/config/config.go`; wire into `cmd/opengrok-go-mcp/main.go` + startup log
- [X] T018 [US4] Populate the `diagnostics` block only when `cfg.Diagnostics` and add `omitempty` in `internal/mcpserver/types.go`, `search_core.go`, `results.go`
- [X] T019 [US4] Remove `slimSchema`/`slimSchemaInPlace`/`schemaForCompactType` slimming; render full field docs on compact in `internal/mcpserver/compact_schema.go` + `register_compact.go`
- [X] T020 [US4] Run `go test ./internal/mcpserver/ ./internal/config/ -run 'Diagnostic|Slim|Schema'`

**Checkpoint**: Default responses lean; compact carries full ground truth.

---

## Phase 6: User Story 1 - Honest descriptions (Priority: P1) 🎯 MVP value

**Goal**: Render descriptions from the registry — OpenGrok nature, supported/unsupported
syntax, example, named default project — so a cold agent stops making AST/regex assumptions.

**Independent Test**: ListTools snapshot shows the required slots on the default surface; a
cold-agent task that invites an inheritance assumption no longer issues an unsupported query.

### Tests (write first)

- [X] T021 [P] [US1] ListTools snapshot test: each compact tool description contains the nature claim (text+ctags, not AST/inheritance), the relevant supported + unsupported syntax (regex via `/…/`), ≥1 example, and the named default project; and references only registry `claim_id`s (no orphan prose), in `internal/mcpserver/compact_descriptions_test.go`

### Implementation

- [X] T022 [US1] Render the description slots from the registry (nature/supported/unsupported/example/default-project name from `cfg.DefaultProject`) per `contracts/description-contract.md` in `internal/mcpserver/compact_descriptions.go`
- [X] T023 [US1] Verify `full` and gateway surface descriptions stay coherent with the registry (no prose claiming an unsupported form) in `internal/mcpserver/register_full.go` and `register_gateway.go`
- [X] T024 [US1] Run `go test ./internal/mcpserver/ -run 'Descript|Compact'`

**Checkpoint**: Honest, registry-driven descriptions on every surface.

---

## Phase 7: User Story 2 - Test-backed claims (Priority: P1)

**Goal**: A live conformance suite proves every claimed form behaves as documented (positive +
negative control) and closes the bijection.

**Independent Test**: Against a live OpenGrok, every supported claim's example is accepted and
discriminates from its wrong-control; every unsupported claim is rejected; bijection set-equal.

### Tests / harness (write first)

- [X] T025 [P] [US2] Build the live conformance harness ranging over `gate=live` claims — issue each `example` through the MCP surface, assert `positive_assertion` holds and `negative_control` is discriminated, gated on `OPENGROK_MCP_LIVE_EVAL=1` + `OPENGROK_MCP_BASE_URL`; each case registers its `claim_id` — in `evals/conformance_test.go`

### Implementation

- [X] T026 [US2] Wire conformance cases for every supported/conditional claim (phrase, wildcard `*`/`?`, leading-wildcard, `defs:`/`refs:`/`path:`/`hist:`/`type:`, boolean, fuzzy `~`, proximity, range, `/regex/`, path-regex, auto-quote) and the negatives (`bare-regex`→`QUERY_PARSER_FAILED`, `wildcard-in-phrase` literal) in `evals/conformance_test.go`
- [X] T027 [US2] Tighten the bijection test (T004) to assert full set-equality now that all live/always-on checks register; resolve any orphan claim or test in `internal/mcpserver/claims_test.go`
- [X] T028 [US2] Run `OPENGROK_MCP_LIVE_EVAL=1 OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1 go test ./evals/ -run Conformance -count=1`

**Checkpoint**: Every claim is behaviorally backed; build fails on any future orphan.

---

## Phase 8: Evaluation Validity & Agent-UX (cross-cutting — SC-001/002/006, FR-020–024)

**Purpose**: Make the success metrics valid (Pass^k, deterministic trajectory grading, dual
metric, cost-per-successful-task) — this is also the constitution's Agent-UX validation.

- [X] T029 [P] Add the cost-per-successful-task metric to the token-economy benchmark (sum trajectory tokens incl. retries; demote per-response payload/schema bytes to a secondary anomaly check) in `evals/benchmark.go`
- [X] T030 [P] Build the cold-agent trajectory eval: a **deterministic** grader over the tool-call stream (no AST/inheritance or bare-regex query issued; a failed call's next action addresses the named cause), a **dual** outcome+trajectory metric, reported as **Pass^k** (k=5 default), in `evals/trajectory_test.go`
- [X] T031 Seed the trajectory eval set from the originating audit scenario ("find subclasses of <Type> in <project>") into `evals/testdata/`, marked rotatable (not frozen)
- [X] T032 Run `go test ./evals/ -count=1` and record the baseline cost-per-successful-task

**Checkpoint**: Metrics measure purpose, not proxies; SC-001/002/006 are gradeable.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T033 [P] Update docs: `docs/limitations.md` (text+ctags vs AST; regex `/…/` nuance; best-effort/page-local), `docs/tool-contracts.md` (new error codes + `suggestion`, diagnostics default-off, no-slimming schema policy), `docs/configuration.md` (`OPENGROK_MCP_DIAGNOSTICS`), `docs/agent-usage-patterns.md` (query syntax), `README.md` (config), `CHANGELOG.md` (migration notes for the four public-contract changes)
- [X] T034 [P] Add the review gate (`contracts/review-gate.md` G1–G6) into `docs/review-checklist.md`; record the no-slimming + transparent-but-bounded rule in `docs/agent-ux.md`
- [X] T035 Documentation reconciliation gate: walk every row of `docs/README.md` (source-of-truth map), update the single home of each affected concern or mark N/A — no restating canon across docs
- [X] T036 Dispatch a fresh lightweight/mid-tier subagent with the audit first-use task and minimal context; capture first-use findings on descriptions, schemas, warnings, defaults, examples (Principle I) — cold Sonnet agent ran the audit "find subclasses" task against the rendered compact tool JSON; confirmed it did NOT fall into the AST/inheritance or bare-regex trap (quoted the `opengrok-nature`/`inheritance`/`bare-regex` claims). Surfaced a real contract bug: the `default-project` claim's search-shaped example `{"operation":"code","query":"Engine"}` was rendered as the "Example:" on `opengrok_projects`/`opengrok_symbols`/`opengrok_read`, where no `code` operation or `query` field exists — a broken first call. Fixed by rendering per-tool schema-valid examples and adding `TestCompactToolExamplesValidateAgainstSchema` (guards operation∈enabled-ops and fields⊆schema; verified it fails on the re-injected bug). Deferred non-blocking UX findings recorded in memory.
- [X] T037 [P] Run `gofmt -w` on all changed Go files; `git diff --check` for whitespace — gofmt -l clean, git diff --check clean, go vet clean
- [X] T038 Run `go test ./...` (full verification) and `go test ./evals/ -count=1` — all packages pass; evals ran uncached (~112s)
- [X] T039 Run `graphify update .` to refresh the knowledge graph after code changes — AST re-extraction, no topology change

---

## Dependencies & Execution Order

### Phase dependencies (safety-first, per plan.md)

- **Setup (P1)** → **Foundational (P2: registry + bijection)** blocks US1 and US2.
- **US5 (P3)** — after Foundational; independent; placed early as cheap guards.
- **US3 (P2 errors)** — after Foundational; independent of US1/US4/US5.
- **US4 (P2 diagnostics + de-slimming)** — after Foundational; **must precede US1** (descriptions need the un-slimmed schema).
- **US1 (P1 descriptions)** — after Foundational + US4.
- **US2 (P1 conformance)** — after Foundational + US1 (examples exist) + a live backend.
- **Phase 8 (eval validity)** — after US1+US3 exist to grade; **Phase 9 (polish/docs)** last.

### Critical path

T001 → T002 → T003 → T004 → (US4: T017–T019) → (US1: T022) → (US2: T025–T027) → T038.

### Parallel opportunities

- Foundational T002 is [P] vs unrelated work; T003 depends on T002 (same file).
- US5 T005/T006/T007 are fully parallel (different files).
- US3 test tasks T008/T009/T010 parallel; impl T011–T013 serialize on `tool_errors.go`.
- US4 T015/T016 parallel; T017 (config) ∥ T019 (schema) are different files.
- Docs T033/T034 parallel; T037 parallel with doc edits.
- **US3, US4, US5 can be worked in parallel by different developers** once Foundational is done; US1 then US2 follow.

---

## Parallel Example: User Story 3

```bash
# Tests first, in parallel (different files):
Task: "T008 validation-class error contract tests in internal/mcpserver/validation_test.go"
Task: "T009 QUERY_PARSER_FAILED test in internal/mcpserver/tool_errors_test.go"
Task: "T010 zero-result empty-state test in internal/mcpserver/search_test.go"
# Then implement (serialize where same file):
Task: "T011 + T013 error codes/suggestion/mapping in internal/mcpserver/tool_errors.go"
Task: "T012 pre-validation middleware in internal/mcpserver/validation.go"
```

---

## Implementation Strategy

### MVP scope

The headline agent-facing value is **US1 (honest descriptions)**, but it depends on the
Foundational registry and US4's de-slimming. **Minimum shippable honest-surface increment** =
Setup + Foundational + US4 + US1 (then validate with the Phase-8 trajectory eval). US3 (errors)
is co-critical and should land in the same release since it's what lets agents self-correct.

### Incremental delivery (recommended order)

Setup+Foundational → US5 (lock) → US3 (errors) → US4 (lean surface) → US1 (descriptions) →
US2 (live conformance) → Phase 8 (eval validity) → Phase 9 (docs + UX run). Each phase has a
checkpoint that is independently testable.

### Gate before completion

Run `contracts/review-gate.md` (G1–G6). The default-shape change (diagnostics) and
advertised-schema change (de-slimming) flip only after their tests + migration notes are green
— flipping a default before its guard is the AP#3 silent-interface-change trap.

---

## Notes

- [P] = different files, no incomplete-task dependency. [Story] maps to spec.md user stories.
- Tests are written first per slice; for US5 they lock existing (passing) behavior by design.
- The claim registry is the contract spine: never hand-edit syntax prose or add a conformance
  test without a `claim_id` — the bijection test (T004/T027) enforces this.
- Open (non-blocking): confirm Pass^k k and thresholds, and the `OPENGROK_MCP_DIAGNOSTICS`
  name, during `/speckit-clarify` — neither reshapes these tasks.
- Commit after each task or logical group; omit the Claude co-author trailer.
