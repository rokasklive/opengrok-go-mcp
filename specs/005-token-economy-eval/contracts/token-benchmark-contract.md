# Token Benchmark Contract: Token Economy Eval

**Feature**: `005-token-economy-eval` | **Date**: 2026-06-11

Maintainer-facing contract for the token economy benchmark. Does **not** change the MCP tool
contract exposed to agents.

## Entrypoints

| Command | Behavior |
|---------|----------|
| `go test ./evals/ -run TestTokenBenchmark -v -count=1` | Full hermetic token benchmark; **does not fail** on byte thresholds in v1 |
| `go test ./evals/ -count=1` | Runs contract eval + token benchmark (when both tests exist) |
| `go test ./...` | Full module including evals |

Contract eval remains:

| Command | Behavior |
|---------|----------|
| `go test ./evals/ -run TestEvalSuite -count=1` | Fails on judged case failures |

## Environment (hermetic)

Same as contract eval — set by `startBackend`:

| Variable | Purpose |
|----------|---------|
| `OPENGROK_MCP_BASE_URL` | Fake OpenGrok REST |
| `OPENGROK_MCP_WEB_BASE_URL` | Web/raw URLs |
| `OPENGROK_MCP_PROJECTS` | `platform,infra` |
| `OPENGROK_MCP_DEFAULT_PROJECT` | `platform` |
| `OPENGROK_MCP_PROBE_FILE` | File capability probe |
| `OPENGROK_MCP_CURSOR_SECRET` | Test cursor signing |
| `OPENGROK_MCP_TRANSPORT` | `stdio` |

Token benchmark additionally sets per subprocess:

| Variable | Values | Purpose |
|----------|--------|---------|
| `OPENGROK_MCP_TOOL_SURFACE` | `full`, `compact`, `gateway` | One value per harness lifecycle |

## Scenario testdata

- Location: `evals/testdata/scenarios/*.json`
- Schema: ordered `steps` with canonical `op` + `args` — see [data-model.md](../data-model.md)
- OpenGrok fixtures: shared `evals/testdata/opengrok/` + `manifest.json` (contract eval)

## Canonical operations (v1)

Adapters must implement mapping for:

| `op` | Notes |
|------|-------|
| `search.definitions` | Symbol definition search |
| `search.references` | Symbol reference search |
| `search.code` | Full-text search |
| `path.search` | File path search (`mode=path` on underlying search) |
| `read.file` | Full file read |
| `files.list` | Directory listing — **skipped on compact** |
| `compound.find_symbol` | Definition + references compound |
| `compound.search_and_read` | Search + read compound |

Gateway operations use `opengrok_discover` + `opengrok_call` with registry names
(e.g. `search.definitions`, `file.read`).

## Metrics contract (per scenario × surface row)

Required fields in `token_report.json` for each `SurfaceRun`:

| Field | Definition |
|-------|------------|
| `list_tools_bytes` | UTF-8 bytes of serialized `ListTools` tools list |
| `schema_bytes_by_tool` | Map: tool name → bytes of name+description+input+output schema only |
| `discover_bytes` | Gateway discover response bytes; 0 for full/compact |
| `request_bytes` | Sum of per-call `{name, arguments}` JSON bytes |
| `response_bytes` | Sum of per-call response bytes |
| `response_text_bytes` | Sum of text content channel bytes |
| `response_structured_bytes` | Sum of structured content channel bytes |
| `largest_response_bytes` | Max single-step `response_bytes` |
| `total_cold_bytes` | See data-model cold formula |
| `total_warm_bytes` | See data-model warm formula |
| `est_tokens_cold` | `total_cold_bytes / 4` (documented heuristic) |
| `est_tokens_warm` | `total_warm_bytes / 4` |
| `call_count` | Executed tool calls |
| `largest_tool_schema_name` | Tool with max `schema_bytes_by_tool` |
| `largest_tool_schema_bytes` | Max schema bytes |
| `largest_response_step` | Step identifier for max response |

Markdown report must:
- Label `est_tokens_*` as heuristic (not model-exact).
- Explain gateway cold vs warm (`discover_bytes` in cold only).
- Note compact `files.list` skip for file-exploration scenario.

## Skip vs fail

| Condition | Outcome |
|-----------|---------|
| Canonical op not on surface (e.g. `files.list` on compact) | Step **skipped**; listed in `skipped_steps`; not a test failure |
| `CallTool` transport error | Recorded on step; v1 benchmark still completes (optional: warn in report) |
| Unknown canonical `op` at load | **Fail** load before benchmark |
| Byte total exceeds informal threshold | **No failure** in v1 |

## Report contract

After benchmark:

- `evals/token_report.json` — full `TokenBenchmarkResult`
- `evals/token_report.md` — comparison tables (scenario × surface), top offenders

Reports must not contain auth tokens. May include byte counts and tool names only.

Optional future: `evals/token_report.baseline.json` for deltas (not required v1 pass/fail).

## Process hygiene

- Zero `opengrok-go-mcp` processes after `go test ./evals/ -run TestTokenBenchmark` completes.
- Three subprocess lifecycles per full benchmark (one per surface) unless optimized later.

## Relationship to other layers

| Layer | Proves | Token benchmark |
|-------|--------|-----------------|
| Contract eval (004) | Correct structured outputs | Complements — does not replace |
| Token benchmark | Byte cost at MCP boundaries | **This feature** |
| LLM tool-selection eval | Model chooses tools | Out of scope v1 |

## v1 scope boundaries

- Deterministic replay only — no LLM agent.
- Hermetic backend only for CI default.
- No byte threshold CI gate.
- No server MCP contract changes.
- No session-amortized `list_tools` across scenarios (full/compact cold = warm).
