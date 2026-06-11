# Feature Specification: Token Economy Eval

**Feature Branch**: `005-token-economy-eval`

**Created**: 2026-06-11

**Status**: Draft

**Input**: User description: "Token economy eval — surface-agnostic scenario benchmarks
that measure MCP byte and estimated token costs across full, compact, and gateway tool
surfaces; deterministic replay (not LLM tool-selection in v1); explicit cold/warm gateway
modes; publish JSON and markdown reports without regression gates initially."

---

## Background & Problem

The project has a **direct-call contract eval harness** (feature 004) that proves tools
return correct structured outputs through a real stdio subprocess. That suite answers
"did the tool behave correctly?" but not "how expensive is a realistic agent workflow?"

Agent ergonomics reviews and surface comparisons (full vs compact vs gateway) need a
**repeatable token-economy benchmark** that:

- Expresses work as **surface-agnostic scenarios** (canonical operations), not per-tool JSON
  cases tied to `search_code` or `opengrok_search`.
- Replays the same logical workflow on each tool surface via adapters.
- Measures **byte costs at MCP boundaries** — especially fixed session bootstrap
  (`ListTools` schemas/descriptions) and per-call request/response payloads.
- Separates **gateway cold vs warm** costs (discovery amortization).
- Splits response bloat into **text vs structured** channels so reviewers can see whether
  overhead is useful code/text or wrapper/metadata/linkage machinery.
- Publishes **diffable artifacts** for CI and human review without failing builds on token
  thresholds until baselines exist.

This feature extends the existing `evals/` package as a second eval mode. It does **not**
change MCP tool contracts or server behavior.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run token economy benchmark in CI (Priority: P1)

A maintainer or CI job runs a token benchmark that builds the MCP server binary, starts a
hermetic OpenGrok stand-in, launches subprocesses for each configured tool surface, replays
a fixed scenario corpus, measures byte costs at MCP boundaries, and writes `token_report.json`
and `token_report.md` as artifacts. The job does **not** fail on token thresholds in v1.

**Why this priority**: Without a published benchmark artifact, surface and default changes
cannot be compared over time or in PR review. This is the core deliverable.

**Independent Test**: Run the benchmark against the hermetic backend; reports are written
for all three surfaces; no orphan subprocesses remain; benchmark completes without a live
OpenGrok instance.

**Acceptance Scenarios**:

1. **Given** hermetic backend and scenarios for full, compact, and gateway surfaces,
   **When** the maintainer runs the token benchmark,
   **Then** each scenario is replayed per surface and per-surface reports include all
   required metrics (see Requirements).
2. **Given** a successful benchmark run,
   **When** the process exits,
   **Then** `token_report.json` and `token_report.md` exist and no MCP server subprocess
   remains running.
3. **Given** v1 configuration,
   **When** token totals exceed any informal threshold,
   **Then** the CI job still passes (artifact-only; no regression gate).

---

### User Story 2 - Compare surfaces on the same scenario (Priority: P1)

A maintainer opens the markdown report and compares **full**, **compact**, and **gateway**
for the same scenario (for example symbol investigation) using `total_cold_bytes`,
`total_warm_bytes`, `list_tools_bytes`, and `call_count` to judge surface economics.

**Why this priority**: The benchmark's value is cross-surface comparison on equal logical
work, not single-surface smoke.

**Independent Test**: Run benchmark once; report table shows the same four scenario types
for each surface with comparable metric columns.

**Acceptance Scenarios**:

1. **Given** scenario "symbol investigation granular" defined with canonical operations,
   **When** benchmark runs on full, compact, and gateway,
   **Then** each surface executes the adapted tool calls for those operations and records
   separate metric rows.
2. **Given** gateway surface,
   **When** report is generated,
   **Then** **cold** totals include discovery cost and **warm** totals exclude
   `discover_bytes` (discovery amortized).

---

### User Story 3 - Identify obese tool definitions and bloated responses (Priority: P2)

A maintainer uses `schema_bytes_by_tool` and top-offender fields (`largest_tool_schema_name`,
`largest_tool_schema_bytes`, `largest_response_bytes`, `largest_response_step`) to find which
tool imposes the most context before any call and which step produced the largest single
response.

