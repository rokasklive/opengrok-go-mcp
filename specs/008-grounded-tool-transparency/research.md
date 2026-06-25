# Phase 0 Research: Grounded, Test-Backed Tool Transparency

All Technical Context unknowns resolved below. Format: Decision / Rationale / Alternatives.

## R1 — How to de-opaque errors when the SDK validates before the handler

**Decision**: Add a receiving **pre-validation pass** in middleware (the same interception
point the existing `scalarCoercer` already uses, `coerce.go`), running *before* the go-sdk
schema validator. For each compact call it checks, in order: operation present and in the
tool's enabled set; required fields for that operation present; declared scalar types
(post-coercion); no unknown fields. On the first failure it returns the structured
`ToolErrorBody` for the matching error class. The discriminated `oneOf` schema is **kept**
(006 decision preserved); the middleware simply intercepts the four classes that today
collapse into the opaque `oneOf` message, so the SDK's generic validator is no longer the
first responder for them.

**Rationale**: Recon proved the friendly `unknownOperationError` (`compact.go:229`) is dead
code because the SDK rejects out-of-enum/missing-field calls first with
`-32602 oneOf: did not validate` (verified by probe). Intercepting pre-validation
un-shadows the friendly errors without abandoning the typed schema that 006 deliberately
introduced. The coercer already mutates `params.Arguments` pre-validation, so the
extension point exists and is proven.

**Alternatives**: (a) Relax the schema to a flat object and validate entirely in-handler —
rejected: discards the typed discrimination 006 added and the schema's own documentation
value. (b) Post-process the SDK error string — rejected: brittle string-matching on a
transport error, and it cannot recover *which* field offended. (c) Map only in the handler —
rejected: the handler never runs for these classes.

## R2 — Representation of the claim registry (single source of truth)

**Decision**: An in-repo Go data structure (`claims.go`: a slice of `Claim` structs, fields
per `data-model.md`). Description fragments are **composed from** it (the
`compact_descriptions.go` builders read claims for nature/syntax/example text); the
conformance and bijection tests **range over** it. An always-on test asserts the bijection:
every `Claim` with `Gate != none` has a resolvable `ConformanceTestRef`, and every
conformance check registers a `claim_id` that exists — orphan on either side fails the build.

**Rationale**: One structure drives both the prose the agent sees (L2) and the tests that
verify it, so they cannot drift (the prose↔test boundary is where L3 would bite). Go data
keeps it in-language, refactor-safe, and free of a codegen step (Principle V).

**Alternatives**: (a) YAML/JSON + codegen — rejected: adds a build step and a second format
boundary for no gain at this size. (b) Prose-only descriptions with separate hand-written
tests — rejected: this is exactly the drift the feature exists to kill.

## R3 — Using `help.jsp` as ground truth

**Decision**: Treat `help.jsp` as the **authoring** source and the **live** oracle, not a
runtime dependency. Each claim records its `GroundTruthSource` (the help.jsp section it came
from); a captured snapshot of the relevant help.jsp text is pinned under `evals/testdata/`
for offline authoring reference. The live conformance suite validates *behavior* against the
real instance; drift is caught by that suite, not by parsing help.jsp at runtime.

**Rationale**: help.jsp is stable stock-OpenGrok content and confirmed reachable, but parsing
it at runtime would add a fragile boundary and a new failure mode for zero agent-facing
benefit. Recon already used it to correct a planned error (regex *is* supported via `/.../`).

**Alternatives**: Runtime scrape of help.jsp to generate descriptions — rejected (fragility,
L3 boundary, no benefit over a pinned snapshot + live behavioral check).

## R4 — Diagnostics gating

**Decision**: New env var `OPENGROK_MCP_DIAGNOSTICS`, bool, **default off**, parsed via the
shared `strconv.ParseBool` convention already used by `OPENGROK_MCP_PROJECT_SCRAPE`. A new
`Config.Diagnostics bool`. When off, the `diagnostics` object is not populated and is omitted
(`omitempty`); when on, behavior is exactly as today.

**Rationale**: Consistent with the existing toggle convention; recurring per-response L1 win;
opt-in preserves the field for operators who use it (compatibility, Principle V).

**Alternatives**: Remove the block entirely — rejected: it's public output contract; gating
preserves it for opt-in. Keep it always-on — rejected: that's the waste being fixed.

## R5 — Removing schema slimming

**Decision**: Delete `slimSchema`/`slimSchemaInPlace` and the `schemaForCompactType` slimming
step; compact tools advertise the same field-level documentation as full. Confirm the net
effect via cost-per-successful-task, not payload bytes.

**Rationale**: Slimming stripped optional-field legal-value docs (`mode`, `sort`,
`context_budget`) on the *default* surface — trading L2 ground truth for one-time L1 schema
tokens, the trade the user has ruled out (see memory `feedback_no_slimming_l2_over_l1`). The
recurring diagnostics win plus fewer failed calls offsets the one-time schema growth.

**Alternatives**: Extend slimming to drop more optional fields — rejected (deepens the L2
hit). Keep slimming but backfill enums into tool prose — rejected (duplication that drifts;
the schema field is the right home and is now un-slimmed).

## R6 — Eval suite placement

**Decision**: Live behavioral conformance → `evals/conformance_test.go`, gated on the
existing `OPENGROK_MCP_LIVE_EVAL=1` + `OPENGROK_MCP_BASE_URL` (matches `evals/evals_test.go`).
Always-on contract/regression locks → `internal/mcpserver/*_test.go` (no backend). The
claim⇔test bijection check is always-on.

**Rationale**: Reuses the established live gate; keeps backend-free guarantees in CI; matches
existing harness structure (Principle V).

**Alternatives**: New top-level eval package — rejected (fragments the harness).

## R7 — Reliability measurement for the trajectory criteria

**Decision**: Grade SC-001/SC-002 at the **trajectory** level, **deterministically** where
mechanical (scan the issued tool-call stream for disallowed patterns / for a corrective
action addressing the named cause), reported as **Pass^k** over k independent runs (state k),
with a **dual metric** (trajectory quality AND task outcome). Any LLM-as-judge for the
qualitative "answer useful" half must be a different model family and calibrated.

**Rationale**: These are reliability requirements; Pass@k would credit luck (habit 7). The
properties are invisible to outcome-only grading (DR10). Deterministic grading avoids judge
drift and same-family inflation and is cheap (L1). (See `evaluation-harness-designer` audit
folded into the spec's FR-020–FR-024.)

**Alternatives**: Outcome-only / Pass@k / LLM-judge-by-default — all rejected per the audit.

## Open clarifications (non-blocking; defaults assumed)

- Success thresholds (95% / ≥80% / k value, ±tolerance) are directional; confirm against the
  current eval baseline during `/speckit-tasks` or `/speckit-clarify`. Assumed: k=5, SC-001
  Pass^k ≥ 95%, SC-002 Pass^k ≥ 80%, cost-per-successful-task ≤ baseline.
- `OPENGROK_MCP_DIAGNOSTICS` name is the working name (R4); confirmable at implementation.
