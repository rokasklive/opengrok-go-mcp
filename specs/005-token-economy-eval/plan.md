# Implementation Plan: Token Economy Eval

**Branch**: `005-token-economy-eval` | **Date**: 2026-06-11 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-token-economy-eval/spec.md`

**Note**: Filled by `/speckit-plan`. Extends feature 004 eval harness with deterministic
scenario replay and MCP byte-cost reporting across full, compact, and gateway surfaces.

## Summary

Add a **token economy benchmark** to package `evals/` that reuses the hermetic stdio subprocess
path from 004, replays **surface-agnostic scenarios** (canonical operations + args) through
**per-surface adapters**, measures UTF-8 bytes at MCP boundaries (`ListTools`, gateway discover,
per-call request/response with text vs structured split), and writes **`token_report.json`**
and **`token_report.md`**. v1 runs four scenario types across three surfaces with gateway
cold/warm totals; **no byte threshold CI gate** — artifact-only.

No changes to MCP server tool contracts or behavior.

## Technical Context

**Language/Version**: Go 1.25

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.0 (existing);
`encoding/json` for byte measurement; existing `evals/backend.go` hermetic fake

**Storage**: Ephemeral benchmark state; `evals/token_report.json` / `evals/token_report.md`
on disk (gitignored); scenarios in `evals/testdata/scenarios/`; shared OpenGrok fixtures

**Testing**: `go test ./evals/ -run TestTokenBenchmark -count=1` (primary); `go test ./evals/`
includes contract eval; `go test ./...` for full module

**Target Platform**: Linux/macOS CI and local dev; stdio subprocess + loopback httptest

**Project Type**: Maintainer eval extension (same package as 004)

**Performance Goals**: Full benchmark (3 surfaces × 4 scenarios) completes in &lt;120s on CI
hardware; dominated by subprocess startup × 3

**Constraints**: No server code changes; no MCP contract changes; secrets only via harness
env; compact `files.list` step skipped with explicit reporting; `est_tokens` is bytes÷4
heuristic

**Scale/Scope**: 4 scenarios, 3 surfaces, ~8 canonical ops in adapter table; deterministic
replay only (no LLM)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **MCP Contract**: **No changes** to tool names, schemas, warnings, cursors, citations,
  capability gates, or surfaces. Benchmark **measures** existing surfaces via subprocess.
- **OpenGrok Semantics**: Scenarios use hermetic fixtures with stable paths/symbols; benchmark
  does not assert semantic exhaustiveness — byte metrics only.
- **Test Evidence**:
  - New: `TestTokenBenchmark` — proves byte ledger, three surfaces, report artifacts.
  - Existing: `TestEvalSuite` unchanged; both run under `go test ./evals/`.
  - Verification: `go test ./evals/ -run TestTokenBenchmark -count=1`; `go test ./...`.
  - **Test-first ordering**: Token counting + adapter unit tests before full benchmark test;
    scenario JSON after adapter smoke.
- **Agent UX Validation**: Not applicable (maintainer tooling). Indirectly supports agent UX
  by measuring surface/schema economics — no cold-agent task required.
- **Security**: Hermetic backend; no secrets in scenario JSON; reports contain metrics only.
- **Compatibility and Docs**: No operator/agent behavior change. Update `evals/README.md`,
  `AGENTS.md` testing bullet; optional CI artifact upload for `token_report.*`.
- **Experimental Surface**: Gateway measured but not promoted; experimental label unchanged.
- **Resource Bounds**: Bounded by scenario count and three subprocess starts; no new server
  response limits.

**Post-design re-check**: PASS — design artifacts describe maintainer measurement only;
server unchanged.

No constitution violations → **Complexity Tracking is empty**.

## Project Structure

### Documentation (this feature)

```text
specs/005-token-economy-eval/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── token-benchmark-contract.md
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
evals/
├── harness.go           # extend: StartOptions.ToolSurface
├── backend.go             # unchanged (shared)
├── transport.go         # unchanged
├── models.go            # extend or token_models.go: Scenario, SurfaceRun, TokenBenchmarkResult
├── scenarios.go         # load scenarios, validate ops
├── surface.go           # canonical op → tool/args per surface
├── tokens.go            # byte counting, ledger, cold/warm totals
├── token_report.go      # markdown + JSON writers
├── benchmark.go         # RunScenario, RunBenchmark
├── token_benchmark_test.go  # TestTokenBenchmark
├── evals_test.go        # TestEvalSuite (004) — unchanged entry
├── testdata/
│   ├── scenarios/       # NEW: four v1 scenarios
│   │   ├── symbol_investigation_granular.json
│   │   ├── text_search_and_read.json
│   │   ├── file_exploration.json
│   │   └── compound_symbol_investigation.json
│   ├── manifest.json    # shared
│   └── opengrok/        # shared
├── token_report.json    # generated (gitignore)
└── token_report.md      # generated (gitignore)
```

**Structure Decision**: Extend flat `evals/` package; prefix new files (`token_`, `scenario_`,
`surface`) or group in same files if small. Reuse 004 harness lifecycle — parameterize surface.

## Implementation Design

### Slice 1 — Models and byte counting

- `Scenario`, `ScenarioStep`, `SurfaceRun`, `TokenBenchmarkResult` types (see data-model.md).
- `tokens.go`: `countListTools`, `countSchemaByTool`, `countCallToolRequest`,
  `countCallToolResponse` (text + structured split).
- Unit tests: fixed JSON blobs → expected byte counts; cold/warm formula.

**Exit**: `go test ./evals/ -run TokenCount -count=1` green.

### Slice 2 — Surface adapters

- `surface.go`: `Resolve(surface, op, args) → (tool, arguments, skipped, reason)`.
- Table for full, compact, gateway per research R3.
- Unit tests: each canonical op maps correctly; `files.list` skipped on compact.

**Exit**: Adapter tests without subprocess.

### Slice 3 — Scenario loader

- `scenarios.go`: load `testdata/scenarios/*.json`, validate unique ids, non-empty steps.
- Four v1 scenario files with hermetic-aligned args.

**Exit**: Loader test loads four scenarios.

### Slice 4 — Harness surface parameter

- Extend `Harness.Start` with `Options{ToolSurface string}`.
- Set `OPENGROK_MCP_TOOL_SURFACE` in subprocess env.
- Contract eval `TestMain` uses default `full` (unchanged behavior).

**Exit**: Manual smoke — `ListTools` differs between full and compact subprocess.

### Slice 5 — Benchmark runner

- `benchmark.go`: for each surface → start harness → record bootstrap bytes → for each
  scenario → replay steps → aggregate `SurfaceRun`.
- Gateway: record `discover_bytes` on cold path; warm totals exclude discover.
- Optional per-step `no_error` log only (no fail).

**Exit**: Single scenario × single surface produces `SurfaceRun` struct.

### Slice 6 — Reports

- `token_report.go`: write JSON + markdown tables (scenario × surface, metrics columns from
  contract); top offenders; cold/warm notes; `est_tokens` heuristic label.

**Exit**: Files written after partial benchmark run.

### Slice 7 — TestTokenBenchmark + polish

- `token_benchmark_test.go`: full hermetic run; assert reports exist; **do not** assert byte
  ceilings.
- `.gitignore`: `evals/token_report.*`
- `evals/README.md`: token benchmark section
- `AGENTS.md`: testing bullet for `TestTokenBenchmark`
- Optional: CI workflow artifact upload for `token_report.*`

**Exit**: `go test ./evals/ -run TestTokenBenchmark -count=1` green; no orphan processes.

## Test Plan

| ID | Slice | Command / area | Proves |
|----|-------|----------------|--------|
| T1 | 1 | `tokens` unit tests | Byte counting rules |
| T2 | 2 | `surface` unit tests | Adapter mapping + compact skip |
| T3 | 3 | `scenarios` loader test | Four scenarios load |
| T4 | 4 | Harness surface smoke | Different ListTools per surface |
| T5 | 5 | Single scenario integration | SurfaceRun populated |
| T6 | 6 | Report files | FR-009 artifacts |
| T7 | 7 | `TestTokenBenchmark` | SC-001, SC-006 end-to-end |
| T8 | 7 | `pgrep` after benchmark | No subprocess leak |
| T9 | All | `go test ./evals/ -count=1` | Contract + token tests coexist |
| T10 | All | `go test ./...` | Module regression |

## Agent UX Validation Plan

**Deferred** — maintainer benchmark. Future: correlate byte report with ergonomics review
findings when changing defaults (`response_mode`, `include_links`).

## Complexity Tracking

> Empty — no constitution violations.

## Quickstart Reference

See [quickstart.md](./quickstart.md).

## Contracts Reference

See [contracts/token-benchmark-contract.md](./contracts/token-benchmark-contract.md).

## Research Reference

See [research.md](./research.md) — all Technical Context items resolved.

## Next Step

Run **`/speckit-tasks`** to generate dependency-ordered `tasks.md`.
