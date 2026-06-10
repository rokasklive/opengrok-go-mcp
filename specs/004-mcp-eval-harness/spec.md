# Feature Specification: MCP Eval Harness

**Feature Branch**: `004-mcp-eval-harness`

**Created**: 2026-06-10

**Status**: Draft

**Input**: User description: "MCP eval harness — stdio subprocess eval suite for
opengrok-go-mcp: dataset-driven cases, hermetic OpenGrok backend, scored markdown/JSON
reports, regression tracking. Direct-call deterministic mode (P0); LLM tool-selection
deferred."

---

## Background & Problem

The project already has strong **in-process** test coverage: handler unit tests with a fake
backend, in-memory MCP transports, and startup capability probing in `cmd`. Those layers
prove handlers and registration but do **not** exercise the full path a production MCP
client uses: compile the server binary, spawn a stdio subprocess, send JSON-RPC over pipes,
and validate responses after real configuration and capability gating.

Maintainers and CI lack a **repeatable, dataset-driven end-to-end eval** that:

- Boots the compiled server once per suite (not per case)
- Calls named tools with fixed inputs and checks structured outputs
- Skips cases when a tool is capability-gated (not registered)
- Produces a scored report (pass rate, latency, coverage) that can be compared across runs

This feature adds that fourth layer without duplicating handler-level tests or changing the
public MCP tool contract.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run end-to-end eval suite in CI (Priority: P1)

A maintainer or CI job runs a single test command that builds the MCP server binary, starts
a hermetic OpenGrok stand-in backend, launches the server as a stdio subprocess, executes a
corpus of eval cases against registered tools, and fails the job if any judged case fails
checks or if the aggregate score regresses below a configured threshold.

**Why this priority**: Without a green stdio subprocess suite, regressions in JSON-RPC
framing, env configuration, startup gating, or cross-tool wiring can slip past in-memory
tests. This is the core value of the harness.

**Independent Test**: Run the eval suite against the hermetic backend with no live OpenGrok
instance; all cases for enabled tools pass; report is written; no orphan server processes
remain after the run.

**Acceptance Scenarios**:

1. **Given** a hermetic OpenGrok backend and default full tool surface,
   **When** the maintainer runs the eval suite,
   **Then** each case whose tool is registered is executed, checks are evaluated, and the
   suite completes with a pass/fail outcome and aggregate score.
2. **Given** a case whose tool is not registered due to capability gating,
   **When** the suite runs,
   **Then** the case is marked skipped (not failed) and contributes to coverage metrics as
   "no judgment."
3. **Given** a successful suite run,
   **When** the process exits,
   **Then** no MCP server subprocess remains running.

---

### User Story 2 - Add and maintain cases without Go changes (Priority: P2)

A contributor adds or updates eval cases by editing JSON files grouped by tool (for example
search, read, symbols, projects). They do not need to modify harness Go code for routine
case additions. New cases are picked up on the next suite run.

**Why this priority**: Dataset-driven evals only scale if case authoring stays accessible.
One file per tool keeps diffs reviewable and matches how tools are grouped in the server.

**Independent Test**: Add a new case to a tool's JSON file; re-run suite; new case is
executed and reflected in the report without code changes outside testdata.

**Acceptance Scenarios**:

1. **Given** a valid eval case JSON entry with tool name, input, and result checks,
   **When** the suite loads testdata,
   **Then** the case runs and its outcome appears in per-tool and aggregate scores.
2. **Given** malformed case JSON,
   **When** the suite loads testdata,
   **Then** loading fails with a clear error before subprocess startup (or cases are
   rejected with an explicit validation message).

---

### User Story 3 - Compare eval reports across runs (Priority: P3)

A maintainer runs the suite twice (for example before and after a change) and receives
reports that show per-tool pass rates and latency summaries, with deltas versus the
previous run when a baseline report exists. Reviewers can see regressions at a glance
(for example a tool dropping from 100% to 85% pass rate).

**Why this priority**: A single green run does not catch slow degradation. Delta reporting
makes eval scores actionable in PR review and release gates.

**Independent Test**: Run suite twice; second run writes markdown and JSON reports that
include delta lines per tool when the prior JSON artifact is present.

**Acceptance Scenarios**:

1. **Given** a prior suite result artifact,
   **When** a new suite completes,
   **Then** the report shows per-tool pass-rate and latency deltas versus the baseline.
2. **Given** no prior artifact,
   **When** a suite completes,
   **Then** the report still includes full scores and latencies without delta sections.

---

### Edge Cases

- **Startup failure**: If the subprocess cannot start (no reachable OpenGrok backend, config
  error), the suite fails fast with a clear message — not silent hangs.
- **Subprocess leak**: Stdio pipes and child processes must be closed on success, failure,
  or test timeout.
- **Slow cases**: Cases exceeding a per-case latency budget fail the latency check but
  still record timing for percentile metrics.
- **Partial capability surface**: When list/search tools are enabled but list_files is
  gated, file-list cases skip; project and search cases still judge.
- **Concurrent CI**: Hermetic backend must not bind fixed ports that collide when multiple
  jobs run on one host (use ephemeral binding).
- **Live OpenGrok optional**: Hermetic mode is the default for CI; live-instance mode may
  exist as an optional maintainer workflow but is not required for suite green.

