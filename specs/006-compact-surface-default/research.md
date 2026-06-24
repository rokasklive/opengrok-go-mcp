# Phase 0 Research: Compact Surface as Default

**Feature**: 006-compact-surface-default | **Date**: 2026-06-24

This research resolves the technical unknowns behind making the compact surface
default-eligible. It is deliberately grounded in the project's agent-ergonomics
skills, as requested:

- **agent-ergonomics-inspector** — the 5 laws (L1 Bounded Attention, L2 Interface
  Ground Truth, L3 Boundary Entropy, L4 Compounding Friction, L5 Ergonomic
  Dominance) and the anti-pattern catalog frame *why* each decision matters.
- **design-templates** — shapes `contracts/tool-interface-spec.md` (WHAT the tools
  must contain; typed schemas, progressive disclosure, error format).
- **evaluation-harness-designer** — shapes the eval decisions (measure
  cost-per-*successful-task*, cross-surface equivalence, deterministic graders).
- **implementation-playbooks** — *order is the safety property*: the default flip
  ships last, after parity + equivalence are proven.
- **review-checklists** — shapes the go/no-go gate in `quickstart.md`.

Each decision below records **Decision / Rationale / Alternatives**, with the law
or anti-pattern it answers.

---

## D1. Consolidated, non-overlapping tool taxonomy

**Decision**: Reduce the compact surface to **four capability-gated tools**, each
owning one job, with no cross-tool overlap (the memory tool is omitted — clarified
2026-06-24):

| Tool | Operations | Owns |
|------|-----------|------|
| `opengrok_projects` | `list`, `files`, `overview` | project/file navigation (**closes parity gaps**) |
| `opengrok_search` | `code`, `read` | text/path/history search; `read` = search + surrounding context |
| `opengrok_symbols` | `definitions`, `references`, `find`, `implementations`, `cross_project` | **all** symbol/structure/reference work |
| `opengrok_read` | `file`, `context` | reading a known file (full or line window) |

**Rationale**: The current compact surface still exhibits **anti-pattern #1
(near-duplicate tools / selection ambiguity, L4←L1,L2)**: reference-finding is
duplicated across `opengrok_search` (`references`), `opengrok_symbols`
(`implementations`, `cross_project_references`), and `opengrok_compound`
(`find_symbol_and_references`). An agent choosing "find who calls X" faces three
plausible tools. Consolidating *all* symbol/reference work into `opengrok_symbols`
removes the ambiguity (one obvious tool per task — FR-001/FR-002). The vague
`opengrok_compound` name (an L2 ground-truth failure — the name does not tell the
agent what it does, FR-003) is **removed**: its two operations fold into the tool
that owns their domain (`search_and_read` → `opengrok_search.read`;
`find_symbol_and_references` → `opengrok_symbols.find`). Project file listing and
overview — absent in compact today — become `opengrok_projects.files`/`.overview`,
giving capability parity (FR-007/FR-008) — **except memory, intentionally omitted
from compact** (clarified 2026-06-24; memory stays full-only pending sunset, FR-014).
Four tools sits comfortably inside the ~5–10 healthy-band heuristic and meets SC-002
(stable 3–4 tools, zero overlap).

**Alternatives considered**:
- *Keep the current 6-tool split with overlap, just rename.* Rejected — leaves the
  reference-finding ambiguity (the core complaint) intact.
- *One mega-tool `opengrok` with all operations.* Rejected — collapses distinct
  domains into one oversized discriminated schema; harms tool-selection legibility
  and bloats every `ListTools` (L1). Five domain tools keep each schema scannable.
- *Add `opengrok_navigate` separate from `opengrok_projects`.* Rejected — `list`,
  `files`, `overview` are one domain (project navigation); splitting adds a tool
  without removing ambiguity.

---

## D2. Typed, discriminated-by-operation input schemas

**Decision**: Replace the untyped `{operation, payload: object}` envelope with a
**flattened schema discriminated by `operation`**, composed from the *existing*
typed input structs. For each compact tool, build a `*jsonschema.Schema` whose
top level requires `operation` (a `string` with an `enum` of the enabled
operations) and whose `allOf` carries one `if (operation const) → then (that
operation's properties + required)` branch per operation. Each branch's properties
are generated with `jsonschema.For[T]()` over the operation's current input type
(`SearchCodeInput`, `SymbolSearchInput`, `FileContextInput`, …). Set the result on
`mcp.Tool.InputSchema` (which is typed `any` and accepts a `*jsonschema.Schema`).

