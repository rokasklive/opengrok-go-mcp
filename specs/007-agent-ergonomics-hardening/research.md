# Phase 0 Research: Agent Ergonomics Hardening

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24

Research resolves plan-phase unknowns and grounds every decision in the project's
agent-ergonomics skills:

- **agent-ergonomics-inspector** — laws L1–L5 and anti-patterns #2, #3, #4, #7 as
  the *why* behind each choice.
- **design-templates** — shapes `contracts/` (WHAT agents see; progressive
  disclosure; error/actionable copy).
- **evaluation-harness-designer** — trajectory graders measure purpose (warning
  heeded, correct tool path), not field presence alone; ListTools gate guards L1.
- **implementation-playbooks** — order is the safety property: quick wins and
  additive contracts first; profile + manifest next; eval gates last.
- **review-checklists** — shapes `quickstart.md` go/no-go (Tested > Documented).

Each decision: **Decision / Rationale / Alternatives**, with the law or
anti-pattern it answers.

---

## D1. Agent profile bundles (economy vs rich)

**Decision**: Add `OPENGROK_MCP_AGENT_PROFILE` with values `economy` and `rich`.
When unset, behave as **`economy`** (shipped default). Set `rich` to restore expanded defaults.
explicitly flips the shipped default (spec FR-003).

| Setting | `rich` (default) | `economy` |
|---------|------------------|-----------|
| Auto context expansion (`expand_context` default) | `true` | `false` |
| Response detail (`response_mode` default when omitted) | `full` | `compact` |
| Include links (`include_links` default) | `true` | `false` |

Per-call fields (`expand_context`, `response_mode`, `include_links`,
`include_snippets`, `context_budget`, `page_size`) **always override** the profile
(FR-002). Implementation: resolve defaults in `Service` helpers
(`shouldExpandContext`, new `resolveResponseMode`, `includeLinks`) reading
`cfg.AgentProfile` when the per-call pointer/string is empty.

**Rationale**: **Anti-pattern #7 / L1** — optimizing descriptions while leaving
verbose tool output as the happy path wastes ~67% of trace budget on responses.
Bundling knobs into one operator-facing profile is **L5 Ergonomic Dominance**: a
structural lever beats expecting every cold agent to discover five optional fields.
`economy` targets multi-step investigation; `rich` preserves one-shot helpfulness
for operators who want it.

**Alternatives considered**:
- *Flip shipped defaults without a profile knob.* Rejected — breaks Constitution V
  backward compatibility without migration note; spec defers default flip.
- *Only document economy knobs in prose.* Rejected — L2 failure; agents won't read
  docs before first call.
- *Separate env vars per default.* Rejected — reproduces the discovery problem;
  one profile is the progressive-disclosure layer operators need.

**SC-001 validation**: Re-run `TestTokenBenchmark` with
`OPENGROK_MCP_AGENT_PROFILE=economy` on compact; target ≥15% warm-total reduction
vs rich on the four existing scenarios. Record actual delta in `gate-results.md`.

---

## D2. `response_mode=compact` naming — prose disambiguation only (v1)

**Decision**: **Keep** the JSON value `compact` for terse response detail in v1.
Disambiguate in tool descriptions and `response_mode` field copy:

- **Tool surface** `compact` = four consolidated MCP tools (`OPENGROK_MCP_TOOL_SURFACE`).
- **Response detail** `response_mode=compact` = lean per-result payload (citation kept;
  redundant URLs and auto-expansion skipped for that call).

Add one sentence to each affected compact tool header (FR-011). Do **not** rename to
`terse` in this feature — avoids a breaking input-value change and duplicate
migration; revisit as `008-response-detail-terse` if confusion persists in
fresh-subagent probes.

**Rationale**: **L2 Interface Ground Truth** — shared vocabulary creates false
beliefs ("I'm on compact surface so responses are already lean"). Prose fix is
cheap; value rename is a contract migration.

**Alternatives considered**:
- *Accept `terse` as alias input, emit `compact` in output.* Workable follow-up;
  deferred to limit scope.
- *Rename value to `terse` now.* Rejected — breaking for existing agents passing
  `response_mode=compact`; needs version bump and migration map.

---

## D3. Dynamic `expand_context` schema text

