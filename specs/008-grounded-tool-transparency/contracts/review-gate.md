# Contract: Review Gate (PR checklist for feature 008)

Derived with `review-checklists`. Each item is binary, cites the contract obligation it
verifies, and carries an **evidence standard**: *Documented* (artifact exists) ·
*Tested* (a test produced the behavior) · *Build-enforced* (CI fails otherwise). Behavioral
contracts (descriptions, errors) require **Tested** — a design doc claiming the behavior is
not evidence it happens (L2). Items are scoped to this change only.

**Scope-limitation statement** (mandatory): *This is a contract-compliance verification, not a
system-performance evaluation, and it does not certify that the tests it relies on are
themselves correct.* A pass means "meets the verified contracts," not "ready to ship."

## G1 — Claim registry & bijection  (data-model Entity 1; claim-registry.md)

- [ ] **G1.1** Every registry claim with `gate≠none` has a resolvable `conformance_test_ref`, and every conformance/contract check binds a `claim_id` that exists. — *Build-enforced* — *Fail if:* an orphan claim or orphan test merges without breaking CI.
- [ ] **G1.2** Each claim carries non-empty `ground_truth_source`, `positive_assertion`, and `negative_control` (or, for `unsupported`, the rejection asserted). — *Tested* — *Fail if:* a claim renders into a description with empty evidence fields.
- [ ] **G1.3** `none`-gate claims (limitations) are explicitly justified as bijection-exempt. — *Documented* — *Fail if:* a positively-assertable claim is marked `none` to dodge a test.

## G2 — Descriptions  (description-contract.md; FR-001/002/003/004/012)

- [ ] **G2.1** Each compact tool description is composed from the registry (no hand-prose claim lacking a `claim_id`). — *Tested* (snapshot) — *Fail if:* a syntax claim appears in prose with no registry row.
- [ ] **G2.2** Descriptions state OpenGrok's text+ctags / non-AST-inheritance nature, supported AND unsupported syntax (regex needs `/…/`), ≥1 example, and the named default project when configured. — *Tested* (ListTools snapshot) — *Fail if:* any slot missing.
- [ ] **G2.3** No schema slimming on any surface — compact field docs equal full. — *Tested* (`slimSchema` removed; schema test asserts equality) — *Fail if:* an optional field's description is blank on compact but present on full.

## G3 — Errors & state legibility  (error-taxonomy.md; FR-008/009/010/011)

- [ ] **G3.1** Each of the four validation classes returns a `ToolErrorBody` with the offending operation/field named + a `suggestion`, not a raw `-32602`. — *Tested* (one contract test per class) — *Fail if:* a wrong-operation call yields `oneOf: did not validate`.
- [ ] **G3.2** `QUERY_PARSER_FAILED` is returned for malformed Lucene (400) and is distinct from `UPSTREAM_HTTP_ERROR`, with corrective guidance. — *Tested* — *Fail if:* a bare-regex query maps to the generic upstream code.
- [ ] **G3.3** A zero-result search returns a labeled empty state (`total_hits=0`, `IsError=false`), distinct from error. — *Tested* — *Fail if:* zero results is indistinguishable from a failure.

## G4 — Diagnostics  (FR-014)

- [ ] **G4.1** `diagnostics` is absent by default and present iff `OPENGROK_MCP_DIAGNOSTICS=true`. — *Tested* (on/off snapshot) — *Fail if:* the block appears with the var unset.

## G5 — Evaluation validity  (FR-020–024; SC-001/002/003/006)

- [ ] **G5.1** Trajectory criteria graded deterministically from the tool-call stream; any LLM-judge is different-family + calibrated. — *Documented + Tested* — *Fail if:* SC-001/002 graded by outcome only or by a same-family judge.
- [ ] **G5.2** Reliability criteria reported as Pass^k (k stated), not Pass@k. — *Documented* — *Fail if:* a best-of-k number is labeled "reliability."
- [ ] **G5.3** Token economy reported as cost-per-successful-task; payload/schema bytes are a secondary anomaly check only. — *Tested* — *Fail if:* the gate metric is per-response byte size (would flag de-slimming as a regression).
- [ ] **G5.4** Dual metric present — trajectory quality AND task outcome. — *Tested* — *Fail if:* "avoided the bad query" can pass while the task answer is unusable.

## G6 — Compatibility & coherence  (P-I/P-V; FR-018)

- [ ] **G6.1** Migration notes exist for the public-contract changes (error shape/`suggestion`, default response shape, advertised schema, new env var). — *Documented* — *Fail if:* CHANGELOG/docs omit the default-shape change.
- [ ] **G6.2** full, compact, and gateway surfaces remain coherent (no surface claims what the registry marks unsupported). — *Tested* — *Fail if:* gateway prose contradicts the corrected nature claim.
- [ ] **G6.3** Fresh-subagent first-use run recorded (the audit scenario) per constitution Principle I. — *Tested* — *Fail if:* no first-use evidence for this agent-facing change.

## Cadence

Per-change gate (these items fire on this PR; G2/G3 are description/behavior changes that
cannot be deferred to a scheduled audit). Re-verify G1 bijection on any later claim addition.
Stamp with contract versions (data-model v1, contracts v1) and date on application.
