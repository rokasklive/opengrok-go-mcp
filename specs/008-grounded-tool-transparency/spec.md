# Feature Specification: Grounded, Test-Backed Tool Transparency

**Feature Branch**: `008-grounded-tool-transparency`

**Created**: 2026-06-25

**Status**: Draft

**Input**: User description: "If we are mixing help.jsp in as ground truth, it needs to be backed by tests — our tools must support the syntax (or parts of it) before we claim it. And we need to be very transparent and upfront in the tool descriptions to cut confusion; the one-time L2 hit is acceptable, but the description should not be a book. No specific constraints, just be conscious of it."

## Context & Motivation

A first-use audit by a cold agent surfaced repeated failed tool calls rooted not in
correctness bugs but in **agent ergonomics** — specifically the Law of Interface
Ground Truth (L2): the tool descriptions are the agent's only reality, and today
they let the agent believe things that are not true.

Observed failures and their root causes (verified against the code during recon):

- The agent expected **AST / inheritance-aware search** (e.g. "find subclasses of
  X") and issued queries OpenGrok cannot answer, because nothing in the descriptions
  states OpenGrok is **full-text + ctags, not an AST/call-graph engine**.
- The agent guessed **unsupported query syntax** (bare regex `class.*extends`) and
  got an upstream 400, because the supported Lucene syntax — including that regex
  *is* supported but only inside `/slashes/` — was never stated.
- Four structurally different mistakes (wrong operation, missing required field,
  wrong scalar type, unknown field) all collapse into one **opaque** message,
  `oneOf: did not validate against any of [...]`, so the agent could not tell which
  problem it had and **misdiagnosed working behavior as broken** (it filed
  `projects[]` and `before`/`after` as bugs when both already work).
- An internal **`diagnostics` block is emitted on every response** with no way to
  disable it, spending attention budget (L1) on three internal integers the agent
  never uses for its task.
- The default (compact) surface **strips optional field descriptions** ("slimming"),
  hiding legal-value enumerations (`mode`, `sort`, `context_budget`) and limitations
  that *do* exist on the full surface — trading L2 ground truth to save one-time L1
  schema tokens.

This feature makes the interface state ground truth — honest, explicit, bounded
descriptions and specific, actionable errors — and guarantees that every capability
claim is **backed by a test** against OpenGrok's own documentation (`help.jsp`), so
the descriptions cannot drift away from what the tools actually do.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent learns what OpenGrok can and cannot do before calling (Priority: P1)

A cold agent reads the tool descriptions and immediately understands that OpenGrok
is a full-text + ctags index — **not** an AST, call-graph, or type-hierarchy engine —
and which query syntax is supported, unsupported, and commonly mis-formed. It
therefore does not attempt inheritance/subclass queries or bare-regex queries; it
either uses a supported text/ctags approach or states the limitation to the user.

**Why this priority**: This is the root-cause fix (L2 / Interface Ground Truth) for
the largest class of observed failures. The interface *is* the agent's reality;
correcting it eliminates whole categories of failed calls without any model change
(L5 / Ergonomic Dominance). Highest agent-impact, lowest dependency.

**Independent Test**: A cold-agent trajectory eval whose task invites an
AST/inheritance assumption; success = the agent avoids issuing an unsupported query
and instead chooses a text/ctags approach or states the limitation. Plus a ListTools
snapshot showing the OpenGrok-nature + supported/unsupported-syntax + example content
present on the **default** surface.

**Acceptance Scenarios**:

1. **Given** the default tool surface, **When** an agent reads the search tool
   description, **Then** it states OpenGrok is full-text + ctags (not AST/call-graph),
   enumerates supported Lucene syntax (phrases, wildcards `* ?`, fields
   `defs:/refs:/path:/hist:/type:`, boolean, fuzzy `~`, proximity, ranges, regex via
   `/.../`), names the common unsupported/pitfall forms (bare regex without slashes,
   wildcards inside quoted phrases), and includes at least one concrete example.
2. **Given** a task that asks for subclasses/implementers of a type, **When** the
   agent plans, **Then** it does not issue an inheritance/AST query and instead uses
   a documented text-search approximation or states that OpenGrok cannot answer it
   semantically.
3. **Given** a server with a configured default project, **When** the agent reads the
   project-scope guidance, **Then** the resolved default project name is stated, so
   the agent knows what omitting `project` targets.

---

### User Story 2 - Every capability claim is backed by a test against ground truth (Priority: P1)

A maintainer (and CI) can verify that every query-syntax/capability asserted in a
tool description is actually supported by the tools, validated against OpenGrok's own
documented behavior (`help.jsp`). A claim with no passing backing test is a defect the
suite surfaces; the server does not claim syntax it cannot demonstrably pass through.

**Why this priority**: This is what makes US1 *trustworthy over time* rather than a
one-off doc edit (L2 + L5; anti-pattern "silent interface change without a CUJ test").
Descriptions are contracts; without a behavioral test they drift or lie. Co-equal P1
because the user made test-backing a hard precondition for claiming anything.

**Independent Test**: Run the conformance suite against a live OpenGrok (under the
existing live-eval gate). Each documented-supported form executes end-to-end through
the MCP surface without an upstream parser error; the suite fails if a description
claims a form the surface cannot pass through, or claims a capability OpenGrok does
not support.

**Acceptance Scenarios**:

1. **Given** the live conformance suite, **When** it exercises each supported syntax
   form named in the descriptions (phrase, wildcard, field filter, boolean, fuzzy,
   proximity, range, `/regex/`), **Then** every one is accepted by OpenGrok through
   the MCP surface (no parser-level rejection).
2. **Given** a description that claims a syntax form, **When** the conformance suite
   runs, **Then** there exists a corresponding check for that claim; a claim with no
   check (or a failing check) fails the suite.
3. **Given** a syntax form OpenGrok does **not** support (or the surface does not pass
   through), **When** descriptions are authored, **Then** that form is documented as
   unsupported rather than claimed.

---

### User Story 3 - Failed calls return a specific, actionable signal (Priority: P2)

When a tool call fails, the agent receives an error that identifies the **cause class**
and names the offending operation/field, with a suggestion it can act on — instead of
one opaque `oneOf` string that is identical for four different mistakes. Distinct
response states (success, empty/zero-result, error, truncated, warning, unauthorized)
remain distinguishable.

**Why this priority**: Opacity (L2) caused blind retries (L1 waste) and misdiagnosis.
P2 because US1/US2 prevent many failures up front, but specific errors are what let an
agent self-correct on the calls that still fail.

**Independent Test**: Contract tests asserting that each failure mode yields a distinct,
named, actionable error: wrong/disabled operation lists the enabled operations; a
missing required field names it; a wrong scalar type states the expected type; an
unknown field is identified; a malformed Lucene query maps to a query-parser error
code with corrective guidance. A zero-result search is a labeled empty state, not an
error or an ambiguous empty payload.

**Acceptance Scenarios**:

1. **Given** an operation not valid for a tool, **When** the call is made, **Then** the
   error names the invalid operation and lists the operations the tool actually
   supports (and, where applicable, points to the correct tool).
2. **Given** a call missing a required field for its operation, **When** validated,
   **Then** the error names the missing field rather than returning a generic schema
   failure.
3. **Given** a malformed Lucene query (e.g. bare-regex), **When** OpenGrok returns a
   parser error, **Then** the tool returns a distinct query-parser error code with
   guidance (e.g. wrap regex in `/.../`, quote phrases).
4. **Given** a search with zero hits, **When** it returns, **Then** the response is a
   clearly labeled empty result (total_hits = 0), distinguishable from an error.

---

### User Story 4 - Default responses spend the attention budget on signal (Priority: P2)

The agent's default responses do not carry per-response internal noise, and the default
tool surface no longer hides honest field documentation. Attention budget (L1) is
balanced by removing recurring waste and using progressive disclosure — never by
omitting contract ground truth (L2).

**Why this priority**: The diagnostics block is recurring per-response waste (L1); the
slimming it was paired against was a one-time cost paid in the wrong currency (L2).
Fixing both keeps the net token economy healthy *and* honest. P2: improves every call
but is not the primary failure driver.

**Independent Test**: A response snapshot shows no diagnostics block by default and the
block present only when explicitly enabled; a ListTools snapshot shows full field-level
documentation (legal values, limitations) on the **compact** surface; the token-economy
benchmark shows net payload not regressed beyond the agreed tolerance.

**Acceptance Scenarios**:

1. **Given** diagnostics are not enabled, **When** any search/symbol response returns,
   **Then** it contains no diagnostics block.
2. **Given** diagnostics are enabled via configuration, **When** a response returns,
   **Then** the diagnostics block is present.
3. **Given** the compact (default) surface, **When** ListTools is inspected, **Then**
   every field that carries legal-value/limitation documentation on the full surface
   carries the same documentation here (no slimming gap).

---

### User Story 5 - Already-correct behavior is locked against regression (Priority: P3)

Behaviors the audit wrongly reported as broken — plural `projects[]` arrays,
string-encoded scalar coercion, and default-project resolution when `project` is
omitted — are pinned by always-on tests so they cannot silently regress and are not
re-reported as bugs.

**Why this priority**: Lowest risk (the behavior already works), but locking it makes
the contract explicit ground truth and protects the US1–US4 changes from accidental
breakage during refactors. P3.

**Independent Test**: Always-on (no live backend) contract tests: a `projects:[...]`
array is accepted and applied to scoping; a string-encoded scalar (e.g. `"10"` for an
integer field) is coerced before validation; a scoped call with `project` omitted
resolves to the configured default without a prior discovery call.

**Acceptance Scenarios**:

1. **Given** an input type that declares `projects`, **When** a `projects:[...]` array
   is sent, **Then** it validates and is applied to project scoping.
2. **Given** a scalar field sent as a JSON string, **When** the call is made, **Then**
   it is coerced to the declared type and accepted.
3. **Given** a configured default project and a call omitting `project`, **When** the
   call is made, **Then** it resolves to the default without requiring a project
   discovery call first.

### Edge Cases

- **Deployment variance**: `help.jsp`/behavior can vary with OpenGrok version or
  indexer options (e.g. leading wildcards disabled via indexer `-a`). Claims target
  stock/default OpenGrok behavior; a capability whose availability depends on indexer
  configuration is documented as conditional, and the conformance suite records the
  configuration it assumes.
- **No live backend in CI**: the conformance suite (US2) is gated/skipped without a
  live OpenGrok; the always-on regression suite (US5) still runs. Capability claims are
  verified whenever a backend is configured.
- **Silent degradation / false pass**: OpenGrok may accept an unrecognized form (HTTP 200)
  while silently falling back to full-text, which would let a parseability-only check
  greenlight a false claim. Conformance therefore checks observable behavior with a
  wrong-control comparison, and treats "supported-but-legitimately-empty" (the form parsed
  and behaved correctly but the corpus has no match) as a pass, distinct from a degraded
  fallback.
- **Description length pressure**: when full honesty would bloat a description, the
  must-know stays inline and depth moves to examples and the capabilities manifest
  (progressive disclosure), so length stays bounded without hiding truth.
- **Consumers of the always-on diagnostics field**: gating it off by default changes the
  default response shape; restored via opt-in configuration and called out in migration
  notes.
- **Gateway (experimental) surface**: its descriptions/errors must not contradict the
  corrected ground truth; consistency is preserved unless a change is explicitly scoped
  to one surface.

## Constitution Alignment *(mandatory)*

- **MCP Contract Impact**: Changes **tool descriptions** (contract); changes **error
  signaling** — adds a suggestion/guidance element and a distinct query-parser error
  code, and makes validation errors name the cause/field (contract); changes the
  **default response shape** (diagnostics omitted unless enabled); changes the
  **advertised schema** on the compact surface (field documentation no longer stripped);
  adds a **configuration/env var** for diagnostics. Behavior kept consistent across full,
  compact, and gateway surfaces unless explicitly scoped. `citation.url` preserved. All
  changes are public contract and require migration notes.
- **OpenGrok Semantics**: This feature *is* a semantics-honesty change — it states
  explicitly that retrieval is full-text + ctags (not AST/call-graph/inheritance), that
  cross-file/implementation/kind results are best-effort or page-local, and that regex is
  supported only via `/.../`. Uncertainty continues to be surfaced via warnings; no new
  claim of semantic certainty is introduced.
- **Security Impact**: One new configuration toggle for diagnostics verbosity; default
  off. No secrets, no new inbound exposure, no change to TLS/raw-fallback posture. The
  diagnostics block contains no sensitive data, but defaulting it off reduces incidental
  internal-detail leakage. Otherwise None.
- **Documentation Impact**: `docs/tool-contracts.md` (error codes/shape, diagnostics
  default, schema-description policy), `docs/limitations.md` (text+ctags vs AST, regex
  `/.../` nuance, best-effort/page-local), `docs/agent-usage-patterns.md` (query-syntax
  guidance), `docs/configuration.md` (diagnostics env var), `docs/agent-ux.md`
  (transparent-but-bounded description policy; no-slimming rule), `README.md` (config),
  `CHANGELOG.md` (contract changes), and `docs/review-checklist.md` (claim ⇔ test rule).
- **Experimental Impact**: None of these are experimental; they harden the stable
  surface. The gateway surface remains experimental and must stay consistent with the
  corrected ground truth.
- **Resource Bounds**: Removing the always-on diagnostics block reduces per-response size
  (recurring L1 win). Removing slimming increases per-session schema size (one-time L1
  cost); this is accepted but bounded — descriptions stay scannable and depth is
  delegated to examples/manifest, with the token-economy benchmark guarding that net
  economy does not regress beyond tolerance.

## Requirements *(mandatory)*

### Functional Requirements

**Ground-truth descriptions (US1)**

- **FR-001**: Tool descriptions MUST state OpenGrok's retrieval model — full-text plus
  ctags definitions — and MUST state that it does **not** provide AST, call-graph,
  type-hierarchy, or inheritance/subclass queries.
- **FR-002**: Tool descriptions MUST document the supported query syntax (at minimum:
  exact phrases, single/multi-char wildcards `? *`, field filters
  `defs: refs: path: hist: type:`, boolean `AND OR NOT + -`, fuzzy `~`, proximity,
  ranges, and regular expressions via `/.../` enclosure) and MUST call out the common
  unsupported or mis-formed inputs (bare regex without slashes, wildcards inside quoted
  phrases, history-only filters used elsewhere).
- **FR-003**: Each tool/operation MUST include at least one concrete example invocation
  in its description.
- **FR-004**: When the server has a resolved default project, the project-scope guidance
  MUST name that default project so the agent knows what omitting `project` targets.

**Test-backed claims (US2)**

- **FR-005**: The server MUST NOT assert a query-syntax or capability claim in a
  description unless the tools demonstrably support it; descriptions and supporting tests
  MUST be kept in correspondence such that a claim lacking a passing backing check is
  surfaced as a defect.
- **FR-006**: Capability claims MUST be validated against OpenGrok's own documented
  behavior (`help.jsp`) via a conformance suite that exercises each claimed syntax form
  end-to-end through the MCP surface, gated to run when a live OpenGrok backend is
  configured. Conformance MUST assert **observable behavior matching the claim**, not
  merely that the query is accepted without a parser error — each supported form needs a
  positive-plus-negative discrimination (the form returns what a deliberately-wrong
  control does not), so a query that silently degrades to full-text cannot false-pass.
- **FR-007**: The "unsupported / pitfall" claims MUST be verified too: the conformance
  suite MUST include negative checks proving the documented failures are real (e.g.
  bare regex without `/.../` is rejected; no AST/inheritance operator exists), so the
  description's unsupported half is ground truth, not assertion.
- **FR-007a**: Capability claims MUST originate from a single machine-checkable **claim
  registry** that drives both the description content and the conformance test matrix,
  so a claim without a backing check (or a check without a claim) fails the build — the
  claim⇔test correspondence MUST be enforced mechanically, not maintained by hand.

**Actionable errors & state legibility (US3)**

- **FR-008**: A failed tool call MUST return a structured error that distinguishes the
  cause class — unknown/disabled operation, missing required field, wrong field type,
  unknown field — and names the offending operation or field, rather than a single
  generic schema-validation message.
- **FR-009**: Validation errors MUST carry actionable guidance the agent can apply (e.g.
  the enabled operations, the required field, the expected type, or the correct tool).
- **FR-010**: Upstream query-parser failures (malformed Lucene → HTTP 400) MUST map to a
  distinct, documented error code with corrective guidance, separate from generic
  upstream-error mapping.
- **FR-011**: Each distinct response state — success, empty/zero-result, error,
  partial/truncated, warning, unauthorized — MUST be distinguishable by the agent; in
  particular a zero-result query MUST be a clearly labeled empty result, not an error or
  an unlabeled empty payload.

**Honest default surface (US4)**

- **FR-012**: The default (compact) tool surface MUST advertise the same field-level
  documentation (legal values, limitations) as the full surface; field-description
  reduction ("slimming") MUST be removed so no surface hides documented behavior.
- **FR-013**: Attention-budget reduction MUST be achieved through removing
  non-task-bearing payload and progressive disclosure (examples, capabilities manifest),
  and MUST NOT be achieved by omitting contract ground truth (descriptions, legal values,
  limitations).
- **FR-014**: The internal diagnostics block MUST be omitted from responses by default and
  included only when explicitly enabled via configuration; the toggle MUST default to off
  and be documented.

**Regression locks (US5)**

- **FR-015**: Inputs that declare a plural `projects` array MUST accept it and apply it to
  project scoping (regression-locked by an always-on test).
- **FR-016**: String-encoded scalar arguments MUST be coerced to their declared types
  before schema validation (regression-locked by an always-on test).
- **FR-017**: When `project` is omitted and a default project is configured, scoped calls
  MUST resolve to the default project without requiring a prior project-discovery call
  (regression-locked by an always-on test).

**Compatibility & bounds (cross-cutting)**

- **FR-018**: All public-contract changes (error shape/codes, default response shape from
  diagnostics gating, advertised schema documentation, new configuration toggle) MUST be
  documented with migration notes and MUST remain consistent across full, compact, and
  gateway surfaces unless a change is explicitly scoped to one surface.
- **FR-019**: Tool descriptions MUST remain bounded and scannable — leading with the
  high-frequency path and delegating depth to examples and the capabilities manifest — and
  the net token economy MUST NOT regress beyond the agreed tolerance as measured by the
  token-economy benchmark.

**Evaluation validity (cross-cutting)**

- **FR-020**: The cold-agent behavior criteria MUST be graded at the **trajectory** level
  (the tool-call stream the agent issued), not by final-answer outcome alone, since
  "avoided an unsupported query" and "addressed the named error cause" are invisible to
  outcome-only checks. Trajectory assertions MUST be deterministic where the property is
  mechanically checkable (e.g. scanning issued calls for a disallowed pattern); any
  LLM-as-judge used for the qualitative "answer is useful" half MUST be a different model
  family from the system under test and MUST be calibrated against a human gold-set.
- **FR-021**: Reliability-style criteria (e.g. "in ≥X% of runs") MUST be measured as
  per-run consistency over k independent runs (Pass^k), never as best-of-k (Pass@k);
  the criterion MUST state k.
- **FR-022**: Behavioral criteria MUST use a **dual metric** — trajectory quality AND task
  outcome — so the agent cannot score well by avoiding the unsupported query through giving
  up; it must avoid the bad call **and** still produce a useful, source-grounded answer.
  Self-correction MUST be credited only when the corrective action addresses the cause the
  error named, not for a blind retry that happens to succeed.
- **FR-023**: Token economy MUST be evaluated as **cost-per-successful-task** across a
  representative trajectory (including retries avoided by clearer errors/descriptions), with
  per-response payload and schema size demoted to a secondary anomaly check — so the
  one-time description-size increase is not misread as a regression while whole-task cost
  improves.
- **FR-024**: The cold-agent eval set MUST be rotated and seeded from real failure
  transcripts (starting with the audit that motivated this feature) rather than frozen, to
  avoid over-fitting descriptions to a fixed task list; and the evaluation's own token cost
  MUST be bounded via sampling and a run cadence rather than running the live/agent suites
  on every commit.

### Key Entities *(include if feature involves data)*

- **Tool description**: the prose contract for a tool/operation — OpenGrok nature,
  supported/unsupported syntax, examples, and resolved default project. The agent's
  ground truth (L2).
- **Capability claim**: a specific supported-syntax or behavior assertion within a
  description, paired one-to-one with a backing conformance check.
- **Ground-truth reference**: OpenGrok's `help.jsp`-documented query syntax, the source
  of truth the conformance suite validates claims against.
- **Structured error**: the failure payload — cause class, offending operation/field,
  error code, and actionable suggestion.
- **Diagnostics block**: internal pagination/expansion metadata, now opt-in via
  configuration.
- **Response state**: the distinguishable outcomes a call can emit (success, empty, error,
  truncated, warning, unauthorized).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a cold-agent eval whose task invites an AST/inheritance assumption, the
  agent both (a) avoids issuing an unsupported query — graded deterministically from the
  tool-call stream — and (b) still produces a useful, source-grounded answer, in at least
  95% of runs measured as Pass^k (each of k independent runs passes), with k stated.
- **SC-002**: For failures caused by wrong operation, missing required field, wrong type,
  or unknown field, the agent's next action addresses the cause the error named (not a
  blind retry) and the corrected call succeeds, in a substantial majority of cases (target
  Pass^k ≥ 80%), measured by trajectory eval — replacing today's opaque-error retry loops.
