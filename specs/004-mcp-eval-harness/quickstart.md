# Quickstart: MCP Eval Harness

**Feature**: `004-mcp-eval-harness` | **Date**: 2026-06-10

## Run the suite (hermetic, no live OpenGrok)

```bash
cd /path/to/opengrok-go-mcp
go test ./evals/ -run TestEvalSuite -v -count=1
```

Expected:
- Builds `opengrok-go-mcp` binary to a temp path (or uses prebuilt via env)
- Starts httptest fake OpenGrok
- Spawns MCP server subprocess on stdio
- Runs all cases in `evals/testdata/*.json`
- Writes `evals/report.md` and `evals/report.json`
- Exits 0 when all **judged** cases pass

## Full module verification

```bash
go test ./... -count=1
```

## Inspect reports

```bash
less evals/report.md
jq . evals/report.json
```

Look for:
- Per-tool pass rate and skip counts
- Coverage@K (judged vs total)
- Latency p50/p95
- Failed case `id` and check messages

## Compare to a baseline

```bash
cp evals/report.json evals/report.baseline.json
# make changes
go test ./evals/ -run TestEvalSuite -count=1
# report.md shows Δ vs baseline
```

## Add a case (no Go changes)

1. Edit `evals/testdata/search_code.json` (or create tool file).
2. Add an object with `id`, `tool`, `input`, `expected.result_checks`.
3. Re-run suite.

Example check types: `no_error`, `has_results` + `field: results`, `field_present` + `field: results.citation.url`.

Field names must match `internal/mcpserver/types.go` JSON tags for the tool output type.

## Seed data source

Copy from skill pack when bootstrapping:

```text
.agents/skills/mcp-eval-harness/test_data_pack/evalcases/  → evals/testdata/
.agents/skills/mcp-eval-harness/test_data_pack/opengrok/   → evals/testdata/opengrok/
.agents/skills/mcp-eval-harness/test_data_pack/manifest.json → evals/testdata/manifest.json
```

## Optional: live OpenGrok smoke (maintainer)

Not required for CI green:

```bash
export OPENGROK_MCP_BASE_URL=https://opengrok.home/api/v1
export OPENGROK_MCP_DEFAULT_PROJECT=opengrok-go-mcp
# export OPENGROK_MCP_API_TOKEN=...  if required
go test ./evals/ -run TestEvalSuiteLive -v -count=1   # if implemented as optional test
```

## Troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Subprocess exits immediately | OpenGrok backend unreachable; check fake URL env |
| All search cases skipped | Search capability probe failed; fix `/search` fixture |
| Orphan processes | Missing `session.Close()` / transport cleanup |
| Flaky latency failures | Per-case budget too tight; tune `latency_ms` check |

## Verify no leaks

```bash
go test ./evals/ -count=1
pgrep -f opengrok-go-mcp   # should be empty
```
