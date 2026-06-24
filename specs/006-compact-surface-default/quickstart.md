# Quickstart & Go/No-Go Gate: Compact Surface as Default

**Feature**: 006-compact-surface-default | **Date**: 2026-06-24

How to verify this feature, and the gate that authorizes flipping the default.
The checklist follows the **review-checklists** discipline: every item traces to a
spec FR/contract, carries an **evidence standard** (Documented < Tested <
Monitored), and the gate is **PASS / CONDITIONAL-PASS / FAIL**. Behavioral items
require *Tested* evidence — a doc claim does not satisfy them (L2).

## Verification commands

```bash
# Build + full verification (Constitution III)
go test ./...

# Targeted: compact wrappers and registration/schemas
go test ./internal/mcpserver/ -run 'Compact|Register' -count=1

# Eval harness: contract suite (now parameterized full + compact) + token economy
go test ./evals/ -count=1
go test ./evals/ -run TestTokenBenchmark -count=1

# Refresh reports + committed baselines (incl. new compact baseline)
./scripts/update-eval-results.sh

# Format
gofmt -w <changed .go files>
git diff --check
```

## Manual real-instance check (FR-017)

Against the real instance `https://opengrok.home/` (no auth, per
`specs/002-minimal-setup-surface/quickstart.md`):

```bash
export OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1
# default surface is now compact (no OPENGROK_MCP_TOOL_SURFACE set)
# Drive a cold-agent journey: projects.list -> search.code -> read.context ->
# symbols.find -> projects.overview, confirming citations + pagination round-trip.
```

## Fresh-subagent UX validation (Constitution I — required gate)

Dispatch a fresh lightweight/mid-tier subagent (or fresh-session simulation) with a
**realistic task and minimal context** — e.g. *"Using only this MCP server, find
where `PaymentProcessor` is defined, read the surrounding code, and tell me what
languages the project uses. Cite sources."* Capture first-use findings on: tool
selection (did it pick one obvious tool per step?), schema legibility (did it build
calls from schemas without guessing?), descriptions, warnings, and errors. This is
a usability probe, **not** an LLM-as-judge metric (deterministic graders own
scoring; research D5).

## Go/No-Go gate

> **Scope-limitation statement**: This is a contract-compliance + agent-ergonomics
> verification, not a blanket system-performance guarantee, and it does not certify
> that the tests it relies on are themselves complete. A PASS means the verified
> contracts and the equivalence/parity invariants hold on the eval corpus and the
> fresh-subagent probe — not that every real-world query is covered.

| # | Gate item | Traces to | Evidence standard | How verified |
|---|---|---|---|---|
| G1 | Compact exposes 3–4 tools (no memory tool), zero overlapping tools/ops | FR-001/002, SC-002 | Tested | registration test asserts tool/op set |
| G2 | Vague `opengrok_compound` removed; names self-describe | FR-003 | Tested | registration test; no `compound` tool |
| G3 | Every operation's required fields/enums discoverable from schema (no prose-only fields) | FR-004, SC-004 | Tested | schema-introspection test + schema-only CUJ |
| G4 | Operation descriptions at ≥ full depth (purpose/when/gotchas) | FR-005 | Documented | description review vs full |
| G5 | Invalid op + missing field → actionable errors | FR-006 | Tested | negative eval cases |
| G6 | Capability parity incl. `projects.files` + `.overview`; nothing full-only | FR-007/008, SC-003 | Tested | migration-map coverage + eval cases |
| G7 | Cursors/`total_*`/truncation/warnings/`citation.url` preserved | FR-012 | Tested | cross-surface equivalence assertion |
| G8 | Capability gating honored at **tool + operation** level (a tool with no available ops is not registered → not in ListTools) | FR-013, SC-013 | Tested | gated-registration test |
| G9 | Compact registers **no** memory tool (memory full-only, pending sunset) | FR-014 | Tested | registration test asserts no memory tool |
| G10 | All six response states distinguishable per tool | FR-015, SC-009 | Tested | response-state eval coverage |
| G11 | Contract suite runs on compact (none full-only) + compact-specific cases exist | FR-019/020, SC-010 | Tested | `go test ./evals/` surface coverage |
| G12 | Cross-surface equivalence holds on shared scenarios | FR-021, SC-011 | Tested | equivalence assertion green |
| G13 | Compact baseline committed; CI fails on compact contract regression | FR-022, SC-012 | Monitored (CI) | `evals/baselines/` + CI config |
| G14 | Default = compact; `full` selectable + byte-for-byte stable | FR-009/010, SC-007 | Tested | config test both surfaces |
| G15 | Migration note + map present; restore path documented | FR-011 | Documented | migration-map + README/docs |
| G16 | Experimental framing removed from compact; gateway still labeled | FR-016 | Documented | docs/description review |
| G17 | Fresh-subagent first-use probe shows correct first-try tool selection | Constitution I, SC-001 | Tested | subagent findings captured |
| G18 | Agent-ergonomics review: no Critical findings; T&I + Economic ≥ full | SC-006, US5 | Tested | inspector review recorded |

**Gate rule**: All behavioral items (G1–G14, G17) must be PASS on *Tested*
evidence before the default flip (G14) is allowed to land — the flip is sequenced
last (research D4). G13/G18 may be CONDITIONAL-PASS with a tracked action item only
if explicitly risk-accepted and recorded. Any FAIL on G1–G12 blocks the flip.

## Definition of done (Constitution completion gate)

- `go test ./...` green; eval suite green on full + compact; equivalence assertion
  green; compact baseline committed.
- README, `docs/configuration.md`, `docs/tool-contracts.md`, `docs/limitations.md`,
  `docs/agent-usage-patterns.md`, `evals/README.md`, `CHANGELOG.md`, and the
  migration note updated (spec Documentation Impact).
- Fresh-subagent usability findings recorded; agent-ergonomics review attached.
- No unexplained constitution gate violations; default-flip break recorded in
  plan.md Complexity Tracking.
