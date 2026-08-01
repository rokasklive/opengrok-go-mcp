---
description: "Task list for Honest Hit Accounting and Schema Fidelity (009)"
---

# Tasks: Honest Hit Accounting and Schema Fidelity

**Input**: Design documents from `/specs/009-hit-accounting-and-schema-fidelity/`

**Prerequisites**: spec.md (decisions D1–D3), plan.md

> **D1 superseded during implementation (2026-08-01).** OpenGrok reports no
> global line count — `resultCount` is constant regardless of `maxresults`
> (verified: 52 documents reported whether 1 or all 52 were fetched, yielding 51
> vs 910 lines). A line-unit `total_hits` could therefore only ever be
> page-local, making SC-002 unsatisfiable and the `total_` prefix wrong. The
> maintainer settled on: `total_hits` keeps document units (no breaking change)
> and a new `results_on_page` carries `len(results)`. Task text below reflects
> the original D1; the delivered shape is the superseding one.
>
> **D2 assumption corrected during T023.** The plan assumed `page_size` bounds
> files. It does not: the server caps returned *lines* at `page_size` while the
> cursor advances by whole files, so line matches beyond `page_size` inside a
> fetched file are dropped and unreachable. Verified live — `page_size:2` over a
> query with 85 line-hits in the first 2 files surfaced 2 and skipped to file 3.
> This is a pre-existing defect outside 009's scope; it is now documented in
> `docs/limitations.md` and named in the `PAGE_SIZE_TRUNCATED` warning, and
> needs its own spec to fix.

**Tests**: REQUIRED. `total_hits` is a breaking change to a public output field;
constitution Principle III requires the guard to exist and fail before the field
moves.

**Phase order note**: Phases follow the **build order in plan.md** — the breaking
unit change (US1) is plumbed, repointed, and only then flipped, so the field's
meaning changes exactly once under a guard that was red beforehand. US2 (P2) is
sequenced last despite being lower risk, so it cannot delay the security-relevant
correctness fix.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependencies)
- **[Story]**: US1–US2 from spec.md

---

## Phase 1: Foundational (Blocking Prerequisite)

**Purpose**: Prove the defect before changing anything. This test is the contract
for the whole feature.

**⚠️ CRITICAL**: Blocks all of US1.

- [X] T001 [US1] Fixture + failing test: a search result whose `results` map has one path with two hits and two paths with one hit each (`resultCount: 3`, 4 line hits); assert `total_hits == len(results)` on an unpaged response. MUST FAIL against current `main` (asserts 3 == 4) — record the failure output in the PR. In `internal/mcpserver/search_test.go`

**Checkpoint**: Red test pins the defect; any later refactor that "fixes" it silently is caught.

---

## Phase 2: User Story 1 - Trustworthy hit count (Priority: P1)

**Goal**: `total_hits` counts lines and agrees with `len(results)`; `total_files`
carries the document count and drives pagination; `page_size` is documented as
bounding documents.

**Independent Test**: T001 goes green, and a full pagination walk accumulates a
result count matching the advertised total with `has_more` false only on the last
page.

### Step 2a — Plumb the document count (additive, no observable change)

- [X] T002 [US1] Add `TotalFiles int` to `opengrok.SearchResult` and set it from `searchResponse.ResultCount` alongside the existing assignment, in `internal/opengrok/client.go`
- [X] T003 [US1] Add `TotalFiles int \`json:"total_files"\`` to the search `Pagination` struct and any response struct that carries `total_hits`, in `internal/mcpserver/types.go`
- [X] T004 [US1] Propagate `TotalFiles` through the search, symbol, and compound response builders, in `internal/mcpserver/search_core.go`, `internal/mcpserver/symbols.go`, `internal/mcpserver/compound.go`, `internal/mcpserver/search_handlers.go`
- [X] T005 [P] [US1] Test: `total_files` equals OpenGrok's `resultCount` and is present on search, symbol, and compound responses, in `internal/mcpserver/search_test.go`

**Checkpoint**: `total_files` exposed; every pre-existing test still green; `total_hits` unchanged.

### Step 2b — Repoint pagination at the document count (still no observable change)

- [X] T006 [US1] Change `newPagination` to take the document count explicitly for `total_pages`/`has_more` rather than reusing the hit count, in `internal/mcpserver/pagination.go` and its call sites
- [X] T007 [US1] Pass the document count to `nextCursor` so `offset` advancement stays document-based per D2; cursor payload format unchanged, in `internal/mcpserver/search_core.go` and `internal/mcpserver/helpers.go`
- [X] T008 [P] [US1] Test: `total_pages`, `has_more`, and cursor `offset` are computed from the document count, asserted on a fixture where document and line counts differ, in `internal/mcpserver/search_test.go`
- [X] T009 [P] [US1] Test: an existing cursor round-trips unchanged (no cursor format or offset-semantics regression), in `internal/cursor/cursor_test.go`

