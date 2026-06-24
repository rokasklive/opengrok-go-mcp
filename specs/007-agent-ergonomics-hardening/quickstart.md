# Quickstart & Go/No-Go Gate: Agent Ergonomics Hardening

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24

Verification commands and release gate. Follows **review-checklists**: behavioral
items need *Tested* evidence (L2).

## Verification commands

```bash
# Full suite
go test ./...

# Targeted: profile, manifest, kind metadata, catalog
go test ./internal/config/ -run Profile -count=1
go test ./internal/mcpserver/ -run 'Profile|Capabilities|KindFilter|Catalog|Manifest' -count=1

# Eval: contract + trajectory + token ceiling
go test ./evals/ -count=1
go test ./evals/ -run TestTrajectorySuite -count=1
go test ./evals/ -run TestTokenBenchmark -count=1

# Economy vs rich token delta (SC-001)
OPENGROK_MCP_AGENT_PROFILE=economy go test ./evals/ -run TestTokenBenchmark -count=1

# Refresh baselines (after intentional schema growth)
./scripts/update-eval-results.sh

gofmt -w <changed .go files>
git diff --check
```

## Manual real-instance check

```bash
export OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1
export OPENGROK_MCP_AGENT_PROFILE=economy

# 1. Read manifest
#    resources/read opengrok://capabilities — tools match ListTools

# 2. Cold journey (compact default)
#    projects list → symbols definitions → read context
#    Confirm citations present, payloads leaner than rich profile

# 3. Partial gate simulation (if instance lacks references)
#    Confirm manifest gated[] explains missing references ops
```

## Fresh-subagent UX validation (Constitution I — gate G7)

Dispatch fresh subagent with **minimal context**:

> *"Read opengrok://capabilities, then find where `PaymentProcessor` is defined,
> read surrounding code, and cite the source. Use only listed operations."*

Capture: manifest used?, wasted calls on gated ops?, economy knobs discovered from
tool headers?, `total_hits` vs kind metadata understood on a list sweep?

## Go/No-Go gate

| # | Gate item | Traces to | Evidence | How verified |
|---|-----------|-----------|----------|--------------|
| G1 | `OPENGROK_MCP_AGENT_PROFILE` economy/rich bundles work; unset = economy | FR-001–003, D1 | Tested | config + helper tests |
| G2 | Per-call overrides beat profile | FR-002 | Tested | helper tests |
| G3 | `expand_context` schema text matches profile default | FR-004, D3 | Tested | schema registration test |
| G4 | `opengrok://capabilities` matches ListTools | FR-005–006, D4 | Tested | resource + gated fixture tests |
| G5 | Capability preamble in agent-usage-patterns | FR-007 | Documented | doc review |
| G6 | Kind-filter additive fields | FR-008, D5 | Tested | symbols + trajectory tests |
| G7 | Catalog metadata + UNKNOWN_PROJECT snapshot hint | FR-009–010, D6 | Tested | projects tests |
| G8 | Tool-header economy hints + surface/detail disambiguation | FR-011, D9 | Documented | description review |
| G9 | Trajectory suite ≥3 scenarios, ≥8 graders | FR-012, SC-004 | Tested | `TestTrajectorySuite` |
| G10 | Compact ListTools CI ceiling | FR-013, SC-005, D8 | Monitored | `TestTokenBenchmark` fail |
| G11 | Description CUJ grader | FR-014, D7 | Tested | trajectory test |
| G12 | Compound outputs keep warnings under economy | FR-015, D11 | Tested | compound regression test |
| G13 | SC-001: economy warm totals ≥15% below rich on 4 scenarios | SC-001 | Tested | token report in gate-results |
| G14 | Docs: configuration, tool-contracts, limitations, CHANGELOG | Constitution V | Documented | PR checklist |
| G15 | Fresh-subagent probe completed | Constitution I | Documented | gate-results.md notes |

**PASS**: G1–G12 green; G13 measured and recorded (may CONDITIONAL-PASS if 10–14%
with follow-up); G14–G15 complete before tag.

## gate-results.md

Create `specs/007-agent-ergonomics-hardening/gate-results.md` at implementation
end with SC-001 actual percentages, subagent findings, and PASS/CONDITIONAL/FAIL.
