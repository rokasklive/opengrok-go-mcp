# Data Model: Token Economy Eval

**Feature**: `005-token-economy-eval` | **Date**: 2026-06-11

Benchmark state is ephemeral per run. Optional `token_report.json` on disk for CI diffing.

## Scenario (testdata JSON)

| Field | Type | Required | Rules |
|-------|------|----------|-------|
| `id` | string | yes | Unique; report row key |
| `description` | string | yes | Human-readable |
| `steps` | array | yes | Min 1 step |
| `steps[].op` | string | yes | Canonical operation id (see registry in research.md) |
| `steps[].args` | object | yes | Operation arguments; must match real tool fields after adapter mapping |

Location: `evals/testdata/scenarios/*.json` — one scenario object per file or array of
scenarios per file (loader choice: array in each file, same as contract eval).

## ScenarioStep (runtime)

| Field | Meaning |
|-------|---------|
| `index` | 0-based step order |
| `op` | Canonical operation |
| `args` | From scenario JSON |
| `skipped` | true if adapter reports op unavailable on surface |
| `skip_reason` | e.g. `compact: files.list not available` |
| `tool` | Resolved MCP tool name (if executed) |
| `request_bytes` | Serialized CallTool name + arguments |
| `response_text_bytes` | Text content channel |
| `response_structured_bytes` | Structured content channel |
| `response_bytes` | Sum of text + structured for step |
| `error` | Transport or tool error string (optional smoke) |

## SurfaceRun (per scenario × surface)

| Field | Type | Meaning |
|-------|------|---------|
| `scenario_id` | string | Scenario id |
| `surface` | string | `full`, `compact`, or `gateway` |
| `list_tools_bytes` | int | Total ListTools payload |
| `schema_bytes_by_tool` | map[string]int | Tool name → schema bytes (ListTools only) |
| `discover_bytes` | int | Gateway discover; 0 for non-gateway |
| `request_bytes` | int | Sum of step request bytes |
| `response_bytes` | int | Sum of step response bytes |
| `response_text_bytes` | int | Sum of step text bytes |
| `response_structured_bytes` | int | Sum of step structured bytes |
| `largest_response_bytes` | int | Max single-step response |
| `largest_response_step` | string | e.g. `step:1:read.file` |
| `call_count` | int | Executed tool calls (excludes skipped steps) |
| `skipped_steps` | []string | Canonical ops skipped on this surface |
| `total_cold_bytes` | int | See cold formula below |
| `total_warm_bytes` | int | See warm formula below |
| `est_tokens_cold` | int | `total_cold_bytes / 4` (integer division) |
| `est_tokens_warm` | int | `total_warm_bytes / 4` |
| `largest_tool_schema_name` | string | Max entry in `schema_bytes_by_tool` |
| `largest_tool_schema_bytes` | int | Max schema bytes |

### Cold / warm formulas

```text
step_bytes = request_bytes + response_bytes (per run, summed over executed steps)

gateway cold  = list_tools_bytes + discover_bytes + step_bytes
gateway warm  = list_tools_bytes + step_bytes

full/compact cold = warm = list_tools_bytes + step_bytes
```

## TokenBenchmarkResult (aggregate)

| Field | Meaning |
|-------|---------|
| `benchmark_name` | e.g. `token-economy-hermetic` |
| `mode` | `deterministic-replay` |
| `timestamp` | Run time |
| `surfaces` | `["full","compact","gateway"]` |
| `scenarios` | Scenario ids run |
| `runs` | []SurfaceRun |
| `summary_by_surface` | Optional aggregates per surface |

## Harness options (token benchmark)

| Field | Default | Purpose |
|-------|---------|---------|
| `ToolSurface` | `full` | `OPENGROK_MCP_TOOL_SURFACE` for subprocess |
| `GatewayWarmDiscover` | true | Pre-run discover for warm path measurement |

## Report artifacts

| Path | Format | Purpose |
|------|--------|---------|
| `evals/token_report.json` | TokenBenchmarkResult JSON | CI diff, future baseline |
| `evals/token_report.md` | Markdown tables | Human review |
| `evals/token_report.baseline.json` | optional | Future delta (not required v1) |

Gitignore: add `evals/token_report.json`, `evals/token_report.md`, `evals/token_report.baseline.json`.

## Lifecycle

```text
For each surface in [full, compact, gateway]:
  StartHarness(surface) → ListTools (record bootstrap bytes)
  If gateway: optional warm discover (record discover_bytes for cold rows only)
  For each scenario:
    For each step:
      Resolve(op) → skip OR CallTool → record byte ledger
    Aggregate SurfaceRun (cold + warm totals)
  Teardown harness
Write token_report.json + token_report.md
```

## Validation rules

- Reject duplicate scenario `id` across files.
- Reject empty `steps` or unknown `op` at load time (if strict) or at adapter (documented error).
- Scenario `args` must not contain secret fields; hermetic env only.
- Skipped steps: `call_count` does not increment; `skipped_steps` lists op; totals use only
  executed step bytes (not fabricated).

## Relationship to contract eval entities

| Contract eval (004) | Token benchmark |
|---------------------|-----------------|
| `EvalCase.tool` | Resolved per step via adapter |
| `EvalCase.input` | Step `args` after mapping |
| `ResultCheck` | Optional `no_error` smoke only |
| `SuiteResult.score` | Not used; byte metrics primary |
| `Harness` subprocess | Shared; surface param added |
