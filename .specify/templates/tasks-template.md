---

description: "Task list template for feature implementation"
---

# Tasks: [FEATURE NAME]

**Input**: Design documents from `/specs/[###-feature-name]/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Behavioral code changes require focused tests that fail against old
behavior or otherwise prove the new behavior. For non-trivial behavior changes,
prefer test-first task ordering unless the implementation plan documents why
another sequence is clearer.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Command entrypoint**: `cmd/opengrok-go-mcp/`
- **Core MCP behavior**: `internal/mcpserver/`
- **OpenGrok client behavior**: `internal/opengrok/`
- **Configuration**: `internal/config/`
- **Supporting packages**: `internal/cache/`, `internal/cursor/`, `internal/links/`
- **Docs**: `README.md`, `docs/`
- Paths shown below assume this repository layout - adjust based on plan.md structure

<!--
  ============================================================================
  IMPORTANT: The tasks below are SAMPLE TASKS for illustration purposes only.

  The /speckit-tasks command MUST replace these with actual tasks based on:
  - User stories from spec.md (with their priorities P1, P2, P3...)
  - Feature requirements from plan.md
  - Entities from data-model.md
  - Endpoints from contracts/

  Tasks MUST be organized by user story so each story can be:
  - Implemented independently
  - Tested independently
  - Delivered as an MVP increment

  DO NOT keep these sample tasks in the generated tasks.md file.
  ============================================================================
-->

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create project structure per implementation plan
- [ ] T002 Initialize [language] project with [framework] dependencies
- [ ] T003 [P] Configure linting and formatting tools

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

Examples of foundational tasks (adjust based on this Go MCP project):

- [ ] T004 Identify affected MCP tools/resources and capability gates in internal/mcpserver/
- [ ] T005 [P] Identify affected OpenGrok client behavior in internal/opengrok/
- [ ] T006 [P] Identify affected configuration or environment variables in internal/config/
- [ ] T007 Define warning, citation, pagination, cursor, experimental-label, and resource-bound contract changes
- [ ] T008 Confirm README/docs updates required by user-facing behavior
- [ ] T009 Confirm secure handling for tokens, HTTP transport, TLS, and raw fallbacks
- [ ] T010 Define explicit limits, defaults, and warnings for response-size, tool-call, or automatic-fetch changes

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - [Title] (Priority: P1) 🎯 MVP

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 1

> **NOTE: For non-trivial behavior changes, write these tests first and confirm
> they fail against old behavior. For simpler changes, ensure the listed tests
> otherwise prove the new behavior.**

- [ ] T011 [P] [US1] Add proving contract/unit test for [MCP behavior] in internal/mcpserver/[name]_test.go
- [ ] T012 [P] [US1] Add proving client/config test for [edge case] in internal/[package]/[name]_test.go

### Implementation for User Story 1

- [ ] T013 [P] [US1] Update input/output types in internal/mcpserver/types.go
- [ ] T014 [P] [US1] Update OpenGrok client or config behavior in internal/[package]/[file].go
- [ ] T015 [US1] Implement MCP service behavior in internal/mcpserver/server.go
- [ ] T016 [US1] Add warnings, citations, pagination, capability-gating, experimental-label, or resource-bound behavior
- [ ] T017 [US1] Update README.md or docs/ for user-visible behavior
- [ ] T018 [US1] Run targeted Go test for this story

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - [Title] (Priority: P2)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 2

- [ ] T019 [P] [US2] Add proving contract/unit test for [MCP behavior] in internal/mcpserver/[name]_test.go
- [ ] T020 [P] [US2] Add proving edge-case test in internal/[package]/[name]_test.go

### Implementation for User Story 2

- [ ] T021 [P] [US2] Update relevant Go types/helpers in internal/[package]/[file].go
- [ ] T022 [US2] Implement MCP service/client behavior in internal/[package]/[file].go
- [ ] T023 [US2] Update documentation or limitations notes
- [ ] T024 [US2] Run targeted Go test for this story

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - [Title] (Priority: P3)

**Goal**: [Brief description of what this story delivers]

**Independent Test**: [How to verify this story works on its own]

### Tests for User Story 3

- [ ] T025 [P] [US3] Add proving contract/unit test for [MCP behavior] in internal/mcpserver/[name]_test.go
- [ ] T026 [P] [US3] Add proving edge-case test in internal/[package]/[name]_test.go

### Implementation for User Story 3

- [ ] T027 [P] [US3] Update relevant Go types/helpers in internal/[package]/[file].go
- [ ] T028 [US3] Implement MCP service/client behavior in internal/[package]/[file].go
- [ ] T029 [US3] Run targeted Go test for this story

**Checkpoint**: All user stories should now be independently functional

---

[Add more user story phases as needed, following the same pattern]

---

## Phase N: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] TXXX [P] Documentation reconciliation gate: before completion/push, walk every row of `docs/README.md` (the documentation source-of-truth map) and update the single home of each concern this change affects, or mark it explicitly N/A. Do not restate canon across docs.
- [ ] TXXX Dispatch a fresh lightweight/mid-tier subagent with a realistic task and minimal context; capture its first-use findings on tool descriptions, schemas, warnings, defaults, and examples
- [ ] TXXX Code cleanup and refactoring
- [ ] TXXX Performance optimization across all stories
- [ ] TXXX [P] Additional Go tests for uncovered edge cases
- [ ] TXXX Verify experimental labels and resource-bound warnings across tools, docs, and config
- [ ] TXXX Security hardening
- [ ] TXXX Run quickstart.md validation
- [ ] TXXX Run `gofmt` on changed Go files
- [ ] TXXX Run `go test ./...`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - May integrate with US1 but should be independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - May integrate with US1/US2 but should be independently testable

### Within Each User Story

- For non-trivial behavior changes, proving tests SHOULD come before implementation
- Types/helpers before service wiring
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All tests for a user story marked [P] can run in parallel
- Models within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Add proving contract/unit test for [MCP behavior] in internal/mcpserver/[name]_test.go"
Task: "Add proving client/config test for [edge case] in internal/[package]/[name]_test.go"

# Launch independent implementation support tasks together:
Task: "Update input/output types in internal/mcpserver/types.go"
Task: "Update OpenGrok client or config behavior in internal/[package]/[file].go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2
   - Developer C: User Story 3
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- For non-trivial behavior changes, verify proving tests fail before implementing;
  otherwise document how the tests prove the new behavior
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
