---
description: "Task list for Agent Ergonomics Hardening"
---

# Tasks: Agent Ergonomics Hardening

**Input**: Design documents from `specs/007-agent-ergonomics-hardening/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Test-first (Constitution III). Proving tests are written before the
implementation they cover and confirmed to fail against old behavior.

**Phase order note**: Phases follow the plan's **wave build sequence**
(research D10 / implementation-playbooks), not raw story priority. Wave 1 ships
additive L2 clarity (US6, US5, US3) before the economy profile (US1) and manifest
(US2), so rich-default deployments gain metadata without behavior change first.
Trajectory gates (US4) run last because they grade the finished surface.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete dependencies)
- **[Story]**: US1–US6 (setup/foundational/polish tasks carry no story label)

## Path Conventions

- Config: `internal/config/`
- MCP server: `internal/mcpserver/`
- Entrypoint: `cmd/opengrok-go-mcp/`
- Eval harness: `evals/`
- Docs: `README.md`, `docs/`, `CHANGELOG.md`

---

## Phase 1: Setup

**Purpose**: Green baseline and contract alignment before code changes.

- [x] T001 Establish a green baseline: `go test ./... -count=1`
- [x] T002 [P] Walk `specs/007-agent-ergonomics-hardening/contracts/` and `data-model.md`; note FR→file mapping in implementation notes (no code)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared config types used by US1 (profile) and US2 (manifest).

**⚠️ CRITICAL**: Complete before user-story phases.

- [x] T003 Add `CapabilityReport`, `ToolCapability`, `GatedCapability`, and `ProjectCatalogMeta` types in `internal/config/capability_report.go` per `data-model.md`
- [x] T004 Add `AgentProfile` field and parse/validate `OPENGROK_MCP_AGENT_PROFILE` (`economy`|`rich`, unset → economy) in `internal/config/config.go` with fail-fast on invalid values
- [x] T005 [P] Proving test for valid/invalid agent profile env in `internal/config/config_test.go`
- [x] T006 [P] Add empty `CapabilityReport` placeholder on `config.Config` ready for startup population in `internal/config/config.go`

**Checkpoint**: Config types and profile parsing ready; no MCP behavior change yet.

---

## Phase 3: User Story 6 - Tool headers teach economy (Priority: P3) — Wave 1

**Goal**: Cold agents discover economy knobs and surface/detail naming from tool
descriptions alone (FR-011).

**Independent Test**: Read compact tool descriptions; each search/symbols/read tool
includes one economy-hint sentence and distinguishes compact **tool surface** from
`response_mode=compact` **payload detail**.

### Tests for User Story 6 (write first)

- [x] T007 [P] [US6] Proving test in `internal/mcpserver/compact_test.go` that `compactSearchDescription`, `compactSymbolsDescription`, and `compactReadDescription` output contains economy-hint and disambiguation prose per `contracts/agent-profile-contract.md`

### Implementation for User Story 6

- [x] T008 [US6] Update economy hints and surface-vs-detail disambiguation in `internal/mcpserver/compact_descriptions.go`
- [x] T009 [P] [US6] Apply equivalent economy/disambiguation copy to full-surface search/symbol/read tool descriptions in `internal/mcpserver/register_full.go`
- [x] T010 [US6] Run `go test ./internal/mcpserver/ -run 'Compact|Description' -count=1`

**Checkpoint**: Tool headers teach economy without reading external docs.

---

## Phase 4: User Story 5 - Project catalog freshness (Priority: P3) — Wave 1

**Goal**: Agents see catalog source and snapshot semantics; `UNKNOWN_PROJECT`
advises restart (FR-009, FR-010).

**Independent Test**: `list_projects` returns `catalog_source` and
`catalog_is_snapshot`; unknown project after startup lists snapshot-restart hint.

### Tests for User Story 5 (write first)

- [x] T011 [P] [US5] Proving tests for `catalog_source` and `catalog_is_snapshot` on `ListProjectsOutput` in `internal/mcpserver/projects_test.go`
- [x] T012 [P] [US5] Proving test that `UNKNOWN_PROJECT` message mentions startup snapshot and restart in `internal/mcpserver/projects_test.go` or `internal/mcpserver/search_test.go`

### Implementation for User Story 5

- [x] T013 [P] [US5] Add `catalog_source` and `catalog_is_snapshot` to `ListProjectsOutput` in `internal/mcpserver/types.go`
- [x] T014 [US5] Populate catalog metadata from `cfg.ProjectSource` in `internal/mcpserver/projects.go` (always `catalog_is_snapshot: true`)
- [x] T015 [US5] Extend `validateConfiguredProjects` `UNKNOWN_PROJECT` message in `internal/mcpserver/helpers.go` with snapshot staleness and restart guidance
- [x] T016 [P] [US5] Document catalog fields in `docs/tool-contracts.md` and `docs/limitations.md` per `contracts/kind-filter-catalog-contract.md`
- [x] T017 [US5] Run `go test ./internal/mcpserver/ -run 'Project|Unknown' -count=1`

**Checkpoint**: Project list and unknown-project errors expose catalog freshness.

---

## Phase 5: User Story 3 - Kind-filter semantics (Priority: P2) — Wave 1

**Goal**: Page-local kind filtering cannot be misread as global `total_hits`
(FR-008).

**Independent Test**: `list_symbols` with `kind` set returns `kind_filter_active`,
`kind_matches_on_page`, and `total_hits_scope=pre_kind_filter`; fields absent when
`kind` omitted.

### Tests for User Story 3 (write first)

- [x] T018 [P] [US3] Proving tests for kind-filter metadata presence/absence and `kind_matches_on_page == len(symbols)` in `internal/mcpserver/symbols_test.go`

### Implementation for User Story 3

- [x] T019 [US3] Add `kind_filter_active`, `kind_matches_on_page`, `total_hits_scope` to `ListSymbolsOutput` in `internal/mcpserver/types.go`
- [x] T020 [US3] Emit kind-filter metadata in `internal/mcpserver/symbols.go` when `input.Kind != ""`; retain `KIND_FILTER_PAGE_LOCAL` warning
- [x] T021 [P] [US3] Document kind-filter output fields in `docs/tool-contracts.md`
- [x] T022 [US3] Run `go test ./internal/mcpserver/ -run ListSymbols -count=1`

**Checkpoint**: Kind-filter responses are structurally unambiguous.

---

## Phase 6: User Story 1 - Economy profile (Priority: P1) 🎯 MVP — Wave 2

**Goal**: Opt-in `OPENGROK_MCP_AGENT_PROFILE=economy` bundles token-frugal defaults;
unset profile = economy (shipped default); per-call overrides win (FR-001–004, FR-015).

**Independent Test**: With `economy`, bare search calls omit auto-expansion and use
`response_mode=compact` by default while keeping `citation.url`; token benchmark warm
totals drop ≥15% vs rich on four scenarios (SC-001).

### Tests for User Story 1 (write first)

- [x] T023 [P] [US1] Proving tests for rich vs economy default resolution (`expand_context`, `response_mode`, `include_links`) in `internal/mcpserver/helpers_test.go`
- [x] T024 [P] [US1] Proving tests that per-call knobs override profile in `internal/mcpserver/search_test.go`
- [x] T025 [P] [US1] Proving test that `SearchAndRead` / `FindSymbolAndReferences` retain `warnings[]` under economy profile in `internal/mcpserver/search_test.go` or `internal/mcpserver/compound.go` tests (FR-015)
- [x] T026 [P] [US1] Proving test for `expand_context` jsonschema description reflecting active profile in `internal/mcpserver/compact_schema_test.go`

### Implementation for User Story 1

- [x] T027 [US1] Implement `resolveResponseModeDefault`, profile-aware `shouldExpandContext`, and profile-aware `includeLinks` in `internal/mcpserver/helpers.go` per `contracts/agent-profile-contract.md`
- [x] T028 [US1] Wire profile bundles (`economy`/`rich`) to resolved defaults in `internal/config/config.go` and expose on `config.Config`
- [x] T029 [P] [US1] Add `patchExpandContextDescription` and apply at registration in `internal/mcpserver/register_compact.go` and `internal/mcpserver/register_full.go`
- [x] T030 [P] [US1] Document `OPENGROK_MCP_AGENT_PROFILE` in `docs/configuration.md` and `README.md`
- [x] T031 [US1] Extend `evals/token_benchmark_test.go` to run compact scenarios under `economy` and `rich`; record deltas for SC-001
- [x] T032 [US1] Run `go test ./internal/config/ ./internal/mcpserver/ -count=1`

**Checkpoint**: Economy profile delivers measurable token savings on opt-in.

---

## Phase 7: User Story 2 - Capability manifest (Priority: P1) — Wave 3

**Goal**: `opengrok://capabilities` lists live tools, gated families, and remediation
(FR-005–007).