**Decision**: At tool registration time, set `expand_context` jsonschema description
from the active profile:

- Rich: `"Optional. Defaults to on (auto-expands file context around hits). Set false to skip."`
- Economy: `"Optional. Defaults to off. Set true to include extra lines around hits."`

Apply to all search/symbol input types registered on compact **and** full surfaces
(shared structs get description patched in `register_*` after `jsonschema.For`, or
via a small `patchExpandContextDescription(schema, profile)` helper).

**Rationale**: **L2** — current copy says "set true to include" while default is
already on (anti-pattern: description contradicts behavior). Schema is ground truth
for schema-aware clients.

---

## D4. Capability manifest as MCP resource `opengrok://capabilities`

**Decision**: Register a **fixed URI** resource always available (even when all
search tools are gated):

```text
opengrok://capabilities
```

Payload (JSON):

- `tool_surface` — `compact` | `full` | `gateway`
- `agent_profile` — `economy` | `rich` | `""` (rich-equivalent)
- `tools[]` — `{ name, operations[], description_one_liner }` mirroring live
  `ListTools` registration
- `gated[]` — families probed but disabled: `{ capability, reason_code, remediation }`
- `project_catalog` — `{ source, snapshot: true, project_count, default_project }`

Build `tools[]` by reusing `compact*Operations(cfg)` / full registration helpers —
**no static superset** (FR-006). Build `gated[]` from new `config.CapabilityReport`
populated in `main.go` during startup probes (today only `logf`; persist structured
outcomes).

Remediation strings reference env var **names only** (Constitution IV): e.g.
`OPENGROK_MCP_API_TOKEN`, `OPENGROK_MCP_PROBE_FILE` — never values.

**Rationale**: **L2 + L3** — static docs (`agent-usage-patterns.md`) drift from
runtime gated schemas; agents plan impossible workflows. Manifest is passive
ground truth at session start (Information Architecture domain). Matches existing
`opengrok://projects` pattern (`resources.go`).

**Alternatives considered**:
- *New `opengrok_discover_capabilities` tool.* Rejected — adds ListTools bytes
  (L1/L4); resource is fetch-on-demand progressive disclosure.
- *Embed manifest in every tool description.* Rejected — stale on partial gate;
  duplicates per-tool budget cost.
- *Stderr-only probe hints.* Rejected — agents cannot see logs (L2).

---

## D5. Kind-filter additive output fields

**Decision**: Extend `ListSymbolsOutput` additively when `kind` filter is non-empty:

| Field | Type | Meaning |
|-------|------|---------|
| `kind_filter_active` | `bool` | `true` when `kind` was requested |
| `kind_matches_on_page` | `int` | Count of symbols matching `kind` on this page |
| `total_hits_scope` | `string` | `"pre_kind_filter"` when kind active; omit otherwise |

Keep existing `KIND_FILTER_PAGE_LOCAL` warning (belt and suspenders). When kind
filter active and `kind_matches_on_page == 0` but `has_more`, warning text already
covers continue-pagination case — add eval assertion.