---

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: None on the server. The harness **validates** the existing
  contract (tool names, inputs, outputs, errors, warnings) via direct tool calls. Server
  tool schemas and behavior must not change for this feature unless a separate spec
  requires it.
- **OpenGrok Semantics**: Eval cases must not assert exhaustive semantic knowledge; checks
  validate structured fields, presence, and bounded outcomes consistent with documented
  best-effort behavior. Cases that depend on heuristic search should use stable hermetic
  fixtures, not live index drift.
- **Security Impact**: Hermetic backend only in default CI path; no new secrets in case
  files. Optional live mode uses existing env-based auth patterns; tokens never logged in
  reports.
- **Documentation Impact**: Add maintainer docs for running the eval suite, interpreting
  reports, and adding cases (`README` in eval package or `docs/` section). No agent-facing
  tool description changes.
- **Experimental Impact**: None on MCP tools. LLM tool-selection eval mode is explicitly
  out of scope for v1.
- **Resource Bounds**: Suite boots subprocess once per run; case corpus size and latency
  budgets are configurable with sensible defaults so CI stays bounded.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide an eval suite that builds (or uses) the MCP server
  binary and connects via stdio subprocess transport, matching production client semantics.
- **FR-002**: The system MUST start the server subprocess once per suite execution and
  reuse the session for all cases in that run.
- **FR-003**: The system MUST load eval cases from JSON testdata grouped by tool, without
  requiring Go changes for routine case additions.
- **FR-004**: Each case MUST specify a tool name, input arguments, and one or more
  result checks (for example: no error, field present, minimum result count, latency budget).
- **FR-005**: The system MUST call `ListTools` once per suite and skip (not fail) cases
  whose tool is not registered, recording them as skipped for coverage metrics.
- **FR-006**: The system MUST run in **direct-call** mode for v1: the case names the tool
  explicitly; the harness does not use an LLM to choose tools.
- **FR-007**: The system MUST provide a hermetic OpenGrok backend suitable for CI (no live
  instance required for default suite green).
- **FR-008**: The system MUST produce a human-readable report (markdown) and a machine-readable
  report (JSON) after each suite run, including per-tool pass counts, skip counts, fail
  counts, and latency summaries.
- **FR-009**: The system MUST support delta reporting against a prior JSON result when a
  baseline artifact is available.
- **FR-010**: The system MUST integrate with `go test` so `go test ./evals/` (or equivalent
  package path) runs the full suite and fails on regression.
- **FR-011**: The system MUST not duplicate handler unit tests; cases focus on end-to-end
  tool calls through the subprocess boundary.
- **FR-012**: v1 MUST include eval cases covering at minimum: `list_projects`, `search_code`,
  `read_file` or `get_file_context`, and one symbol-oriented tool (`search_symbol_definitions`
  or `list_symbols`), using hermetic fixtures aligned with those tools.

### Key Entities

- **Eval case**: Tool name, input payload, expected checks, optional tags/description,
  optional latency budget.
- **Result check**: Named assertion against tool response (error flag, JSON field path,
  minimum counts, string contains, etc.).
- **Suite result**: Aggregate and per-tool scores, skip/fail/pass counts, latency
  percentiles, timestamps, mode identifier (direct-call).
- **Harness lifecycle**: Build binary → start backend → spawn subprocess → list tools →
  run cases → teardown → report.
- **Baseline artifact**: Previous suite JSON used for delta comparison.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: CI can run the hermetic eval suite without a live OpenGrok instance and
  complete with 100% pass rate on all judged cases for the seed corpus.
- **SC-002**: Adding a new case via JSON alone causes that case to run on the next suite
  execution without harness code changes.
- **SC-003**: A full suite run leaves zero orphan MCP server processes on the test host.
- **SC-004**: Report includes per-tool pass rate and p50/p95 latency for judged cases; when
  a baseline exists, deltas are visible for at least pass rate per tool.
- **SC-005**: Direct-call mode reports skip coverage (judged vs skipped cases) so reviewers
  understand capability-gated omissions.
- **SC-006**: Maintainer can identify a failing tool and case from the report within one
  minute without reading harness source code.

---

## Assumptions

- Go module and MCP SDK version match the main server (`modelcontextprotocol/go-sdk` v1.4.x).
- Default eval configuration uses full tool surface and stdio transport, matching the most
  common agent integration path.
- Seed testdata and hermetic OpenGrok fixtures can be adapted from the `mcp-eval-harness`
  skill's `test_data_pack` as a starting corpus.
- LLM tool-selection eval mode (MRR, confusion matrix for model-chosen tools) is deferred
  to a follow-up feature; metrics scaffolding may reserve fields but v1 does not require
  them.
- Eval package lives at repository root `evals/` (sibling to `cmd/` and `internal/`), not
  inside `internal/mcpserver`.
- Score regression thresholds for CI (if any) are set in harness config or test code with
  conservative defaults; not agent-facing.

---

## Out of Scope

- Changing MCP tool names, schemas, descriptions, or server behavior.
- Replacing or moving existing `internal/mcpserver` unit or in-memory integration tests.
- LLM-as-judge or model-driven tool-selection eval mode in v1.
- Production observability, dashboards, or scheduled eval runs outside `go test`.
- Evaluating compact or gateway tool surfaces in v1 (may add later as separate case files).
- Performance benchmarking beyond per-case latency checks and suite-level percentiles.
