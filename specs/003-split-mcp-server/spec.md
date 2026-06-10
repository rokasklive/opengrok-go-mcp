# Feature Specification: Split MCP Server Monolith

**Feature Branch**: `003-split-mcp-server`

**Created**: 2026-06-10

**Status**: Draft

**Input**: User description: "de-monolith the server.go, non functional mcp server
refactoring/splitting into smaller, easier to mantain and test pieces. Follow best
practices for go and mcp server devolopment"

---

## Background & Problem

The MCP server implementation is concentrated in a single large source file within the
`mcpserver` package. That file holds tool registration, request handling, response shaping,
pagination, warnings, gateway routing, memory tools, and shared helpers together. The
package already has smaller companion files, but the bulk of behavior still lives in one
place.

This concentration slows down maintenance: a change to one tool requires navigating a large
file, reviews are harder to scope, and tests are more likely to exercise broad paths instead
of isolated units. Contributors adding or adjusting tools face higher cognitive load and
greater risk of accidental cross-cutting edits.

This feature is a **non-functional refactor**: agents, operators, and end users should see
no change in MCP tool names, schemas, defaults, warnings, pagination, citations, capability
gating, or runtime behavior. The goal is a clearer internal structure that is easier to
maintain, extend, and test.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Preserve agent-facing MCP behavior (Priority: P1)

An agent or operator uses the MCP server exactly as before — same tools on each surface
(full, compact, gateway), same inputs and outputs, same warnings and error codes, same
pagination cursors, and same citation URLs. After the refactor, every existing workflow
(list projects, search, read files, symbol tools, memory tools, gateway discover/call)
produces indistinguishable results for the same inputs and OpenGrok backend responses.

**Why this priority**: A refactor that changes the public MCP contract would violate project
constitution and break downstream agents. Behavioral equivalence is the primary constraint.

**Independent Test**: Run the full existing test suite and any live or recorded integration
checks against a fixed backend fixture. Compare tool outputs, error codes, and warning text
before and after the refactor; they must match.

**Acceptance Scenarios**:

1. **Given** the server is built from the refactored code and configured identically,
   **When** an agent invokes each registered tool with representative inputs,
   **Then** responses match pre-refactor outputs for fields, warnings, cursors, and citations.
2. **Given** a tool is unavailable due to capability gating at startup,
   **When** the agent lists tools or attempts the gated operation,
   **Then** registration and error behavior match the pre-refactor server.
3. **Given** invalid inputs (bad cursor, missing project, unknown gateway operation),
   **When** the agent calls the affected tool,
   **Then** the same structured error codes and messages are returned.

---

### User Story 2 - Maintain and extend tools with localized changes (Priority: P2)

A contributor needs to fix a bug or add a small enhancement to one MCP tool family (for
example, file reading, code search, symbol listing, or gateway dispatch). They can locate
the relevant logic in a focused module without reading unrelated tool implementations in
the same file. Their change touches only the module and tests for that concern.

**Why this priority**: De-monolithing delivers value when day-to-day edits become smaller
and reviewable. Locality of behavior is the main maintainability outcome.

**Independent Test**: Assign a maintainer (or reviewer) a task to change one tool family's
handler logic using only package documentation and file naming; measure that the edit spans
a bounded set of files clearly tied to that family, not the entire monolith.

**Acceptance Scenarios**:

1. **Given** a contributor opens the package to change search-related behavior,
   **When** they follow package layout and naming,
   **Then** search registration, handlers, and helpers reside together outside the
   remaining central wiring file.
2. **Given** a contributor adds a new helper used by one tool family only,
   **When** they implement the helper,
   **Then** it lives alongside that family rather than in a shared catch-all file unless
   genuinely shared across families.
3. **Given** a code review for a single-tool fix,
   **When** reviewers inspect the diff,
   **Then** the change set is scoped to that tool family plus shared wiring if registration
   moved, without unrelated tool logic in the same diff hunk.

---

### User Story 3 - Run focused tests per concern (Priority: P3)

A contributor runs tests scoped to one area (pagination, warnings, coercion, a specific tool
family, gateway routing) and gets fast feedback without executing the entire package through
one mega-test file. New tests for a tool family can live next to that family's code.

**Why this priority**: Testability follows structure. Smaller modules enable table-driven and
handler-level tests that fail precisely when one concern regresses.

**Independent Test**: Run package tests filtered by file or build tag for one concern;
confirm failures pinpoint the affected module and complete in a short wall-clock time
relative to the full package run.

**Acceptance Scenarios**:

1. **Given** tests organized by concern alongside split modules,
   **When** a contributor runs tests for one module only,
   **Then** those tests exercise that concern's logic without requiring the full monolithic
   handler file to compile unrelated paths for unrelated failures.
2. **Given** a regression in warning assembly for search results,
   **When** the focused test suite for that concern runs,
   **Then** it fails with a test name and location tied to search/warning logic, not a
   generic server integration test alone.
3. **Given** the full `go test` for the repository,
   **When** the refactor is complete,
   **Then** all tests pass and coverage for MCP contract behaviors is preserved or improved.

---

### Edge Cases

- **Partial refactor in progress**: Intermediate states during incremental splitting must
  still pass the full test suite; no "half-moved" registration that drops tools from a
  surface.
- **Shared helpers**: Logic used by multiple tool families (pagination, citation building,
  project validation, cursor encoding) must remain single-sourced; splitting must not
  duplicate divergent copies.
- **Gateway dynamic dispatch**: Gateway manifest and call routing must stay consistent;
  splitting must not desynchronize discover vs call metadata.
