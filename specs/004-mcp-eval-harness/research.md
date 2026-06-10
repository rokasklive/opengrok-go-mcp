# Research: MCP Eval Harness

**Feature**: `004-mcp-eval-harness` | **Date**: 2026-06-10

## R1 — Package location and module boundary

**Decision**: New top-level Go package `evals/` at repository root (`github.com/rokasklive/opengrok-go-mcp/evals`).

**Rationale**:
- Spec FR-010 and assumptions require `go test ./evals/` as the entrypoint.
- Keeps subprocess + httptest lifecycle out of `internal/mcpserver` (handler tests stay in-process).
- `evals` imports `mcp` SDK and spawns `cmd/opengrok-go-mcp` binary — not a library for agents.

**Alternatives considered**:
- `internal/evals` — rejected; subprocess test is maintainer/CI tooling, not server internals.
- `cmd/opengrok-go-mcp/evals_test` — rejected; mixes binary entry with suite harness.

---

## R2 — Stdio transport via go-sdk CommandTransport

**Decision**: Use `mcp.CommandTransport` with `exec.Command` on the built binary; connect with `mcp.NewClient().Connect(ctx, transport, nil)`.

**Rationale**:
- Matches production stdio MCP clients (JSON-RPC framing, env config, process lifecycle).
- SDK v1.4.0: `session.Close()` closes stdin, waits, SIGTERMs child — mirror `testutil_test.go` cleanup discipline.
- In-memory transports (`NewInMemoryTransports`) remain in `internal/mcpserver` — not duplicated.

**Alternatives considered**:
- Raw JSON-RPC over pipes without SDK — rejected; more fragile than `CallTool` / `ListTools`.
- HTTP transport for eval — rejected; default agent path is stdio; out of scope for v1.

---

## R3 — Hermetic OpenGrok backend (mandatory for subprocess start)

**Decision**: Manifest-driven `httptest.Server` loading fixtures from `evals/testdata/opengrok/` + `manifest.json`, adapted from `.agents/skills/mcp-eval-harness/test_data_pack/`.

**Rationale**:
- `cmd/opengrok-go-mcp` exits if no search capability is reachable at startup.
- Manifest fake answers `/projects/indexed`, `/search`, `/file/content`, `/list` with shapes matching `internal/opengrok/client.go`.
- Ephemeral `httptest` port avoids CI collisions.
- Env wiring: `OPENGROK_MCP_BASE_URL`, `OPENGROK_MCP_WEB_BASE_URL`, `OPENGROK_MCP_PROJECTS`, `OPENGROK_MCP_DEFAULT_PROJECT`, `OPENGROK_MCP_PROBE_FILE`, `OPENGROK_MCP_CURSOR_SECRET` (see backend-strategies.md).

**Alternatives considered**:
- Hand-coded handlers only — rejected; test_data_pack already provides coherent corpus + routing.
- Live OpenGrok for default CI — rejected; flaky, auth-dependent, index drift.

---

## R4 — Evaluation mode for v1

**Decision**: **Direct-call only** in v1. Cases name `tool` + `input`; harness calls that tool directly.

**Rationale**:
- Spec FR-006 and skill guidance: MRR, tool accuracy, confusion matrix are meaningless (constant) in direct-call mode.
- Report headline metrics: per-check pass rate, per-tool score, Coverage@K, latency p50/p95.
- Reserve selection-mode fields in `SuiteResult` JSON but print `Tool-selection metrics: n/a (direct-call mode)`.

**Alternatives considered**:
- Dual-mode v1 — rejected; scope creep; selection mode is a follow-up feature.

---

## R5 — Capability gating semantics

**Decision**: After subprocess start, call `ListTools` once; if case `tool` not in live tool set → `Skipped: true`, not failed.

**Rationale**:
- Server registers tools only when startup capability probes succeed (matches `register_full.go`).
- Skipped cases feed Coverage@K = judged/total; environment fact, not regression.
- Aligns with spec FR-005 and skill design rules.

**Alternatives considered**:
- Fail skipped cases — rejected; causes false reds when `list_files` gated but search enabled.

---

## R6 — Suite lifecycle (once per run)

**Decision**: `TestMain` builds binary → `startBackend` → spawn subprocess → `ListTools` → run all cases → teardown. Single shared `*mcp.ClientSession`.

**Rationale**:
- Startup + capability probes dominate cost; per-case spawn would distort latency percentiles.
- Skill and spec FR-002 require once-per-suite boot.

**Alternatives considered**:
- Per-case subprocess — rejected for latency and flakiness.

---

## R7 — Testdata layout and seed corpus

**Decision**:
- `evals/testdata/*.json` — MCP eval cases (one file per tool family).
- `evals/testdata/opengrok/*.json` + `manifest.json` — REST fixtures.
- Copy seed from `test_data_pack/evalcases/` and `test_data_pack/opengrok/`.

**Minimum v1 tools** (FR-012): `list_projects`, `search_code`, `read_file` or `get_file_context`, `search_symbol_definitions` or `list_symbols`.

**Alternatives considered**:
- Single monolithic cases.json — rejected; one-file-per-tool for reviewability.

---

## R8 — Reports and delta tracking

**Decision**: Write `evals/report.md` and `evals/report.json` after each suite run; optional read of prior `report.json` (or `report.baseline.json`) for per-tool pass-rate and latency deltas.

**Rationale**: Spec FR-008, FR-009, US3. JSON artifact is machine-diffable; markdown is PR-review friendly.

**Alternatives considered**:
- stdout-only — rejected; no baseline for deltas.

---

## R9 — CI integration

**Decision**: v1 gates on `go test ./evals/ -count=1` locally and in any future CI workflow; add `go test ./evals/` to full verification docs (`AGENTS.md` testing section) but do not require separate workflow file in this feature unless repo already has test.yml pattern.

**Rationale**: No `.github/workflows` in repo today; harness must pass via `go test ./...` when evals is included in module.

**Alternatives considered**:
- Mandatory new GitHub Actions job in v1 — optional polish task; not blocking harness value.
