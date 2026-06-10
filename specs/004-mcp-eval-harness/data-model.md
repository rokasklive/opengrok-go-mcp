# Data Model: MCP Eval Harness

**Feature**: `004-mcp-eval-harness` | **Date**: 2026-06-10

Eval harness state is ephemeral (one suite run). No persistence beyond optional report artifacts on disk.

## EvalCase (testdata JSON)

| Field | Type | Required | Rules |
|-------|------|----------|-------|
| `id` | string | yes | Unique within suite; report row label |
| `tool` | string | yes | Registered MCP tool name (direct-call) |
| `description` | string | yes | Human-readable; appears in report |
| `input` | object | yes | Tool arguments (JSON object) |
| `capability_gate` | string | optional | Documentation only; not enforced in code |
| `expected.tool_called` | string | yes | Equals `tool` in direct-call mode |
| `expected.arguments` | object | yes | Mirror `input` for self-consistency |
| `expected.result_checks` | array | yes | Min 1 check |
| `expected.latency_ms` | number | optional | Per-case budget; fail check if exceeded |

## ResultCheck

| `type` | Fields | Pass condition |
|--------|--------|----------------|
| `no_error` | — | No transport error; `IsError` false |
| `has_results` | `field`, `min` | Array at dotted path length ≥ `min` (default 1) |
| `field_present` | `field` | Dotted path resolves (arrays: first element) |
| `latency_ms` | `max` optional | Call duration ≤ budget |

v1 does **not** evaluate `tool_called` / `arg_match` checks (selection mode); omit from seed corpus.

## EvalResult (per case, in memory)

| Field | Meaning |
|-------|---------|
| `case_id` | From EvalCase.id |
| `tool` | Tool name |
| `passed` | All checks passed |
| `skipped` | Tool not in ListTools |
| `score` | passed_checks / total_checks |
| `latency` | Wall time for CallTool |
| `checks` | Per-check pass/fail messages |
| `errors` | Transport or parse errors |

## SuiteResult (aggregate)

| Field | Meaning |
|-------|---------|
| `suite_name` | e.g. `direct-call-hermetic` |
| `mode` | `direct-call` |
| `total` | All cases loaded |
| `skipped` | Capability-gated |
| `passed` | Judged cases all checks green |
| `failed` | Judged cases with failing checks |
| `score` | Mean case score over judged cases |
| `coverage_k` | judged / total |
| `per_tool` | map tool → mean score |
| `latency_p50/p95/p99` | Over judged case latencies |
| `results` | []EvalResult |
| `timestamp` | Run time |

## Harness lifecycle states

```text
Init → BuildBinary → StartBackend → SpawnSubprocess → ListTools
  → ForEachCase(RunCase) → Aggregate → WriteReports → Teardown
```

**RunCase**:
1. If tool ∉ registeredTools → Skipped
2. CallTool(input) + time
3. Parse StructuredContent or TextContent JSON
4. Run result_checks
5. Append EvalResult

## Hermetic backend manifest

`manifest.json` entries route HTTP method + path patterns to fixture files under `testdata/opengrok/`. Fixtures must satisfy startup probes (search returns 200) and case assertions.

## Report artifacts

| Path | Format | Purpose |
|------|--------|---------|
| `evals/report.json` | SuiteResult JSON | Baseline for deltas, CI artifact |
| `evals/report.md` | Markdown | Human review |
| `evals/report.baseline.json` | optional copy | Stable baseline path for delta |

## Validation rules

- Load testdata: reject duplicate `id`, empty `tool`, empty `result_checks`.
- Malformed JSON: fail load before subprocess spawn.
- Subprocess env: never embed secrets in case files; use env from `startBackend` + test-only cursor secret.
