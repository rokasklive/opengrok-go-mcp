# Implementation Plan: Grounded, Test-Backed Tool Transparency

**Branch**: `008-grounded-tool-transparency` | **Date**: 2026-06-25 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/008-grounded-tool-transparency/spec.md`

## Summary

Make the MCP interface state ground truth (L2): honest, explicit, bounded tool
descriptions that surface OpenGrok's full-text+ctags (non-AST) nature and the
supported/unsupported Lucene syntax with examples; specific, actionable errors that
replace the opaque `oneOf` message; and an opt-in diagnostics block. Every capability
claim is driven by a **single machine-checkable claim registry** that generates both the
description content and the conformance test matrix, so a claim cannot exist without a
backing behavioral test (verified against OpenGrok's `help.jsp`). Schema "slimming" is
removed (no surface hides documented behavior); the resulting one-time attention cost is
balanced by removing the recurring diagnostics waste and by progressive disclosure, and
is measured as cost-per-successful-task, not payload bytes.

Three agent-ergonomics skills shaped this plan: `agent-ergonomics-inspector` (root-law
framing), `evaluation-harness-designer` (eval validity — Pass^k, dual metric, behavioral
conformance, cost-per-successful-task), and `design-templates` (the registry / error /
description artifact shapes in `data-model.md` + `contracts/`). `review-checklists` is
applied to the review-gate artifact (`contracts/review-gate.md`).

## Technical Context

**Language/Version**: Go 1.24

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp`,
`github.com/google/jsonschema-go/jsonschema`, OpenGrok HTTP API + web (`help.jsp`)

**Storage**: In-memory process state; configuration via environment variables

**Testing**: `go test ./...`; targeted package tests; `go test ./evals/ -count=1`;
token-economy benchmark; live conformance gated on `OPENGROK_MCP_LIVE_EVAL=1` +
`OPENGROK_MCP_BASE_URL`

**Target Platform**: Local stdio MCP server and loopback HTTP transport

**Project Type**: Go CLI/MCP server

**Performance Goals**: Net cost-per-successful-task not regressed (ideally improved);
per-response payload reduced by diagnostics gating; description growth bounded/scannable

**Constraints**: Preserve MCP schema compatibility for the `full` surface; keep full,
compact, and gateway surfaces coherent (Principle I); preserve `citation.url`; capability
gating unchanged; secrets stay in env

**Scale/Scope**: ~4 compact tools + full surface; no multi-agent fan-out (scale curve N/A)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **MCP Contract**: Changes — (a) tool **descriptions** (all compact tools; full-surface
  descriptions stay coherent); (b) **error** contract: new `error_code`s
  (`UNKNOWN_OPERATION` un-shadowed, `MISSING_REQUIRED_FIELD`, `INVALID_FIELD_TYPE`,
  `UNKNOWN_FIELD`, `QUERY_PARSER_FAILED`) + a `suggestion` field on the error body; (c)
  **default response shape**: `diagnostics` omitted unless `OPENGROK_MCP_DIAGNOSTICS=true`;
  (d) **advertised schema**: compact field descriptions no longer slimmed; (e) new env var.
  No tool/operation names, cursors, citations, or pagination semantics change. All gated
  behaviors stay capability-gated.
- **OpenGrok Semantics**: This feature *implements* Principle II — descriptions state
  full-text+ctags vs AST explicitly, mark best-effort/page-local/conditional forms, and
  document regex as supported only via `/.../`. No new claim of semantic certainty.
- **Test Evidence**: Test-first per slice. New/changed tests: claim⇔test bijection check
  (always-on, fails build on orphan claim or orphan test); behavioral conformance per claim
  (live-gated, positive + negative control); error-taxonomy contract tests (one per class,
  asserting structured body not raw `-32602`); diagnostics on/off snapshot; no-slimming
  schema test; regression locks (`projects[]`, scalar coercion, default-project resolution).
  Targeted verification: `go test ./internal/mcpserver/ ./internal/config/` then
  `go test ./...` and `go test ./evals/ -count=1`.
- **Agent UX Validation**: Re-run the originating audit as a fresh-subagent first-use task
  (minimal context): "find subclasses of <Type> in <project> using only this MCP." Pass =
  the agent does not issue an AST/inheritance or bare-regex query, uses a documented
  approach or states the limitation, and still produces a source-grounded answer
  (deterministic trajectory grade + dual outcome metric).
- **Security**: One new env toggle (`OPENGROK_MCP_DIAGNOSTICS`, default off); no secrets,
  no new inbound exposure, no change to TLS/raw-fallback posture. Diagnostics gated off
  reduces incidental internal-detail leakage.
