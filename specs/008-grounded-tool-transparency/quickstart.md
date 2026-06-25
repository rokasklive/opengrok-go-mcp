# Quickstart: Working on Grounded Tool Transparency

Audience: a developer (or agent) implementing or extending feature 008.

## The one rule

**Descriptions and tests both come from the claim registry.** You never edit a tool
description's syntax/limitation prose by hand and you never write a conformance test that
isn't bound to a `claim_id`. Add a row to the registry; the description and the test matrix
follow. The bijection check fails the build if you break this.

## Add or change a capability claim

1. Add/edit a `Claim` in `internal/mcpserver/claims.go` (fields per
   `data-model.md#entity-1`). Set `support_status`, `agent_claim_text`, `example`,
   `ground_truth_source` (the `help.jsp` section — see `evals/testdata/help.snapshot`),
   `positive_assertion`, `negative_control`, `gate`.
2. Bind the conformance/contract check (`conformance_test_ref`):
   - `gate: live` → add a case in `evals/conformance_test.go` keyed by `claim_id`.
   - `gate: always-on` → add a contract test in `internal/mcpserver` keyed by `claim_id`.
   - `gate: none` (limitation only) → add the justification; it's bijection-exempt.
3. `go test ./internal/mcpserver/ -run Bijection` — fails if claim↔test is orphaned.
4. The description renders automatically; verify with the ListTools snapshot test.

## Run the suites

```sh
go test ./internal/mcpserver/ ./internal/config/   # errors, schema (no-slimming), bijection, regression locks
go test ./...                                       # full verification (Principle III gate)
go test ./evals/ -count=1                            # contract + token-economy (cost-per-successful-task)
OPENGROK_MCP_LIVE_EVAL=1 OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1 \
  go test ./evals/ -run Conformance -count=1         # behavioral conformance vs live OpenGrok
```

## Verify the agent-facing changes by hand

- Diagnostics off by default: a search response has no `diagnostics` block; set
  `OPENGROK_MCP_DIAGNOSTICS=true` and confirm it returns.
- Errors are specific: call `opengrok_read` with `operation=read` (wrong tool) → expect
  `UNKNOWN_OPERATION` naming the enabled ops, **not** `oneOf: did not validate`.
- No slimming: inspect ListTools on the compact surface — `mode`/`sort`/`context_budget`
  carry their legal-value descriptions, same as full.

## Definition of done (this feature)

Run the PR gate in `contracts/review-gate.md`. All G1–G6 items pass with the stated evidence
standard, migration notes are written, and a fresh-subagent first-use run (the audit
scenario) is recorded. A green gate means contracts met — not a performance guarantee.