**Rationale**: The envelope is an **L2 Interface Ground Truth** failure: a
schema-aware client sees `payload: object` (empty) and must reverse-engineer
required fields from prose (FR-004, SC-004). `mcp.AddTool` only infers a schema
when `InputSchema` is nil and "uses github.com/google/jsonschema-go for inference
and validation" (verified via `go doc`); that package exposes `For[T](opts)
(*Schema, error)` and a `Schema` with `OneOf`/`AllOf`/`If`/`Then`/`Const`/`Enum`
fields — everything needed to compose a discriminated schema. Generating each
branch from the **same input struct the full surface uses** means the compact
schema cannot drift from full (Constitution I: "coherent views over the same
underlying behavior") and there is one source of truth to maintain. The SDK
validator then rejects an unknown `operation` or a missing required field *before*
the handler runs, turning today's prose-only contract into enforced ground truth.

**Alternatives considered**:
- *Flat typed superset (operation enum + all fields optional).* Rejected by the
  user in clarification — it cannot express "field X is required for operation
  code" (weak per-operation signal).
- *Typed `payload` (keep the wrapper, give `payload` a per-operation schema via
  if/then).* Workable and a smaller handler change, but keeps an extra nesting
  level the agent must construct; the clarified target (preview) is flat fields at
  the top level. Flattening is the better ergonomic and is feasible, so it wins.
- *Hand-write JSON Schema literals per operation.* Rejected — duplicates the field
  definitions already encoded in the Go input structs; guarantees drift (anti-pattern
  #3 territory).

**Spike (verification task, low risk)**: Confirm the SDK's bundled validator
accepts an `allOf`/`if`/`then` discriminated schema and rejects (a) an unknown
`operation` and (b) a branch with a missing required field, with a clear error.
If `if/then` proves brittle in the validator, fall back to top-level `oneOf`
branches each pinning `operation` via `const`. Both are expressible with the same
`Schema` fields; this only selects the composition shape.

---

## D3. Scalar coercion under discriminated schemas

**Decision**: Preserve the string-encoded-scalar tolerance (`coerce.go`) for the
new shape by registering, per compact tool, the **union of scalar fields across
all its operations** (e.g. `opengrok_search` registers `tokenized` bool,
`before`/`after` ints from its operations). The receiving middleware keys on the
tool name and rewrites those fields before validation, exactly as today.

**Rationale**: The coercer reflects over the Go `In` type's top-level scalar
fields. Today compact's `In` is `{Operation, Payload json.RawMessage}`, so **no
payload scalar is currently coerced** — a latent gap that string-serializing
clients (e.g. `tokenized:"true"`) would hit. Flattening fields to the top level
and registering the per-operation scalar union both closes that gap and keeps the
behavior identical to the full surface (Constitution V: compatibility; FR-012).

**Alternatives considered**:
- *Per-operation coercion keyed on the `operation` value.* More precise but the
  union is sufficient (a field name maps to one kind across operations here) and
  far simpler. Revisit only if two operations ever give the same field name
  different scalar kinds.

---

## D4. Default-flip ordering (the safety property)

**Decision**: Sequence the work so the **default flip is the last step**, gated on
parity and cross-surface equivalence being proven first. Order:
1. Build the consolidated typed compact tools (D1, D2, D3) — full surface untouched.
2. Close parity (`projects.files`, `projects.overview`) and remove overlap.
3. Extend the eval harness: parameterize contract scenarios onto compact, add
   compact-specific cases, add the cross-surface equivalence assertion, commit the
   compact baseline (D5).
4. **Only then** change `Default()` from `full` to `compact` and write the
   migration note.

**Rationale**: implementation-playbooks — *order is the safety property*. The
default flip is the "optimization" (smaller, cheaper surface) and AP#3 (**silent
interface change without CUJ testing, L2**) is the documented harm: flipping the
default before compact is proven equivalent silently changes which tools every
cold agent sees, with no error signal. Making equivalence + parity the verified
prerequisite (the CUJ tests) is precisely the fix. Each new compact tool is a new
interface/boundary, so its description and schema are engineered *in-step* with its
handler, not deferred.

**Alternatives considered**:
- *Flip the default first, harden later.* Rejected — this is the exact
  optimization-before-safety inversion the playbook forbids; regresses agent
  reliability on `main` until hardening lands.

---

## D5. Eval coverage: compact as a first-class measured surface

**Decision** (resolved in spec clarifications, designed here):
- **Parameterize** the contract suite (`TestEvalSuite`) over surfaces using the
  existing `evals/surface.go` canonical-op→tool resolver, so every scenario runs on
  full **and** compact. Update `resolveCompact` to emit the new tools/operations
  and the typed (flattened) arguments, and remove the `files.list` "no compact
  equivalent" skip (FR-019/FR-023).
- **Add compact-specific cases**: `projects.overview`, `projects.files` on compact,
  an invalid-`operation` error case, and a "construct the call from the typed schema
  alone" case (FR-020).
- **Assert cross-surface equivalence**: on shared scenarios, compact results must
  match full on hits, `citation.url`, pagination cursors/`total_*`, and warnings;
  divergence fails (FR-021).
- **Gate contract, not tokens**: commit a compact baseline under `evals/baselines/`;
  CI fails on a compact contract regression; token deltas stay reported but
  non-gating per the token-benchmark v1 policy (FR-022).

**Rationale**: evaluation-harness-designer — *the harness defines what the system
optimizes for*. Outcome-only, full-only measurement is structurally blind to
compact's correctness (AP#4 flavor). Cross-surface equivalence is the
highest-value validity mechanism here: it makes parity (FR-008) and
contract-preservation (FR-012) *enforced invariants*, not assertions. Cost is
measured as **cost-per-successful-task** (SC-005), the economic metric, not
cost-per-call.

**Explicitly N/A (don't pad)**: This harness uses **deterministic graders**
(fixtures + structural assertions), not an LLM-as-judge. So judge different-family
constraints (DR4), gold-set rotation, and judge-drift calibration **do not apply**
and are not designed. The one human-in-the-loop check is the constitution's
fresh-subagent UX validation (a usability probe, not a scored judge).

**Alternatives considered**:
- *Compact-specific suite only (no parameterization).* Rejected by the user —
  duplicates shared scenarios and risks corpus drift.
- *Gate token thresholds too.* Rejected by the user for v1 — premature economic
  gating invites baseline churn before the surface stabilizes.

---

## D6. Migration and compatibility

**Decision**: Ship a migration note covering (a) the default change
(`full` → `compact`) and the restore path (`OPENGROK_MCP_TOOL_SURFACE=full`), and
(b) the compact tool-name/shape mapping — both from the **prior compact** tools
(`opengrok_compound` removed; `opengrok_search.references` moved to
`opengrok_symbols`) and from **full** tool names to the new compact
tools+operations. The full surface stays byte-for-byte stable (FR-010); full
consolidation is recorded as a **non-binding recommendation** only (FR-018).

**Rationale**: Constitution V + the spec's clarified scope. The default flip is a
public-default break that the constitution permits only "unless a feature spec and
migration note justify the break" — this plan is that justification, tracked in
plan.md Complexity Tracking.

**Alternatives considered**:
- *Silent flip, no note.* Rejected — violates Constitution V and AP#3.
- *Deprecation alias for `opengrok_compound`.* Rejected — compact is explicitly
  non-default today (experimental framing), so its names are not yet a stable
  contract worth aliasing; the migration note suffices.

---

## D7. Naming and experimental labeling

**Decision**: Keep the `opengrok_*` prefix; use operation names that read as verbs/
nouns an agent recognizes (`code`, `read`, `definitions`, `references`, `find`,
`implementations`, `cross_project`, `file`, `context`, `list`, `files`,
`overview`). Remove all "experimental/non-default" framing from compact in tool
descriptions, docs, and config commentary once it is the default; the **gateway**
surface stays labeled experimental (FR-016).

**Rationale**: Constitution V (experimental labeling) + L2 (names are ground
truth). `find` and `cross_project` are shorter and clearer than the prior
`find_symbol_and_references` / `cross_project_references` for an operation already
scoped by its tool (`opengrok_symbols`).

---

## Open risks

- **Validator behavior on discriminated schemas** (D2 spike) — directional until
  the spike confirms `allOf`/`if`/`then` validation; `oneOf`+`const` fallback noted.
- **Equivalence assertion strictness** (D5) — must compare *semantically*
  (same hits/citations/cursors/warnings), not byte-for-byte response envelopes,
  since compact and full wrap identical payloads differently. The assertion
  compares the decoded `*Output` fields, not raw bytes.
- **Description token cost** (L1) — typed schemas plus full L2 descriptions add
  `ListTools` bytes versus the terse envelope; the token benchmark quantifies this
  and SC-005 bounds it (compact ≤ full per successful task). Reported, non-gating.