**Why this priority**: Average totals hide the failure mode that hurts agents — one bloated
schema or one huge response.

**Independent Test**: After a run, report identifies per-tool schema bytes from `ListTools`
only and names the largest response step for each scenario×surface row.

**Acceptance Scenarios**:

1. **Given** `ListTools` returns multiple tools,
   **When** metrics are aggregated,
   **Then** `schema_bytes_by_tool` counts only each tool's name, description, and input/output
   schema from `ListTools` (not per-call payloads).
2. **Given** a scenario with multiple replay steps,
   **When** one step returns a disproportionately large payload,
   **Then** `largest_response_bytes` and `largest_response_step` reflect that step.

---

### User Story 4 - Add scenarios without harness Go changes (Priority: P2)

A contributor adds or updates scenario definitions in JSON (canonical operations and args)
without modifying replay logic for routine additions. Surface adapters map canonical ops to
surface-specific tool calls in harness code.

**Why this priority**: Scenario corpus must grow as new agent workflows are identified.

**Independent Test**: Add a new scenario JSON file; re-run benchmark; new scenario appears
in reports.

**Acceptance Scenarios**:

1. **Given** a valid scenario with ordered canonical `op` steps and args,
   **When** benchmark loads scenarios,
   **Then** the scenario runs on each surface via adapters.
2. **Given** malformed scenario JSON,
   **When** benchmark loads scenarios,
   **Then** loading fails with a clear validation error before subprocess startup.

---

### User Story 5 - Diagnose response bloat composition (Priority: P3)

A maintainer compares `response_text_bytes` vs `response_structured_bytes` per
scenario×surface to see whether total response cost is dominated by useful text content or
structured wrapper fields (metadata, links, pagination, expansion, warnings).

**Why this priority**: Supports agent-ergonomics tuning (defaults, `response_mode`, link
inclusion) with evidence.

**Independent Test**: Report includes both sub-metrics; when structured dominates text,
reviewer can infer metadata/wrapper overhead without reading raw JSON.

**Acceptance Scenarios**:

1. **Given** a tool response with structured content and optional text content,
   **When** bytes are counted,
   **Then** `response_bytes` equals the sum of measured text and structured channels (both
   may be present on the same call).
2. **Given** `est_tokens` in the report,
   **When** a human reads the markdown report,
   **Then** it is labeled as a heuristic estimate derived from bytes (not model-exact).

---

### Edge Cases

- **Surface capability gaps**: Compact surface lacks some operations (for example directory
  listing). File-exploration scenarios use a **stable canonical path** with steps that exist
  on all surfaces, or mark surface-specific steps as skipped with explicit reporting (not
  silent omission).
- **Gateway discovery**: Cold mode counts `opengrok_discover` (or equivalent) in scenario
  totals; warm mode runs discovery once per harness session and excludes it from warm totals.
- **Full/compact cold vs warm**: In v1, cold equals warm for full and compact (no session
  amortization across scenarios). Session-amortized `list_tools` may be added later.
- **Subprocess lifecycle**: Each surface may use a dedicated subprocess; pipes and children
  must be closed on success, failure, or timeout.
- **Hermetic fixtures**: Scenarios use stable fixture-aligned args (known projects, paths,
  symbols) so byte totals are reproducible in CI.
- **Invalid tool args**: Scenario args must match real tool inputs; harness does not invent
  fields (for example fictional `max_bytes` on read unless such a field exists on the tool).
- **Skipped steps**: When a canonical op has no adapter on a surface, the scenario row
  records skipped steps and does not fabricate comparable totals without annotation.

---

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: None on the server. The benchmark **measures** existing MCP
  surfaces via subprocess calls. No tool names, schemas, warnings, cursors, or citations
  change unless a separate spec requires it.
- **OpenGrok Semantics**: Scenarios use hermetic fixtures and documented best-effort behavior;
  the benchmark does not assert exhaustive semantic knowledge. Canonical ops reflect realistic
  agent workflows (search, read, list, compound symbol).