**Checkpoint**: Pagination provably reads the document count; `total_hits` still documents. Safe to flip.

### Step 2c — Flip `total_hits` to lines (the breaking step)

- [X] T010 [US1] Set `total_hits` from the returned line-hit count instead of `result.TotalHits`, in `internal/mcpserver/search_core.go` and `internal/mcpserver/symbols.go`
- [X] T011 [US1] Verify `PAGE_SIZE_TRUNCATED` (already line-based) now reports the same unit as `total_hits`; update its message only if it reads ambiguously, in `internal/mcpserver/search_core.go`
- [X] T012 [US1] Audit every remaining `result.TotalHits` reader for unit correctness — `searchWarnThreshold` (HIGH_HIT_COUNT), `listSymbolsWarnThreshold` (LARGE_SYMBOL_LIST), and `total_hits_scope` on kind-filtered listings must each be pinned to the unit they intend, in `internal/mcpserver/search_core.go` and `internal/mcpserver/symbols.go`
- [X] T013 [US1] Pagination-walk test: page a multi-line-per-file fixture to exhaustion; accumulated results equal the advertised total and `has_more` is false only on the last page (SC-002), in `internal/mcpserver/search_test.go`
- [X] T014 [US1] Migration notes — `CHANGELOG.md` **Changed (breaking)** entry stating the old unit, the new unit, and `total_files` as the replacement for anyone doing page math; matching field-semantics update in `docs/tool-contracts.md` (SC-001, scenario 4)
- [X] T015 [US1] Update `docs/limitations.md` — replace the "`total_hits` counts matching files" known-issue entry added ahead of this feature with the resolved behavior

**Checkpoint**: T001 green. `total_hits` reconciles with `len(results)`; migration documented in the same change.

### Step 2d — Document what `page_size` bounds

- [X] T016 [P] [US1] Update `page_size` schema descriptions to state it bounds documents, not returned results, so a page may contain more results than `page_size` (scenario 5), in `internal/mcpserver/types.go`
- [X] T017 [P] [US1] Test: a page whose documents carry multiple line hits returns more than `page_size` results without a truncation warning, and the schema text says so, in `internal/mcpserver/search_test.go`

**Checkpoint**: US1 complete and independently shippable.

---

## Phase 3: User Story 2 - Schema never offers a rejected field (Priority: P2)

**Goal**: Every top-level compact property names the operations that accept it; no
advertised (operation, field) pair is rejected with `UNKNOWN_FIELD`.

**Independent Test**: The generated matrix in T019 passes for all four compact tools.

### Tests first

- [X] T018 [P] [US2] Generated test: walk the **published** `tools/list` schema for each compact tool, enumerate every (operation, top-level property) pair, and assert the pair is either accepted by validation or not advertised for that operation (SC-004). Generated from the schema, never hand-listed, so it cannot drift. In `internal/mcpserver/compact_schema_test.go`
- [X] T019 [US2] Confirm T018 fails against current `main` on the known pairs (`operation=code` + `max_results`, and any sibling the walk finds) — record the failing pair list in the PR

### Implementation

- [X] T020 [US2] Annotate each top-level property description with its owning operations (D3; e.g. `max_results` → "operation=read only"), leaving the `oneOf` branches untouched, in `internal/mcpserver/compact_schema.go`
- [X] T021 [US2] Verify the 0.5.1 strict-client behavior is unchanged — its existing regression test must stay green (SC-005), in `internal/mcpserver/compact_test.go`
- [X] T022 [P] [US2] Record the `tools/list` byte delta from the annotations against the agreed budget (SC-006), in `evals/` token benchmark output

**Checkpoint**: US2 complete; annotations cost measured.

---

## Phase 4: Polish

- [X] T023 Update `docs/agent-usage-patterns.md` if hit-count guidance appears there
- [X] T024 Fresh-subagent UX run per AGENTS.md: give a cold agent a paginate-to-exhaustion task and capture whether it reads the counts correctly without prior context
- [X] T025 Full verification — `gofmt -w`, `go test ./...`, `go test ./evals/ -count=1`, `git diff --check`, and a live run against a real OpenGrok host confirming T001's scenario end-to-end

---

## Dependencies

- **T001** blocks all of Phase 2.
- **Step 2a → 2b → 2c is strictly ordered.** T010 must not land before T006–T009: flipping the unit while pagination still infers the document count from `total_hits` silently corrupts `total_pages` and `has_more`.
- **T014 lands with T010**, not after. A public field must not change meaning in a commit that lacks its migration note.
- **Phase 3 is independent of Phase 2** and may proceed in parallel by a second worker; it touches only `compact_schema.go` and its tests.
- **Phase 4** requires Phases 2 and 3.

## Parallel Opportunities

- T005, T008, T009 are different assertions in different files once their step's implementation lands.
- T016/T017 are independent of T018–T022.
- Phase 3 in full can run alongside Phase 2 (disjoint files).