- **SC-003**: 100% of capability claims in tool descriptions are behaviorally verified
  (positive-plus-negative discrimination, plus negative checks for documented unsupported/
  pitfall forms) and are driven by the machine-enforced claim registry; zero
  claimed-but-unverified forms and zero tests without a claim.
- **SC-004**: 0 responses contain a diagnostics block when diagnostics are not enabled; the
  block appears in 100% of responses when enabled — verified by contract tests.
- **SC-005**: 100% of fields that carry legal-value/limitation documentation on the full
  surface carry the same documentation on the compact surface (no slimming gap).
- **SC-006**: Cost-per-successful-task (tokens across a representative search/read
  trajectory, including retries avoided by clearer errors/descriptions) does not regress and
  ideally improves versus the current baseline; per-response payload and schema size are
  tracked only as a secondary anomaly check, not as the pass/fail metric, so the one-time
  description-size increase is not misread as a regression.
- **SC-007**: The three previously-misreported behaviors (`projects[]` arrays, scalar
  coercion, default-project resolution) are each covered by an always-on regression test
  that passes.

## Assumptions

- "Backed by tests" means: every syntax/capability claim has a corresponding conformance
  check; the live conformance suite runs under the existing live-eval gate
  (`OPENGROK_MCP_LIVE_EVAL` + `OPENGROK_MCP_BASE_URL`), while regression locks run without a
  backend in normal CI.