- **Security Impact**: Hermetic backend in default CI path; no secrets in scenario JSON.
  Reports contain byte counts and metadata, not auth tokens.
- **Documentation Impact**: Maintainer docs for running the token benchmark, interpreting
  metrics, and adding scenarios (`evals/` README section). Optional README eval summary hook
  similar to contract eval. No agent-facing tool description changes.
- **Experimental Impact**: Gateway surface remains experimental; benchmark treats it as a
  measured surface without promoting it to default.
- **Resource Bounds**: Benchmark bounded by scenario count, surfaces (3), and replay steps;
  uses same hermetic backend as contract eval; no unbounded live pagination in v1 scenarios.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a **token economy benchmark** in the existing eval
  package that reuses the stdio subprocess path (build binary, hermetic backend, MCP client
  session).
- **FR-002**: Scenarios MUST be defined as **surface-agnostic** ordered steps using
  **canonical operation identifiers** (for example `search.definitions`, `read.file`,
  `files.list`, `path.search`, `compound.find_symbol`) and operation arguments — not as
  hard-coded tool names per surface.
- **FR-003**: The system MUST provide **surface adapters** that map each canonical operation
  to the correct tool call(s) for **full**, **compact**, and **gateway** surfaces.
- **FR-004**: v1 MUST include **four scenario types**:
  - Symbol investigation **granular** (definition → read → references, decomposed steps)
  - Text search + read
  - File exploration / navigation (stable canonical path; compact-aware — see Assumptions)
  - Compound symbol investigation (native compound operation per surface where available)
- **FR-005**: Replay MUST be **deterministic** in v1: the harness executes adapter-mapped
  steps without an LLM choosing tools. LLM-driven scenario runs are out of scope for v1.
- **FR-006**: The system MUST measure and report these metrics per scenario×surface (and
  aggregate as appropriate):
  - `list_tools_bytes` — total serialized `ListTools` payload
  - `schema_bytes_by_tool` — per-tool bytes from `ListTools` only (name, description,
    input schema, output schema); answers "context surface before first call"
  - `discover_bytes` — gateway discovery response size (zero for non-gateway)
  - `request_bytes` — sum of per-call tool name + arguments JSON
  - `response_bytes` — sum of per-call response payload bytes
  - `response_text_bytes` — text content channel bytes
  - `response_structured_bytes` — structured content channel bytes
  - `largest_response_bytes` — max single-step response size
  - `total_cold_bytes` — first-use total (see FR-007)
  - `total_warm_bytes` — repeated-use total (see FR-007)
  - `est_tokens` — heuristic estimate from bytes (documented as non-model-exact)
  - `call_count` — number of tool calls in the scenario replay
- **FR-007**: Gateway MUST report **two modes**:
  - **Cold**: `list_tools_bytes` + `discover_bytes` + all step `request_bytes` +
    `response_bytes`
  - **Warm**: `list_tools_bytes` + step bytes only (`discover_bytes` excluded)
  For full and compact in v1: **cold equals warm** (no discovery; no cross-scenario
  amortization).
- **FR-008**: Reports MUST include actionable top-offender fields at least at scenario×surface
  granularity: `largest_tool_schema_name`, `largest_tool_schema_bytes` (from
  `schema_bytes_by_tool`), and `largest_response_step` (step index or canonical op id).
- **FR-009**: The system MUST write **two report formats** after each benchmark run:
  - `token_report.json` — machine-readable, suitable for CI diffing and baselines
  - `token_report.md` — human-readable tables and notes (including cold/warm gateway
    semantics and `est_tokens` heuristic label)
- **FR-010**: v1 MUST **not** fail CI on token thresholds; publish artifacts only. Threshold
  gates may be added after baseline runs.
- **FR-011**: The benchmark MUST integrate with `go test` (dedicated test entrypoint) and MAY
  upload reports as CI artifacts on pull requests.
- **FR-012**: Scenarios MUST load from JSON testdata (for example `evals/testdata/scenarios/`)
  without Go changes for routine scenario additions; adapters remain in harness code.
- **FR-013**: When a canonical operation is unavailable on a surface, the benchmark MUST
  record the step as skipped with explicit reporting — not fail silently or invent totals.
