# Research: Token Economy Eval

**Feature**: `005-token-economy-eval` | **Date**: 2026-06-11

## R1 — Extend `evals/` rather than new package

**Decision**: Add token benchmark code to existing top-level `evals/` package alongside
contract eval (004).

**Rationale**:
- Reuses `harness.go`, `backend.go`, `transport.go`, hermetic fixtures, and binary build path.
- Same CI entrypoint family (`go test ./evals/`).
- Spec FR-001 and dependency on feature 004.

**Alternatives considered**:
- `evals/benchmark/` subpackage — acceptable if `evals` grows large; v1 keeps flat files with
  clear prefixes (`scenario_`, `token_`, `surface_`).
- Separate `cmd/token-benchmark` — rejected; duplicates subprocess lifecycle.

---

## R2 — Byte measurement at MCP boundaries

**Decision**: Count UTF-8 byte length of JSON-serialized MCP-visible payloads. Report raw
bytes plus `est_tokens = bytes / 4` labeled heuristic in markdown.

**Rationale**:
- Stable across CI runs (no model tokenizer dependency).
- Aligns with agent ergonomics practice of tracking tool-output dominance.
- `StructuredContent` and `TextContent` are already parsed in `assert.go`.

**Measurement rules**:
| Metric | Source |
|--------|--------|
| `list_tools_bytes` | `json.Marshal` of full `ListTools` tools array |
| `schema_bytes_by_tool` | Per tool: `json.Marshal` of `{name, description, inputSchema, outputSchema}` only |
| `discover_bytes` | Gateway: `CallTool(opengrok_discover)` response bytes (structured + text) |
| `request_bytes` | Per step: `json.Marshal({name, arguments})` |
| `response_structured_bytes` | `json.Marshal(result.StructuredContent)` if non-nil |
| `response_text_bytes` | Sum of `TextContent.Text` UTF-8 lengths |
| `response_bytes` | `response_structured_bytes + response_text_bytes` per step (both may be present) |

**Alternatives considered**:
- `tiktoken` / model-specific counting — deferred; adds dep and model coupling.
- Character÷4 only without byte fields — rejected; bytes are the CI diff unit.

---

## R3 — Scenario model: canonical operations + surface adapters

**Decision**: Scenarios JSON lists ordered steps with `op` (canonical id) + `args`. Go
`SurfaceAdapter` maps `(surface, op, args) → (toolName, arguments)` or marks step skipped.

**Rationale**:
- Spec FR-002/FR-003: surface-agnostic scenarios, tool mapping in code.
- Adapters can encode compact `operation`/`payload` and gateway `operation`/`payload` shapes.
- New scenarios via JSON; new canonical ops require adapter table update.

**Canonical op registry (v1)**:

| Canonical `op` | Full | Compact | Gateway |
|----------------|------|---------|---------|
| `search.definitions` | `search_symbol_definitions` | `opengrok_search` / definitions | `search.definitions` |
| `search.references` | `search_symbol_references` | `opengrok_search` / references | `search.references` |
| `search.code` | `search_code` | `opengrok_search` / code | `search.code` |
| `path.search` | `search_code` `mode=path` | `opengrok_search` code + path mode | `search.code` |
| `read.file` | `read_file` | `opengrok_read` / file | `file.read` |
| `files.list` | `list_files` | **skip** | `files.list` |
| `compound.find_symbol` | `find_symbol_and_references` | `opengrok_compound` | `compound.find_symbol_and_references` |
| `compound.search_and_read` | `search_and_read` | `opengrok_compound` | `compound.search_and_read` |

**Alternatives considered**:
- Per-surface scenario files — rejected; duplicates logical work and breaks comparison.
- LLM replay — deferred (spec out of scope v1).

---

## R4 — Gateway cold vs warm

**Decision**:
- **Cold row**: `list_tools_bytes + discover_bytes + Σ(request + response)` for all steps.
- **Warm row**: `list_tools_bytes + Σ(request + response)` — `discover_bytes` excluded.
- **Full/compact**: cold = warm (no discover; no cross-scenario amortization in v1).
- Warm gateway: run `opengrok_discover` once per harness session before scenarios; still
  record `discover_bytes` on cold rows only.

**Rationale**: Matches spec FR-007 and design review — separates first-use gateway penalty
from steady-state.

**Alternatives considered**:
- Amortize `list_tools` across scenarios — deferred to later session mode.

---

## R5 — v1 scenario corpus (four types)

**Decision**: Four JSON scenarios in `evals/testdata/scenarios/`:

1. **`symbol-investigation-granular`** — `search.definitions` → `read.file` → `search.references`
   (fixture: `PaymentProcessor`, `platform`, `src/PaymentProcessor.java`).
2. **`text-search-and-read`** — `search.code` (`Engine`) → `read.file` (`src/Engine.swift`).
3. **`file-exploration`** — `files.list` (`src`) → `path.search` (`Engine.swift`) → `read.file`;
   compact skips `files.list` with explicit skip metadata.
4. **`compound-symbol-investigation`** — single `compound.find_symbol` (`PaymentProcessor`).

Args use real tool fields only (`project`, `symbol`, `file_path`, `path`, `mode`); optional
`response_mode`, `include_links` for economy variants in future scenario versions.

**Alternatives considered**:
- `project.overview` in file exploration — rejected for compact gap; `files.list` + path
  search stresses navigation without symbol duplication.

---

## R6 — Test entrypoint and CI behavior

**Decision**: `TestTokenBenchmark` in `evals/benchmark_test.go` (or `token_benchmark_test.go`);
does **not** fail on byte thresholds in v1. Writes `evals/token_report.json` and
`evals/token_report.md`. Contract eval `TestEvalSuite` remains separate and gateable.

**Rationale**: Spec FR-010, SC-006 — artifact-only until baselines exist.

**CI**: Existing `go test ./...` runs benchmark; upload `token_report.*` as artifact when
workflow supports it (polish task). Add `evals/token_report.*` to `.gitignore`.

**Alternatives considered**:
- Single merged test — rejected; contract failures should not be masked by token metrics.

---

## R7 — Top-offender fields at report time

**Decision**: Derive without extra runtime state beyond per-step ledger:
- `largest_tool_schema_name` / `largest_tool_schema_bytes` — max of `schema_bytes_by_tool`
- `largest_response_bytes` — max step `response_bytes`
- `largest_response_step` — step index + canonical `op`

**Rationale**: Spec FR-008; makes markdown actionable without reading JSON dumps.

---

## R8 — Harness surface parameterization

**Decision**: Extend `Harness.Start` (or `StartWithOptions`) with `ToolSurface` env
`OPENGROK_MCP_TOOL_SURFACE=full|compact|gateway`. Token benchmark starts one harness per
surface (three subprocess lifecycles per full benchmark run).

**Rationale**:
- Surfaces register different tool sets; clean `ListTools` per surface.
- Avoids restarting server mid-session for surface switch.

**Alternatives considered**:
- One process, env reload — rejected; surface is fixed at server startup.