- **Compatibility and Docs**: Public-contract changes (b)(c)(d)(e) require migration notes.
  `full` surface stays behaviorally stable except for the shared honest-error contract and
  diagnostics gating (applied consistently across surfaces — a coherence requirement, not a
  divergence). Docs: `tool-contracts.md`, `limitations.md`, `agent-usage-patterns.md`,
  `configuration.md`, `agent-ux.md`, `review-checklist.md`, `README.md`, `CHANGELOG.md`.
- **Experimental Surface**: None new. Gateway stays experimental and must remain coherent
  with the corrected ground truth.
- **Resource Bounds**: Diagnostics gating reduces per-response size (recurring win).
  De-slimming increases per-session schema size (one-time, bounded — depth delegated to
  examples + `opengrok://capabilities`). Guarded by cost-per-successful-task (SC-006), not
  payload bytes. No new auto-fetch behavior.

**Gate result: PASS.** No unjustified violations; Complexity Tracking not required.

## Project Structure

### Documentation (this feature)

```text
specs/008-grounded-tool-transparency/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions R1–R7
├── data-model.md        # Phase 1 — the 3 artifact templates (claim/error/description) + entities
├── quickstart.md        # Phase 1 — how to add/verify a claim; how to run the suites
├── contracts/           # Phase 1 — claim-registry, error-taxonomy, description-contract, conformance-suite, review-gate
└── tasks.md             # Phase 2 (/speckit-tasks — not created here)
```

### Source Code (repository root)

```text
cmd/opengrok-go-mcp/main.go            # wire OPENGROK_MCP_DIAGNOSTICS into Config; startup log
internal/config/config.go              # parse OPENGROK_MCP_DIAGNOSTICS (ParseBool convention); Diagnostics bool
internal/mcpserver/
├── claims.go            (NEW)          # the claim registry: single source of truth (data)
├── claims_test.go       (NEW)          # claim⇔test bijection check (always-on)
├── compact_descriptions.go             # compose descriptions FROM claims.go; add nature/syntax/example/default
├── compact_schema.go                   # REMOVE slimSchema/slimSchemaInPlace; keep full field docs on compact
├── register_compact.go                 # stop calling the slimmer
├── types.go                            # keep field docs; gate Diagnostics (omitempty + populated only when enabled)
├── tool_errors.go                      # error taxonomy: new codes + suggestion field
├── validation.go        (NEW)          # pre-validation middleware: emit structured errors for the 4 classes
├── compact.go                          # un-shadow unknownOperationError
├── search_core.go / results.go         # populate diagnostics only when enabled
evals/
├── conformance_test.go  (NEW)          # live-gated behavioral conformance, driven by claims.go
├── regression_test.go   (NEW or in mcpserver)  # always-on locks (projects[], coercion, default resolution)
└── surface.go / evals_test.go          # extend; cost-per-successful-task reporting
docs/                                   # tool-contracts, limitations, agent-usage-patterns, configuration, agent-ux, review-checklist
README.md, CHANGELOG.md
```

**Structure Decision**: Extend existing `internal/mcpserver` and `internal/config`
packages; no new packages (Principle V — prefer existing boundaries). The only new files
are the claim registry, its bijection test, the pre-validation middleware, and the two eval
suites — each a single coherent responsibility.

## Build Order (safety-first, mirrors 006 discipline)

Ordered so the riskiest contract change (errors) is proven before the visible one
(descriptions), and the registry exists before anything consumes it:

1. **US5 regression locks first** — pin `projects[]`, scalar coercion, default-project
   resolution (always-on). Cheap, and they guard every later refactor (Principle III).
2. **Claim registry + bijection check (US2 core)** — `claims.go` + `claims_test.go`. The
   data backbone everything else reads. Bijection check is always-on.
3. **US3 errors** — pre-validation middleware + taxonomy + un-shadow; contract tests per
   class. (Highest-risk contract change; do it under test before touching descriptions.)
4. **US4 diagnostics gating + de-slimming** — env toggle; remove slimmer; snapshot tests.
5. **US1 descriptions** — compose from the registry (nature/syntax/example/default).
6. **US2 live conformance** — behavioral checks driven by the registry, live-gated.
7. **Docs + migration notes + cost-per-successful-task baseline + fresh-subagent UX run.**

The default-shape change (diagnostics) and advertised-schema change (de-slimming) flip only
after their tests + migration notes are in place — flipping a default before its guard is
green is the AP#3 silent-interface-change trap.

## Complexity Tracking

No constitution violations requiring justification. (Section intentionally empty.)