**Independent Test**: Read manifest on gated fixture — `tools[]` matches `ListTools`;
`gated[]` explains missing references; preamble in `agent-usage-patterns.md` points
agents here first.

**Depends on**: T006 `CapabilityReport` on config (foundational).

### Tests for User Story 2 (write first)

- [x] T033 [P] [US2] Proving tests for `BuildCapabilityManifest` tool list parity and no secrets in remediation in `internal/mcpserver/manifest_test.go`
- [x] T034 [P] [US2] Proving test that `opengrok://capabilities` is registered even when search tools are gated in `internal/mcpserver/resources_test.go`

### Implementation for User Story 2

- [x] T035 [US2] Populate `cfg.CapabilityReport` from startup probes (enabled + gated + reason codes) in `cmd/opengrok-go-mcp/main.go`
- [x] T036 [US2] Implement `BuildCapabilityManifest` in `internal/mcpserver/manifest.go` per `contracts/capability-manifest-contract.md`
- [x] T037 [US2] Register `opengrok://capabilities` resource handler in `internal/mcpserver/resources.go` (always on)
- [x] T038 [P] [US2] Add capability-discovery preamble to `docs/agent-usage-patterns.md` referencing `opengrok://capabilities`
- [x] T039 [US2] Add hermetic eval case reading `opengrok://capabilities` (gated + full fixtures) in `evals/testdata/manifest.json` and wire in `evals/evals_test.go`
- [x] T040 [US2] Run `go test ./internal/mcpserver/ -run 'Manifest|Resources' -count=1` and `go test ./evals/ -count=1`

