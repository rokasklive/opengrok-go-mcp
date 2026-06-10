---
description: "Task list for Minimal Setup Surface"
---

# Tasks: Minimal Setup Surface

**Input**: Design documents from `/specs/002-minimal-setup-surface/`

**Prerequisites**: [plan.md](./plan.md) (required), [spec.md](./spec.md) (user stories),
[research.md](./research.md), [data-model.md](./data-model.md),
[contracts/configuration-contract.md](./contracts/configuration-contract.md),
[quickstart.md](./quickstart.md)

**Tests**: Behavioral changes use focused tests that fail against old behavior. Non-trivial
slices are ordered test-first per the plan.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks in same phase)
- **[Story]**: `US1` (P1, base-URL-only startup), `US2` (P1, optional auth / 401 guidance),
  `US3` (P2, scrape default-on + opt-out), `US4` (P2, docs / examples)

## Story ↔ plan slice ↔ priority

| Story | Spec priority | Plan slice | Value |
|-------|---------------|------------|-------|
| US1 — Start with only the base URL | P1 | Validation + discovery path | **MVP / north star** |
| US2 — Optional auth with 401 guidance | P1 | `detectCapabilities` soft-start | Core usability |
| US3 — Scraping on by default | P2 | Config + scrape gate | Enables US1 on restricted instances |
| US4 — Reduced mandatory config in docs | P2 | README + docs | Operator-facing completion |

**Build order note**: Spec priority lists US1/US2 before US3/US4, but **US3 config must land
before US1 is fully verifiable on scrape-fallback instances**. Recommended build order:
Foundational → **US3** → **US1** → **US2** → **US4** → Polish. Each story remains
independently testable once its tasks complete.

---

## Phase 1: Setup

**Purpose**: Confirm scope before code changes

- [x] T001 Review [plan.md](./plan.md) and [contracts/configuration-contract.md](./contracts/configuration-contract.md) to confirm scope, breaking-change notice, and canonical auth remediation log text before coding

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Sequencing and docs inventory — no production logic yet

**⚠️ CRITICAL**: Complete before user story implementation

- [x] T002 Document the intended edit sequence for `cmd/opengrok-go-mcp/main.go` (config defaults → `resolveProjectAllowlist` gate/logs → `validateDefaultProjectAfterDiscovery` → `detectCapabilities` → `logStartupDiagnostics`) so story tasks do not conflict on the same functions
- [x] T003 [P] Inventory user-facing docs to update per plan: `README.md`, `docs/configuration.md`, `docs/limitations.md`, `CHANGELOG.md`, and `docs/README.md` reconciliation map

**Checkpoint**: Foundation ready — user story work can begin (US3 first for scrape-default)

---

## Phase 3: User Story 3 — Scraping on by default with opt-out (Priority: P2) — build first

**Goal**: Web-UI project discovery runs automatically when the REST project list fails; operators
opt out via `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE`. Legacy `OPENGROK_MCP_PROJECT_SCRAPE` remains
as a compat shim. (FR-005, FR-006)

**Independent Test**: Against a fixture where `/projects/indexed` fails but the web UI exposes
project options, scraping runs with no enable flag. With disable flag set, no web-UI fetch occurs.

### Tests for User Story 3 (write first)

- [x] T004 [P] [US3] Add `TestDefaultProjectScrapeEnabledIsTrue` proving `config.Default()` sets `ProjectScrapeEnabled == true` in `internal/config/config_test.go` (plan T1; fails against old default `false`)
- [x] T005 [P] [US3] Add table tests for `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` and legacy `OPENGROK_MCP_PROJECT_SCRAPE` precedence in `internal/config/config_test.go` (plan T2–T3)
- [x] T006 [P] [US3] Add `resolveProjectAllowlist` tests: API fail + default scrape → `scraped`; API success → no scrape; disable + API fail → `none` without scrape call in `cmd/opengrok-go-mcp/main_test.go` (plan T5–T7)

### Implementation for User Story 3

- [x] T007 [US3] Implement `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` parsing, legacy `OPENGROK_MCP_PROJECT_SCRAPE` shim, and `ProjectScrapeEnabled` default `true` in `internal/config/config.go` per [research.md](./research.md) R1
- [x] T008 [US3] Invert scrape gate in `resolveProjectAllowlist` and update startup log strings (remove experimental label on default path; log disable reason) in `cmd/opengrok-go-mcp/main.go`
- [x] T009 [US3] Run `go test ./internal/config/ ./cmd/opengrok-go-mcp/`; `gofmt -w` changed Go files