- Scope is **descriptive then corrective**: the feature first claims only what the current
  surface verifiably supports; expanding pass-through support for a high-value form (e.g.
  ensuring `/regex/` or a field filter reaches OpenGrok untouched) is in scope only where a
  needed claim fails its conformance check — otherwise the form is documented as
  unsupported rather than engineered in.
- Diagnostics is **gated off by default**, not removed, controlled by a dedicated
  environment variable (working name `OPENGROK_MCP_DIAGNOSTICS`), preserving opt-in access
  for operators who use it.
- Claims target stock/default OpenGrok behavior as rendered by `help.jsp`; configuration-
  dependent capabilities are documented as conditional.
- Success thresholds (95%, ≥80%, ±5%) are initial targets to be confirmed during planning
  against current eval baselines; they are directional, not contractual.
- The capabilities manifest (`opengrok://capabilities`) is the progressive-disclosure home
  for depth that does not belong inline in tool descriptions.
- No specific length limit is imposed on descriptions per the user's direction; "bounded
  and scannable" is enforced qualitatively plus via the cost-per-successful-task benchmark,
  not a hard character cap.
- Evaluation methodology follows the audited design: trajectory-level deterministic grading
  where possible, Pass^k for reliability criteria, dual outcome+trajectory metrics,
  cost-per-successful-task as the economic metric, and a different-family calibrated judge
  if any LLM-as-judge is used. These are spec-level commitments; the concrete grader/harness
  build is deferred to planning/implementation.
- Scale testing is **N/A**: this is a single stdio/HTTP server exposing ~4 consolidated
  tools with no multi-agent fan-out, so compounding-friction scale curves are not a concern
  for this feature.
- The claim registry is assumed feasible as a single in-repo source of truth; if a claimed
  form cannot be expressed as a discriminating live check, it is documented as unsupported
  rather than claimed (consistent with the descriptive-then-corrective scope).