**Checkpoint**: Runtime capability ground truth closes doc/schema drift.

---

## Phase 8: User Story 4 - Trajectory eval & ListTools gate (Priority: P2) — Wave 4

**Goal**: Deterministic trajectory graders and compact ListTools CI ceiling
(FR-012–014, SC-004, SC-005).

**Independent Test**: `TestTrajectorySuite` runs ≥3 scenarios with ≥8 graders; seeded
description regression fails; `TestTokenBenchmark` fails when compact ListTools bytes
exceed committed ceiling.

### Tests for User Story 4 (write first)

- [x] T041 [P] [US4] Implement grader primitives (`tool_sequence`, `warning_code`, `citation_present`, `field_present`, `field_eq`) in `evals/graders.go`
- [x] T042 [P] [US4] Add trajectory fixtures in `evals/testdata/trajectory/` (`symbol-investigation-compact.json`, `search-narrow-warnings.json`, `kind-filter-metadata.json`, `description-cuj-symbol.json`)
- [x] T043 [US4] Implement `TestTrajectorySuite` in `evals/trajectory_test.go` with ≥8 graders (confirm red before green)
- [x] T044 [P] [US4] Implement `evals/description_cuj.go` resolver and regression test per `contracts/trajectory-eval-contract.md`

### Implementation for User Story 4

- [x] T045 [US4] Add `compact_list_tools_ceiling_bytes` to `evals/baselines/token_report.json` and enforce ceiling in `evals/token_benchmark_test.go`
- [x] T046 [US4] Run `go test ./evals/ -count=1`; refresh baselines via `./scripts/update-eval-results.sh` only when ceiling change is intentional

**Checkpoint**: Agent-workflow regressions and schema bloat are CI-gated.

---

## Phase 9: Polish & Cross-Cutting Concerns — Wave 5

**Purpose**: Documentation, gates, and constitution validation.