**Checkpoint**: Scrape default-on behavior independently testable

---

## Phase 4: User Story 1 — Start with only the base URL (Priority: P1) 🎯 MVP

**Goal**: Server starts with only `OPENGROK_MCP_BASE_URL`; never requires
`OPENGROK_MCP_DEFAULT_PROJECT` at startup; auto-default when exactly one project is known.
(FR-001, FR-002, FR-003)

**Independent Test**: Configure only base URL against a test double; server starts,
`list_projects` returns discovered projects, single-project auto-default works, multi-project
startup succeeds without default env var.

### Tests for User Story 1 (write first)

- [x] T010 [P] [US1] Add `validateDefaultProjectAfterDiscovery` tests: 0 projects + no default → `nil`; N>1 + no default → `nil` in `cmd/opengrok-go-mcp/main_test.go` (plan T8–T9)
- [x] T011 [P] [US1] Add `TestValidateAllowsMultiProjectEnvWithoutDefault` in `internal/config/config_test.go` (plan T4)

### Implementation for User Story 1

- [x] T012 [US1] Relax `validateDefaultProjectAfterDiscovery` for cases 0 and N>1 without default in `cmd/opengrok-go-mcp/main.go` per [data-model.md](./data-model.md)
- [x] T013 [US1] Remove `OPENGROK_MCP_DEFAULT_PROJECT` required when `len(Projects) > 1` from `Validate()` in `internal/config/config.go`
- [x] T014 [US1] Update or remove tests in `cmd/opengrok-go-mcp/main_test.go` that expect startup failure when default project is missing after multi-project or zero-project discovery (plan T14)
- [x] T015 [US1] Run `go test ./internal/config/ ./cmd/opengrok-go-mcp/`; `gofmt -w` changed Go files

**Checkpoint**: Base-URL-only startup path works for API and scrape discovery; MVP validated

---

## Phase 5: User Story 2 — Optional auth with actionable 401 guidance (Priority: P1)

**Goal**: Startup completes when all search probes return unauthorized and no auth token is
configured; emit canonical auth remediation log; capability-gate search tools honestly. TLS/transport
failures still abort startup. (FR-007, FR-008, FR-009, FR-010)

**Independent Test**: Start with only base URL against backend returning 401 on all search probes;
server starts, logs auth env var names, search capabilities false; TLS misconfig still errors.

### Tests for User Story 2 (write first)

- [x] T016 [P] [US2] Add or update `TestDetectCapabilitiesAuthOnlyUnauthorizedReturnsNilError` proving all-search-401 + no token returns `(caps, nil)` with search caps false in `cmd/opengrok-go-mcp/main_test.go` (plan T10–T11; replace `TestDetectCapabilitiesFailsWhenProjectsAndSearchFail` expectation where applicable)
- [x] T017 [P] [US2] Add `TestDetectCapabilitiesTLSFailureStillErrors` in `cmd/opengrok-go-mcp/main_test.go` (plan T12)
- [x] T018 [P] [US2] Add or extend `TestDetectCapabilitiesAnonymousSuccessNoAuthWarning` in `cmd/opengrok-go-mcp/main_test.go` (plan T13)

### Implementation for User Story 2

- [x] T019 [US2] Implement `authRemediationNeeded` helper and soft-start branch in `detectCapabilities` in `cmd/opengrok-go-mcp/main.go` per plan §4
- [x] T020 [US2] Emit canonical auth remediation log line from [contracts/configuration-contract.md](./contracts/configuration-contract.md) when remediation is needed in `cmd/opengrok-go-mcp/main.go`
- [x] T021 [US2] Update `logStartupDiagnostics` to include `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` and scrape state summary in `cmd/opengrok-go-mcp/main.go`
- [x] T022 [US2] Run `go test ./cmd/opengrok-go-mcp/`; `gofmt -w` changed Go files

**Checkpoint**: Auth-optional startup with actionable guidance independently testable

---

## Phase 6: User Story 4 — Reduced mandatory configuration in docs (Priority: P2)

**Goal**: README and docs show one required env var; minimal examples; migration note for scrape
default inversion. (FR-012, FR-013)

**Independent Test**: `docs/configuration.md` Required table lists only `OPENGROK_MCP_BASE_URL`;
README quick-start uses a one-line environment block.

### Implementation for User Story 4

