# Implementation Plan: MCP Eval Harness

**Branch**: `004-mcp-eval-harness` | **Date**: 2026-06-10 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-mcp-eval-harness/spec.md`

**Note**: Filled by `/speckit-plan`. See `.specify/templates/plan-template.md` for workflow.

## Summary

Add a **dataset-driven stdio subprocess eval harness** in top-level package `evals/` that
builds `opengrok-go-mcp`, starts a **manifest-driven httptest OpenGrok fake**, spawns the
server once per suite, runs JSON eval cases via `mcp.CommandTransport`, scores structured
outputs (direct-call mode), and writes **markdown + JSON reports** with optional baseline
deltas. No changes to MCP tool contracts or server behavior — this is a fourth test layer
complementing in-memory handler tests.

## Technical Context

**Language/Version**: Go 1.25

**Primary Dependencies**: `github.com/modelcontextprotocol/go-sdk/mcp` v1.4.0 (`CommandTransport`,
`ClientSession.CallTool`); `net/http/httptest` for hermetic backend; existing `cmd/opengrok-go-mcp`
binary

**Storage**: Ephemeral in-memory suite state; optional `evals/report.json` / `evals/report.md`
on disk; testdata JSON + OpenGrok fixtures under `evals/testdata/`

**Testing**: `go test ./evals/ -run TestEvalSuite -count=1` (primary); `go test ./...` for
full module; harness does not replace `internal/mcpserver/*_test.go`

**Target Platform**: Linux/macOS CI and local dev; stdio subprocess + loopback httptest

**Project Type**: Go CLI/MCP server + maintainer eval package

**Performance Goals**: Suite completes in &lt;60s on CI hardware (SC-003); per-case latency
budgets via optional `latency_ms` checks; aggregate p50/p95 in report

**Constraints**: Preserve MCP schema compatibility — **no server code changes** for contract;
capability-gated tools → skipped cases; secrets only via subprocess env (never in case JSON);
full surface only in v1

**Scale/Scope**: Seed corpus: 2 projects (`platform`, `infra`), ~10–15 cases covering
`list_projects`, `search_code`, `read_file`/`get_file_context`, symbol tool; expandable via
testdata only

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **MCP Contract**: **No changes** to tool names, schemas, warnings, cursors, citations,
  capability gates, or surfaces. Harness **validates** existing contract via subprocess calls.
- **OpenGrok Semantics**: Hermetic fixtures mirror REST shapes from `internal/opengrok/client.go`;
  cases assert MCP-layer fields (`results`, `citation.url`, `total_hits`, etc.) — same
  best-effort semantics as production, not new semantics.
- **Test Evidence**:
  - New: `evals/evals_test.go` (`TestMain` + `TestEvalSuite`) — proves binary + stdio + env +
    capability gating + structured output checks.
  - Existing layers unchanged: `internal/mcpserver/*_test.go`, `connectMCPServer` in-memory,
    `cmd/.../main_test.go`.
  - Verification: `go test ./evals/ -count=1`; `go test ./...`.
  - **Test-first ordering**: Implement harness skeleton + hermetic backend before seed cases;
    add cases incrementally per tool (list → search → read → symbols).
- **Agent UX Validation**: Not applicable to harness feature (maintainer tooling). Optional
  post-implementation: run suite and confirm report readability for PR review. No cold-agent
  task required for v1.
- **Security**: Hermetic backend on loopback; `OPENGROK_MCP_CURSOR_SECRET` test-only; no tokens
  in testdata; reports truncate failure payloads; live-eval path (if added) uses env vars only.
- **Compatibility and Docs**: No operator/agent behavior change. Update `AGENTS.md` testing
  section to mention `go test ./evals/`; optional `docs/` note in polish task. README optional
  one-liner under development/testing.
- **Experimental Surface**: None.
- **Resource Bounds**: Harness bounded by case count and fixture corpus size; no new server
  response limits.

**Post-design re-check**: PASS — design artifacts describe maintainer contract only; server
unchanged.

No constitution violations → **Complexity Tracking is empty**.

## Project Structure

### Documentation (this feature)

```text
specs/004-mcp-eval-harness/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/
│   └── eval-harness-contract.md
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
evals/
├── models.go            # EvalCase, EvalResult, SuiteResult, check types
├── harness.go           # buildBinary, startBackend, spawn subprocess, ListTools
├── runner.go            # loadCases, RunCase, RunSuite
├── assert.go            # result_checks evaluation (dotted paths)
├── metrics.go           # aggregate score, Coverage@K, latency percentiles
├── report.go            # report.md + report.json + baseline delta
├── backend.go           # manifest-driven httptest fake (from test_data_pack)
├── evals_test.go        # TestMain lifecycle + TestEvalSuite
├── testdata/
│   ├── manifest.json
│   ├── list_projects.json
│   ├── search_code.json
│   ├── read_file.json          # or get_file_context.json
│   ├── search_symbols.json     # definitions or list_symbols
│   └── opengrok/
│       ├── projects_indexed.json
│       ├── search_*.json
│       ├── file_content_*.json
│       └── list_*.json
├── report.md            # generated (gitignored or committed — decide in implement)
└── report.json          # generated

cmd/opengrok-go-mcp/     # unchanged — spawned binary
internal/mcpserver/      # unchanged
.agents/skills/mcp-eval-harness/test_data_pack/  # seed source (not runtime dependency)
```

**Structure Decision**: New top-level `evals/` package (not `internal/`) per spec FR-010;
imports MCP SDK only; spawns `cmd/opengrok-go-mcp` via `go build` or `go test` binary path.

## Implementation Design

### Slice 1 — Models and testdata seed

- `models.go`: `EvalCase`, `Expected`, `ResultCheck`, `EvalResult`, `SuiteResult`.
- Copy `test_data_pack/evalcases/*.json` → `evals/testdata/`.
- Copy `test_data_pack/opengrok/` + `manifest.json` → `evals/testdata/`.
- Loader: read all `testdata/*.json` (exclude `opengrok/` dir), validate unique `id`.

**Exit**: Loader unit test or compile-only; fixtures on disk.

### Slice 2 — Hermetic backend

- `backend.go`: manifest router from skill `backend-strategies.md` / test_data_pack README.
- `startBackend()` returns `httptest.Server` URL + teardown.
- Env builder: `OPENGROK_MCP_BASE_URL`, `WEB_BASE_URL`, `PROJECTS`, `DEFAULT_PROJECT`,
  `PROBE_FILE`, `CURSOR_SECRET`.

**Exit**: Manual or small test — fake answers `/projects/indexed` and `/search?full=test`.

### Slice 3 — Harness (binary + subprocess)

- `harness.go`: `buildBinary(ctx)` → temp `opengrok-go-mcp`.
- `CommandTransport` + `exec.Command` with env from backend.
- `ListTools` once → `registeredTools` set.
- Cleanup: `session.Close()`, `server.Close()`, process wait.

**Exit**: Subprocess stays up against fake; `ListTools` returns search tools.

### Slice 4 — Runner and asserts

- `runner.go`: `RunSuite` loops cases; skip if tool missing.
- `assert.go`: `no_error`, `has_results`, `field_present`, `latency_ms`; dotted path resolver
  for arrays (first element).
- Parse `StructuredContent` or JSON from `TextContent`.

**Exit**: Single hardcoded case passes end-to-end.

### Slice 5 — Metrics and reports

- `metrics.go`: judged pass/fail/skip, mean score, Coverage@K, per-tool scores, latency p50/p95.
- `report.go`: write `evals/report.md` and `evals/report.json`; read optional baseline for Δ.

**Exit**: Reports written after suite; markdown lists failed case IDs.

### Slice 6 — Full corpus and TestEvalSuite

- `evals_test.go`: `TestMain` — build → backend → harness → run all cases → assert
  `failed == 0` for judged cases (or threshold if spec requires).
- Wire all seed cases; tune latency budgets if flaky.

**Exit**: `go test ./evals/ -count=1` green; `pgrep` empty after run.

### Slice 7 — Polish (optional in v1)

- `.gitignore` for `evals/report.*` if not committed.
- `AGENTS.md` testing bullet for `./evals/`.
- Optional `TestEvalSuiteLive` behind env guard.
- Future CI workflow when repo adds GitHub Actions.

## Test Plan

| ID | Slice | Command / area | Proves |
|----|-------|----------------|--------|
| E1 | 2 | `httptest` handler test or integration | Fake routes match manifest |
| E2 | 3 | Harness smoke | Subprocess + ListTools |
| E3 | 4 | `list_projects` case | FR-012 minimum tool |
| E4 | 4 | `search_code` case | Structured results + citations |
| E5 | 4 | `read_file` / `get_file_context` | File read path |
| E6 | 4 | Symbol case | Definition/list symbols |
| E7 | 4 | Gated tool case | Skip not fail (FR-005) |
| E8 | 5 | Report files exist | FR-008, FR-009 |
| E9 | 6 | `go test ./evals/ -count=1` | SC-001 end-to-end |
| E10 | 6 | `pgrep -f opengrok-go-mcp` | SC-002 no orphans |
| E11 | All | `go test ./...` | Module regression |

## Agent UX Validation Plan

**Deferred** — harness is maintainer-facing. If report confuses reviewers, improve markdown
template in Slice 5.

## Complexity Tracking

> Empty — no constitution violations.

## Quickstart Reference

See [quickstart.md](./quickstart.md).

## Contracts Reference

See [contracts/eval-harness-contract.md](./contracts/eval-harness-contract.md).

## Research Reference

See [research.md](./research.md) — all Technical Context items resolved.

## Next Step

Run **`/speckit-tasks`** to generate dependency-ordered `tasks.md`.