- **Memory bank and transport**: Memory tools and HTTP vs stdio transport differences must
  remain behaviorally identical (including memory disabled over HTTP).
- **Capability probes at startup**: Tool registration order and gating must not change which
  tools appear when backend capabilities are partial.
- **Import cycles**: Package splits must not introduce circular dependencies that force
  awkward globals or test-only workarounds.

---

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: None intentional. Tool names, input/output schemas, warnings,
  cursors, citations, capability gates, and surface parity (full / compact / gateway) must
  remain unchanged. Any accidental drift is a defect.
- **OpenGrok Semantics**: Unchanged. Search modes, heuristics, truncation, page-local
  behavior, and attribution warnings must not be altered by structural moves.
- **Security Impact**: None. Secrets stay in environment configuration; no new exposure
  paths. Refactor must not log or surface additional sensitive data.
- **Documentation Impact**: Minimal. Agent-facing docs (`README`, `docs/tool-contracts.md`,
  `docs/agent-usage-patterns.md`) update only if internal package layout is documented for
  contributors (optional); no user setup changes.
- **Experimental Impact**: None. Gateway and experimental labels remain as today.
- **Resource Bounds**: Unchanged. Response-size limits, warn thresholds, pagination defaults,
  and auto-fetch behavior must not shift.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST preserve identical MCP tool registration across full, compact,
  and gateway surfaces after the refactor, including capability-gated omission of tools.
- **FR-002**: The system MUST return identical structured outputs, warnings, pagination
  metadata, and citation URLs for the same inputs and backend responses as before the
  refactor.
- **FR-003**: The system MUST return identical error codes and messages for validation
  failures, unknown operations, and backend errors (including existing sentinel codes such
  as invalid cursor, project required, and unknown operation).
- **FR-004**: The system MUST preserve startup behavior: configuration loading, capability
  probing, backend wiring, and transport modes (stdio and HTTP) without new mandatory
  configuration.
- **FR-005**: The system MUST preserve memory tool behavior and HTTP restrictions (memory
  disabled over HTTP) without change.
- **FR-006**: The system MUST split monolithic server logic into multiple cohesive units
  within the MCP server package, each responsible for a clear concern (for example: tool
  registration/wiring, per-tool-family handlers, shared response assembly, gateway dispatch).
- **FR-007**: The system MUST keep shared cross-tool behavior (pagination, warnings,
  coercion, link/citation building, project validation) in reusable units referenced by tool
  families, not duplicated.
- **FR-008**: The system MUST allow contributors to change one tool family with a diff
  scoped primarily to that family's module and its tests.
- **FR-009**: The system MUST include or reorganize tests so each major concern can be
  verified independently while the full repository test suite remains the merge gate.
- **FR-010**: The system MUST not change the exported package API used by `cmd/opengrok-go-mcp`
  unless an equivalent replacement preserves caller behavior without configuration changes.
- **FR-011**: The refactor MUST be deliverable incrementally: each merge slice passes
  `go test ./...` and does not leave duplicate or dead registration paths.

### Key Entities

- **Tool family**: A logical group of MCP tools sharing domain behavior (projects/files,
  search, symbols, read/context, memory, gateway, compact wrappers).
- **Service**: The central type that holds configuration, backend access, link building, and
  memory; coordinates handlers regardless of file layout.
- **Registration surface**: The binding between MCP tool definitions (name, schema,
  description) and handler implementations for a given surface mode.
- **Shared contract helpers**: Pagination, warnings, type coercion, and citation assembly used
  across multiple tool families.
- **Gateway operation**: Manifest entry plus callable handler for dynamic gateway dispatch.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of existing automated tests pass after the refactor with no intentional
  changes to test expectations (behavioral equivalence).
- **SC-002**: A maintainer can identify the module responsible for a given tool family within
  one minute using package layout and naming, without searching a single file exceeding
  roughly one thousand lines of handler logic.
- **SC-003**: At least 80% of tool-family-specific changes in a typical follow-up feature
  touch no more than three non-test source files outside shared helpers and central wiring.
- **SC-004**: Contributors can run tests scoped to one concern (by file or package subtest)
  and receive failing feedback in under half the wall-clock time of the full MCP server
  package test run on the same machine.
- **SC-005**: Zero agent-visible changes: a side-by-side comparison of tool list, JSON
  schemas, and sample tool outputs against a fixed fixture shows no differences.
- **SC-006**: Code review time for isolated tool-family fixes decreases measurably in
  practice (reviewers report smaller, scoped diffs) without increasing post-merge defect
  rate for MCP contract regressions.

---

## Assumptions

- Scope is the `mcpserver` package and its tests; OpenGrok client, configuration, and CLI
  entrypoint behavior change only if required for clean imports, not for feature work.
- The monolithic file is reduced substantially; remaining central wiring may retain
  registration orchestration but not bulk handler implementations.
- Go module and package name `mcpserver` stay the same; splits are additional files or
  internal subpackages only if they avoid import cycles and preserve test ergonomics.
- Best practices for Go MCP servers (handler per tool, clear types, thin registration layer,
  testable backends via interfaces) guide structure but do not mandate a specific file count.
- No new MCP tools, schema fields, or configuration environment variables are introduced.
- Incremental delivery is preferred over a single large change that blocks review.

---

## Out of Scope

- Changing tool descriptions, schemas, defaults, or warnings for product reasons.
- Performance optimization unless required to keep tests and behavior identical.
- Splitting other packages (`opengrok`, `config`) unless strictly necessary to resolve cycles.
- Adding new agent-facing documentation for tool usage (contracts stay the same).
- Introducing new dependency injection frameworks or non-idiomatic abstractions solely for
  structure.