- [x] T023 [P] [US4] Update `docs/configuration.md`: one required variable, `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE`, demoted default project, legacy scrape shim note
- [x] T024 [P] [US4] Update `README.md`: one-line base-URL-only client examples, optional auth subsection, migration blurb from [quickstart.md](./quickstart.md)
- [x] T025 [P] [US4] Update `docs/limitations.md`: default scrape fallback, zero-project startup, scraped list best-effort labeling
- [x] T026 [US4] Add breaking-change and migration entry to `CHANGELOG.md` per [contracts/configuration-contract.md](./contracts/configuration-contract.md)

**Checkpoint**: Operator docs match new posture

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Verification, agent UX, full-suite green

- [x] T027 [P] Documentation reconciliation gate: walk every row of `docs/README.md` and update the single home of each concern this feature affects, or mark N/A explicitly
- [ ] T028 Dispatch a fresh lightweight/mid-tier subagent with only `OPENGROK_MCP_BASE_URL` (+ auth if staging requires it) and task "List projects and search for a symbol"; capture first-use findings per plan Agent UX Validation
- [x] T029 Run [quickstart.md](./quickstart.md) verification checklist against local or staging OpenGrok instance (base-URL-only north star)
- [x] T030 [P] `gofmt -w` all changed Go files under `cmd/opengrok-go-mcp/` and `internal/config/`
- [x] T031 Run `go test ./...` and confirm full suite passes

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 1 (Setup)
    ↓
Phase 2 (Foundational)
    ↓
Phase 3 (US3 — scrape default)  ← build before US1 north-star on restricted instances
    ↓
Phase 4 (US1 — base URL only)   ← MVP checkpoint
    ↓
Phase 5 (US2 — auth guidance)
    ↓
Phase 6 (US4 — docs)            ← can start after US1+US2 behavior is stable; [P] doc tasks parallel
    ↓
Phase 7 (Polish)
```

### User Story Dependencies

- **US3**: Depends on Foundational only; **blocks full US1 verification** on scrape-fallback instances
- **US1**: Depends on US3 for scrape-default path; API-only path testable after Foundational + partial config
- **US2**: Depends on US1 validation landing (shared `main.go`); independently testable via `detectCapabilities` tests
- **US4**: Depends on finalized env var names and behavior from US1–US3

### Parallel Opportunities

- **Phase 2**: T003 ∥ T002 (after T001)
- **Phase 3 tests**: T004 ∥ T005 ∥ T006 (before T007–T008)
- **Phase 4 tests**: T010 ∥ T011 (before T012–T013)
- **Phase 5 tests**: T016 ∥ T017 ∥ T018 (before T019–T021)
- **Phase 6**: T023 ∥ T024 ∥ T025 (before T026 CHANGELOG ties migration together)
- **Phase 7**: T027 ∥ T030 while T028–T029 run manually

---

## Parallel Example: User Story 3

```bash
# Write all US3 tests together (must fail first):
go test ./internal/config/ -run 'TestDefaultProjectScrape|TestFromEnv.*Scrape|TestDisable'
go test ./cmd/opengrok-go-mcp/ -run 'ResolveProjectAllowlist.*Scrape'

# Then implement:
# T007 internal/config/config.go
# T008 cmd/opengrok-go-mcp/main.go
```

---

## Parallel Example: User Story 4

```bash
# Doc updates in parallel (different files):
# T023 docs/configuration.md
# T024 README.md
# T025 docs/limitations.md
# Then T026 CHANGELOG.md migration entry
```

---

## Implementation Strategy

### MVP First (US3 + US1)

1. Complete Phase 1–2
2. Complete Phase 3 (US3) — scrape default-on
3. Complete Phase 4 (US1) — **STOP and VALIDATE** base-URL-only startup with `go test` + manual smoke
4. Add Phase 5 (US2) for auth-soft-start
5. Add Phase 6 (US4) docs before merge

### Suggested single-developer sequence

T001 → T002 → T003 → T004–T006 (tests) → T007–T009 → T010–T011 (tests) → T012–T015 →
T016–T018 (tests) → T019–T022 → T023–T026 → T027–T031

### Incremental delivery checkpoints

| Checkpoint | After tasks | Validates |
|------------|-------------|-----------|
| Scrape default | T009 | US3 independent test |
| MVP | T015 | US1 base-URL-only startup |
| Auth posture | T022 | US2 401 guidance |
| Ship-ready docs | T026 | US4 documentation |
| Merge-ready | T031 | Full regression |

---

## Notes

- No new Go packages or MCP tool schema changes; all code in `internal/config/` and `cmd/opengrok-go-mcp/`
- `internal/opengrok/scrape.go` unchanged — only gate/default/config wiring
- Commit after each task group or logical checkpoint
- Do not commit secrets; dogfood staging instance uses operator-local env only
