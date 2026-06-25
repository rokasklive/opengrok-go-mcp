# Contract: Per-Tool Description (compact surface)

Every compact tool description MUST fill these slots, composed from the claim registry (no
free-floating prose that can drift from the tests). Bounded and scannable; **no slimming** —
field-level schema docs stay present on compact, identical to full.

## Required slots (per tool)

1. **Lead line** — the high-frequency purpose, first. (FR-019, L1)
2. **OpenGrok nature** — references `nature_claim_ref` (full-text + ctags; **not** AST /
   call-graph / inheritance). Rendered once from the shared claim, not re-prosed per tool.
   (FR-001, P-II)
3. **Operation catalog** — the enabled operations (capability-gated) with a one-line blurb
   each, so the agent picks the right operation up front (prevents `UNKNOWN_OPERATION`).
4. **Supported syntax** — references the `supported`/`conditional` query-syntax claim_ids
   relevant to this tool, with at least the high-frequency forms inline. (FR-002)
5. **Unsupported / pitfalls** — references the `unsupported`/`limitation` claim_ids (bare
   regex needs `/…/`; wildcards not inside quoted phrases; no inheritance/AST). (FR-002)
6. **Example** — ≥1 concrete example (from a claim's `example`). (FR-003)
7. **Default project** — when configured, names the resolved default so omitting `project`
   is understood. (FR-004)
8. **Disclosure split** — must-know inline; depth (full syntax catalog, edge conditions)
   deferred to `opengrok://capabilities`. (FR-013/019, L1)

## Cross-surface coherence (P-I, FR-018)

- The `full` surface descriptions stay coherent with the same claims (no contradictory
  prose); they need not be re-templated but MUST NOT claim what the registry marks
  unsupported.
- The gateway (experimental) surface MUST NOT contradict the corrected ground truth.

## Bounded check (Evidence-required)

Each tool description records its scannability/length outcome and contributes to the
cost-per-successful-task baseline (SC-006). "Bounded" is enforced by that measurement, not a
character cap (per user direction). A description that grows past scannable pushes depth to
the manifest rather than dropping ground truth.

## Anti-requirements (what this contract forbids)

- No slimming/stripping of field descriptions on any surface (FR-012).
- No claim in prose that lacks a registry entry (and therefore a test) — prose is generated
  from the registry, closing the drift boundary.
