# Data Model & Artifact Templates

Designed with `design-templates`: every field is an **obligation** (Mandatory),
**Evidence-required** (demands a concrete test/measurement), or **Advisory**. Each traces to
a spec FR, a constitution principle (P-I…P-V), or an agent-ergonomics law (L1/L2). Empty
evidence-required fields produce a build/test warning, never a silent default.

**Ownership boundary**: this defines WHAT each artifact must contain. HOW to build the
registry loader, middleware, and graders is implementation (`/speckit-tasks` →
implementation), not specified here.

---

## Entity 1 — Claim (the Claim Registry)

One row per capability claim. Single source of truth that drives **both** description prose
and the conformance test matrix. The registry is the contract; the bijection is enforced.

| Field | Tier | Obligation it forces |
|-------|------|----------------------|
| `claim_id` | Mandatory | Stable join key linking description prose ↔ test (FR-007a). Without it the bijection has nothing to verify. |
| `category` | Mandatory | `query-syntax` \| `behavioral-guarantee` \| `limitation` — separates "what you can do" from "what you cannot" so limitations are first-class, not omissions (FR-001/002, P-II). |
| `support_status` | Mandatory | `supported` \| `unsupported` \| `conditional` — forces the unsupported/pitfall half onto the page (FR-002, P-II). |
| `condition` | Evidence-required (if `conditional`) | The indexer/version dependency (e.g. leading wildcard needs no `-a`), with the source — prevents claiming a deployment-specific capability as universal (Edge: deployment variance, P-II page-local). |
| `agent_claim_text` | Mandatory | The exact prose the agent is told. This *is* the L2 ground truth the description renders (FR-001/002). |
| `example` | Mandatory | A concrete invocation/query proving the claim — the cold agent's copyable pattern (FR-003). |
| `applies_to` | Mandatory | Tool(s)/operation(s) the claim governs — scopes prose and keeps surfaces coherent (FR-018, P-I). |
| `disclosure_location` | Mandatory | `inline-description` \| `capabilities-manifest` — WHERE prose lives, so L1 is balanced by progressive disclosure, not omission (FR-013/019, L1). |
| `ground_truth_source` | Evidence-required | The `help.jsp` section/snapshot backing the claim — no claim without a documented source (FR-006, P-II). |
| `conformance_test_ref` | Evidence-required | The test that verifies it. Empty (for `Gate≠none`) → build fails (FR-005/006, the bijection). |
| `positive_assertion` | Evidence-required | What the test asserts the form *does* behaviorally (not "parsed OK") (FR-006). |
| `negative_control` | Evidence-required | The wrong-control the form must out-discriminate, or — for `unsupported` — the rejection it must produce. Defeats silent-degradation false-pass (FR-006/007). |
| `gate` | Mandatory | `live` \| `always-on` \| `none` — which suite runs it; `none` only for pure-prose claims exempt from the bijection, which must be justified (FR-006). |

**Invariant (enforced by always-on test)**: bijection — `{claim_id with gate≠none}` ⇔
`{claim_id registered by a conformance/contract check}`. Orphan on either side fails.

---

## Entity 2 — StructuredError (the Error Taxonomy)

One row per error class. Replaces the single opaque `oneOf: did not validate`.

| Field | Tier | Obligation it forces |
|-------|------|----------------------|
| `error_code` | Mandatory | Stable machine code (`UNKNOWN_OPERATION`, `MISSING_REQUIRED_FIELD`, `INVALID_FIELD_TYPE`, `UNKNOWN_FIELD`, `QUERY_PARSER_FAILED`, plus existing `FILE_NOT_FOUND`/`UPSTREAM_HTTP_ERROR`/…). Public contract; tested (FR-008/010, P-I). |
| `cause_class` | Mandatory | Human-readable category distinguishing the four collapsed cases (FR-008). |
| `offending_locus` | Mandatory | `operation` \| `field` \| `query` \| `resource` — what the message must name (FR-008). |
| `locus_value_source` | Evidence-required | How the offending value is determined and surfaced (e.g. field name from the validation pass, enabled-ops list from config) — proves the error actually names it, not generically (FR-008). |
| `suggestion` | Mandatory | The actionable next step (e.g. "enabled operations: …"; "wrap regex in /…/"; "did you mean opengrok_search?") (FR-009). |
| `response_state` | Mandatory | `error` — and the contract asserts it stays distinct from `success`/`empty`/`truncated`/`warning`/`unauthorized` (FR-011, L2 state legibility). |
| `detection_point` | Mandatory | `pre-validation-middleware` \| `handler` \| `upstream` — names where it must be produced so it is not shadowed by the SDK's generic validator (the recon root cause; R1). |
| `surfacing` | Mandatory | Must reach the agent as the structured `ToolErrorBody` (with `suggestion`), never a raw transport `-32602` (FR-008). |
| `test_ref` | Evidence-required | The contract test asserting this class end-to-end (P-III). |

**Invariant**: each non-success response maps to exactly one `response_state`; a zero-result
search is `empty`, never `error` (FR-011).

---

## Entity 3 — ToolDescriptionContract (per compact tool)

A checklist of required content slots; filled once per compact tool. Bounded/scannable, no
slimming.

| Field | Tier | Obligation it forces |
|-------|------|----------------------|
| `tool` | Mandatory | Which tool this contract governs. |
| `lead_line` | Mandatory | High-frequency purpose, stated first (front-loaded) (FR-019, L1 scannability). |
| `nature_claim_ref` | Mandatory | References the shared "text+ctags, not AST/inheritance" claim_id (deduped, not re-prosed) (FR-001, P-II). |
| `operation_catalog` | Mandatory | Enabled operations + one line each — makes wrong-operation preventable up front (FR-008 prevention, discoverability). |
| `supported_syntax_refs` / `unsupported_syntax_refs` | Mandatory (where applicable) | claim_id references for the syntax the tool accepts and the pitfalls it rejects (FR-002). |
| `example_ref` | Mandatory | ≥1 example (a claim with an `example`) (FR-003). |
| `default_project_slot` | Mandatory (if a default is configured) | Names the resolved default so the agent knows what omitting `project` targets (FR-004). |
| `disclosure_split` | Mandatory | What stays inline vs. deferred to `opengrok://capabilities` (FR-013/019, L1). |
| `no_slimming` | Mandatory | Asserts field docs are NOT stripped on compact (FR-012; memory `feedback_no_slimming_l2_over_l1`). |
| `bounded_check` | Evidence-required | The scannability / cost-per-successful-task result for this description — not a guess (FR-019/023). |

---

## Supporting entities

- **ResponseState** — the enumerable outcomes a call emits: `success`, `empty`,
  `truncated`, `warning`, `unauthorized`, `error`. Each must be distinguishable by the agent
  (FR-011, L2). Pre-existing shapes (pagination, warnings) are audited to confirm each state
  is labeled, not inferred from absence.
- **Diagnostics** — internal `{offset_used, opengrok_start, opengrok_max_results}`; gated by
  `Config.Diagnostics` (default off); omitted when off (FR-014).
- **ConformanceCase** — a runtime check bound to a `claim_id`: issues the `example` through
  the MCP surface against live OpenGrok and asserts `positive_assertion` holds and
  `negative_control` is discriminated (FR-006).

## State transitions

None (stateless request/response). The only lifecycle is the **claim**: `authored` → has
`ground_truth_source` → has `conformance_test_ref` (bijection green) → rendered into a
description. A claim cannot reach "rendered" without passing the bijection.