- [x] T047 [P] Documentation reconciliation: walk `docs/README.md` map; update `docs/configuration.md`, `docs/tool-contracts.md`, `docs/limitations.md`, `docs/agent-usage-patterns.md`, `CHANGELOG.md` for all shipped changes
- [x] T048 Dispatch fresh-subagent probe per `quickstart.md` G7 (manifest → symbol → read); capture first-use findings
- [x] T049 Write `specs/007-agent-ergonomics-hardening/gate-results.md` with SC-001 token deltas and quickstart gate PASS/CONDITIONAL/FAIL
- [x] T050 [P] Optional real-instance smoke (`OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1`, `OPENGROK_MCP_AGENT_PROFILE=economy`); notes in `gate-results.md`
- [x] T051 Run `gofmt -w` on changed Go files
- [x] T052 Run `go test ./... -count=1` and `git diff --check`

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 1 Setup
  → Phase 2 Foundational (blocks US1 profile wiring + US2 manifest population)
  → Phase 3 US6 ─┐
  → Phase 4 US5  ├─ Wave 1 (parallelizable across stories after Foundational)
  → Phase 5 US3 ─┘
  → Phase 6 US1 (economy profile — MVP value)
  → Phase 7 US2 (manifest — needs CapabilityReport + optional economy in manifest body)
  → Phase 8 US4 (trajectory — grades finished surface)
  → Phase 9 Polish
```

### User Story Dependencies

| Story | Depends on | Independent test |
|-------|------------|------------------|
| US6 | Foundational | Description string tests |
| US5 | Foundational | `projects_test` catalog fields |
| US3 | Foundational | `symbols_test` kind metadata |
| US1 | Foundational | Profile helper tests + token benchmark |
| US2 | Foundational, US1 optional (manifest includes `agent_profile`) | Resource read + manifest_test |
| US4 | US1–US3 shipped (graders reference their fields) | `TestTrajectorySuite` |

US6, US5, and US3 can proceed in parallel after Phase 2. US2 should follow US1 so
manifest reports the active profile. US4 is last.

### Parallel Opportunities

- **Phase 2**: T005 ∥ T006
- **Wave 1**: US6, US5, US3 phases in parallel (different files)
- **US1 tests**: T023–T026 in parallel
- **US2 tests**: T033 ∥ T034
- **US4**: T041 ∥ T042 ∥ T044
- **Polish**: T047 ∥ T050

---

## Parallel Example: Wave 1 (after Foundational)

```bash
# Three developers — one story each:
# Dev A: US6 T007–T010 (compact_descriptions.go)
# Dev B: US5 T011–T017 (projects.go, helpers.go)
# Dev C: US3 T018–T022 (symbols.go)
```

---

## Parallel Example: User Story 1

```bash
# Tests first (parallel):
go test ./internal/mcpserver/ -run 'Profile|Economy|ExpandContext' -count=1

# Implementation (partial parallel):
# T029 register_compact.go + register_full.go  ∥  T030 docs
```

---

## Implementation Strategy

### MVP First (User Story 1 after Wave 1 quick wins)

1. Complete Phase 1–2 (setup + foundational types)
2. **Optional fast path**: Skip to Phase 6 (US1) if token savings are urgent — Wave 1
   can land in parallel or immediately after
3. Complete Phase 6 (US1) → validate SC-001 with `OPENGROK_MCP_AGENT_PROFILE=economy`
4. **STOP and VALIDATE**: Token benchmark delta ≥15% on four scenarios

### Incremental Delivery

1. Wave 1 (US6 + US5 + US3) → additive clarity, zero default behavior change
2. US1 → economy profile (highest ROI)
3. US2 → manifest (closes cold-agent planning gap)
4. US4 → CI trajectory + ListTools gate
5. Polish → gate-results + subagent probe

### Suggested merge order

`US6 → US5 → US3 → US1 → US2 → US4 → Polish` (matches plan waves; each merge is
independently valuable).

---

## Notes

- Shipped default profile is **economy**; set `OPENGROK_MCP_AGENT_PROFILE=rich` for prior expanded-default behavior.
- `response_mode` JSON value stays `compact`; disambiguation is prose-only (research D2).
- Trajectory v1 uses deterministic graders only — no LLM-as-judge.
- Refresh `evals/baselines/` only alongside intentional schema growth with CHANGELOG note.