- **FR-014**: The benchmark MUST not duplicate contract eval assertions as gates; optional
  per-step `no_error` smoke checks may run but token metrics are the primary output.

### Key Entities

- **Scenario**: Stable id, description, ordered list of canonical operation steps with args.
- **Canonical operation**: Surface-neutral intent (for example `read.file`, `path.search`)
  with a documented adapter mapping per surface.
- **Surface adapter**: Maps canonical op + args → tool name + arguments (or gateway
  operation + payload).
- **Replay step**: One adapter invocation with recorded request/response byte measurements.
- **Token metric row**: Scenario id × surface × (optional gateway mode cold/warm) with all
  required metrics and top-offender fields.
- **Token benchmark result**: Collection of metric rows, timestamps, surface list, scenario
  list, and run metadata.
- **Baseline artifact**: Prior `token_report.json` for optional future delta reporting (not
  required for v1 pass/fail).

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Maintainer can run the hermetic token benchmark without a live OpenGrok
  instance and receive `token_report.json` and `token_report.md` for full, compact, and
  gateway surfaces.
- **SC-002**: Report contains all metrics listed in FR-006 for each of the four v1 scenario
  types per surface.
- **SC-003**: Gateway rows distinguish cold vs warm totals per FR-007; full/compact cold
  equals warm in v1.
- **SC-004**: `schema_bytes_by_tool` is derivable from `ListTools` alone; report names the
  largest tool schema and largest response step for each scenario×surface row.
- **SC-005**: `response_text_bytes` and `response_structured_bytes` are reported separately
  so reviewers can compare wrapper vs content overhead.
- **SC-006**: CI publishes token reports as artifacts without failing on byte/token
  thresholds in v1.
- **SC-007**: Maintainer can identify which surface is cheapest for a given scenario using
  report tables within one minute without reading harness source code.
- **SC-008**: Adding a new scenario via JSON causes it to appear in the next benchmark run
  without harness Go changes (adapters for new canonical ops still require code if ops are
  novel).

---

## Assumptions

- Builds on feature 004 eval harness: same hermetic backend, binary build path, and stdio
  transport patterns.
- Default byte counting uses UTF-8 byte length of serialized MCP-visible payloads; stable
  for CI diffing.
- `est_tokens` uses a simple heuristic (for example total bytes ÷ 4) and is labeled heuristic
  in markdown output — not tied to a specific model tokenizer in v1.
- **File exploration** v1 canonical path stresses navigation without duplicating symbol
  search:
  - Full/gateway: `files.list` → `path.search` → `read.file`
  - Compact (no `files.list`): `path.search` → `read.file` with skipped `files.list` step
    explicitly reported
- Scenario args use real tool fields (`response_mode`, `include_links`, etc.) where economy
  tuning is relevant; no fictional harness-only truncation fields unless added to tools in a
  separate spec.
- One subprocess per surface per benchmark run is acceptable for v1 clarity; session sharing
  optimizations are optional later.
- Contract eval (`TestEvalSuite`) continues to run independently; token benchmark is a
  separate test entrypoint.
- LLM tool-selection and natural-language scenario prompts are deferred to a follow-up
  feature.

---

## Out of Scope

- Changing MCP tool names, schemas, descriptions, defaults, or server behavior.
- CI regression gates on token thresholds in v1 (artifact-only).
- LLM-driven scenario execution or tool-selection accuracy metrics in v1.
- Live OpenGrok token benchmarking as CI default (hermetic only for v1).
- Model-specific tokenizer integration (optional later enhancement).
- Session-amortized `list_tools` across multiple scenarios for full/compact (v1 cold = warm).
- Production agent trace collection or real-user token telemetry.
- Replacing or merging with contract eval pass/fail semantics.

---

## Dependencies

- Feature 004 MCP eval harness (`evals/` package, hermetic backend, subprocess transport).
- Existing tool surfaces: `full`, `compact`, `gateway` (`OPENGROK_MCP_TOOL_SURFACE`).
- Hermetic fixtures aligned with seed corpus (`platform`, `infra`, `PaymentProcessor`,
  `Engine.swift`, etc.).