**Rationale**: **L2** — `total_hits` reads like a global cardinality; agents
integrate it as fact. Structured fields make the trap visible without parsing
warning prose (anti-pattern #4 outcome-only blind spot for *interpretation*).

**Alternatives considered**:
- *Rename `total_hits` when kind active.* Rejected — breaking pagination contract.
- *Warnings only.* Rejected — review found warnings easy to miss in large JSON.

---

## D6. Project catalog metadata

**Decision**: Add to `ListProjectsOutput` and `opengrok://projects` resource:

| Field | Type | Example |
|-------|------|---------|
| `catalog_source` | `string` | `configured`, `api`, `scraped`, `none` |
| `catalog_is_snapshot` | `bool` | always `true` (startup-resolved list) |

Extend `UNKNOWN_PROJECT` message with: *"Project list is a startup snapshot; restart
the server after OpenGrok adds projects."* when `catalog_is_snapshot` (FR-010).

**Rationale**: **L3 Boundary Entropy** — session/time boundary without invalidation
signal; agents trust stale catalog.

---

## D7. Trajectory eval harness (deterministic v1)

**Decision**: Add `evals/trajectory_test.go` + `evals/graders.go` with **deterministic**
graders (no LLM-as-judge in v1 — spec out of scope):

| Grader type | Checks |
|-------------|--------|
| `tool_sequence` | Expected compact tool+op order per scenario step |
| `warning_code` | `warnings[].code` contains code (e.g. `HIGH_HIT_COUNT`) |
| `citation_present` | `results[].citation.url` or symbol equivalent |
| `field_eq` / `field_present` | Kind-filter metadata, catalog fields |
| `description_cuj` | Scripted resolver: task label → expected first tool (guards anti-pattern #3) |

Reuse `evals/testdata/scenarios/*.json` steps where possible; add
`evals/testdata/trajectory/*.json` for multi-step cases. Minimum: **3 scenarios,
8 graders** (FR-012, SC-004).

**Rationale**: **evaluation-harness-designer** — outcome-only contract eval misses
55.6%-class process failures; trajectory is the deliverable for agent-facing
regressions. Deterministic graders are stable in CI (no judge drift).

**Alternatives considered**:
- *LLM-as-judge for tool choice.* Deferred — needs human gold-set (spec out of scope).
- *Extend contract cases only.* Insufficient for sequence and description CUJ (FR-014).

---

## D8. Compact ListTools CI ceiling

**Decision**: In `TestTokenBenchmark`, after computing compact `list_tools_bytes`,
**fail** if `list_tools_bytes > baseline * 1.02` (2% slack) OR
`> OPENGROK_MCP_EVAL_LISTTOOLS_CEILING` env override for emergencies.

Committed ceiling in `evals/baselines/token_report.json` field
`compact_list_tools_ceiling_bytes` (initial value = current baseline + 2% headroom).
Refresh via `./scripts/update-eval-results.sh` with explicit review when schema
growth is intentional.

**Rationale**: **L1 + L5** — ListTools is a session tax; silent schema bloat evaded
measurement (token benchmark v1 was report-only). Gate is compact-only; gateway
already minimal.

---

## D9. Tool-header economy hints (quick win)

**Decision**: Append one sentence to `compactSearchDescription`,
`compactSymbolsDescription`, `compactReadDescription` (and full equivalents where
shared):

> *Large sweeps: `include_snippets=false`, `response_mode=compact` (lean payload —
> not the compact tool surface), `expand_context=false`; deep reads: use
> `opengrok_read` or `expand_context=true`.*

**Rationale**: **L2 progressive disclosure** at the interface layer — highest-signal
knobs visible before external docs.

---

## D10. Implementation order (playbook)

1. **Wave 1 — additive + copy** (low risk): catalog metadata, kind-filter fields,
   UNKNOWN_PROJECT copy, tool-header hints, `expand_context` schema text,
   `agent-usage-patterns.md` capability preamble, docs.
2. **Wave 2 — profile**: config + helper resolution + benchmark profile dimension.
3. **Wave 3 — manifest**: `CapabilityReport` at startup + `opengrok://capabilities`.
4. **Wave 4 — eval gates**: trajectory suite + ListTools ceiling + description CUJ.
5. **Wave 5 — validation**: fresh-subagent probe on real instance + `gate-results.md`.

Default profile flip to `economy` is **explicitly out of this feature** (separate
migration if SC-001 passes).

---

## D11. Compound/heuristic diagnostics (FR-015)

**Decision**: Audit `SearchAndRead`, `FindSymbolAndReferences`, `SearchImplementations`
outputs — ensure `warnings[]`, `best_effort`, and `expansion` diagnostics are never
stripped by `response_mode=compact` or economy profile. Add regression test if any
path omits warnings when expansion partially fails.

**Rationale**: **L3** — merged steps compound heuristic uncertainty; economy mode
must not hide uncertainty to save tokens (L2 would treat silence as certainty).

---

## Resolved unknowns

| Unknown | Resolution |
|---------|------------|
| Response detail rename? | Prose only in v1 (D2) |
| Manifest URI | `opengrok://capabilities` (D4) |
| Default profile when unset | `economy` (D1) |
| Trajectory judge | Deterministic only v1 (D7) |
| SC-001 threshold | ≥15%; validate in gate-results |

No remaining `[NEEDS CLARIFICATION]` blockers for Phase 1 design.
