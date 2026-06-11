# Quickstart: Token Economy Eval

**Feature**: `005-token-economy-eval` | **Date**: 2026-06-11

## Run the token benchmark (hermetic)

```bash
cd /path/to/opengrok-go-mcp
go test ./evals/ -run TestTokenBenchmark -v -count=1
```

Expected:
- Reuses hermetic OpenGrok fake from contract eval
- Spawns MCP server subprocess **three times** (full, compact, gateway)
- Replays four scenario types from `evals/testdata/scenarios/`
- Writes `evals/token_report.json` and `evals/token_report.md`
- Exits **0** regardless of byte totals (v1 — no threshold gate)

## Run contract eval only

```bash
go test ./evals/ -run TestEvalSuite -count=1
```

## Inspect token reports

```bash
less evals/token_report.md
jq '.runs[] | {scenario_id, surface, total_cold_bytes, total_warm_bytes, call_count}' evals/token_report.json
```

Look for:
- Per scenario × surface: `list_tools_bytes`, cold/warm totals, `call_count`
- `schema_bytes_by_tool` — which tool dominates bootstrap context
- `largest_tool_schema_name`, `largest_response_step`
- `response_text_bytes` vs `response_structured_bytes` — code vs wrapper overhead
- Gateway: cold includes `discover_bytes`; warm excludes it
- File exploration on compact: `files.list` in `skipped_steps`

## Compare surfaces (example questions)

| Question | Columns |
|----------|---------|
| Which surface is cheapest for symbol work? | `total_warm_bytes` on `symbol-investigation-granular` |
| Bootstrap cost? | `list_tools_bytes` + max `schema_bytes_by_tool` |
| Gateway first-use penalty? | `total_cold_bytes - total_warm_bytes` on gateway rows |
| One bloated response? | `largest_response_bytes`, `largest_response_step` |

## Add a scenario (routine — no adapter changes)

1. Create or edit `evals/testdata/scenarios/my_scenario.json`.
2. Use canonical `op` values from [contracts/token-benchmark-contract.md](./contracts/token-benchmark-contract.md).
3. Re-run `TestTokenBenchmark`.

New canonical `op` requires adapter code in `evals/surface.go` (or equivalent).

## v1 scenario corpus

| Scenario id | Steps (canonical ops) |
|-------------|------------------------|
| `symbol-investigation-granular` | definitions → read.file → references |
| `text-search-and-read` | search.code → read.file |
| `file-exploration` | files.list → path.search → read.file (compact skips list) |
| `compound-symbol-investigation` | compound.find_symbol |

## Future: baseline diffing

```bash
cp evals/token_report.json evals/token_report.baseline.json
# make surface or default changes
go test ./evals/ -run TestTokenBenchmark -count=1
# compare JSON or extend markdown renderer for Δ columns
```

Threshold CI gates: not in v1 — establish baselines first.

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Benchmark missing gateway rows | Gateway surface failed to start; check subprocess logs |
| Compact file-exploration shows skips | Expected — `files.list` not on compact surface |
| `est_tokens` seems wrong | Heuristic bytes÷4; not model tokenizer |
| Orphan processes | Missing harness teardown |

## Verify no leaks

```bash
go test ./evals/ -run TestTokenBenchmark -count=1
pgrep -f opengrok-go-mcp   # should be empty
```
