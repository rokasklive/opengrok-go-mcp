# MCP eval harness

Dataset-driven stdio subprocess eval suite for `opengrok-go-mcp`. Validates the MCP contract
through the real binary path (build → env → stdio JSON-RPC) against a hermetic OpenGrok fake.

## Run

```bash
go test ./evals/ -run TestEvalSuite -v -count=1
go test ./evals/ -count=1
```

Reports: `evals/report.md` and `evals/report.json` (gitignored locally; uploaded as CI artifacts on PRs).

## CI

[`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) runs `go test -race -count=1 ./...` on every pull
request and push to `main` (eval suite included). Pushes to `main` also refresh the eval summary in
the root `README.md` via `go run ./scripts/update-eval-readme`.

## Add a case (no Go changes)

1. Edit or create `evals/testdata/<tool>.json` (array of case objects).
2. Re-run the suite.

### Case schema

| Field | Required | Notes |
|-------|----------|-------|
| `id` | yes | Unique within suite |
| `tool` | yes | MCP tool name (direct-call) |
| `description` | yes | Shown in report |
| `input` | yes | Tool arguments object |
| `expected.tool_called` | yes | Same as `tool` in direct-call mode |
| `expected.arguments` | yes | Mirror `input` |
| `expected.result_checks` | yes | Min 1 check |

### Check types

| `type` | Fields | Meaning |
|--------|--------|---------|
| `no_error` | — | No transport error; tool not `IsError` |
| `has_results` | `field`, `min` | Array at dotted path length ≥ `min` |
| `field_present` | `field` | Dotted path exists (`results.citation.url`) |
| `latency_ms` | `max` optional | Call duration budget |

Output field names must match JSON tags on `internal/mcpserver` `*Output` types.

## Baseline deltas

```bash
cp evals/report.json evals/report.baseline.json
go test ./evals/ -count=1
```

## Fixtures

- `evals/testdata/*.json` — MCP eval cases
- `evals/testdata/manifest.json` — routes for httptest fake
- `evals/testdata/opengrok/` — canned OpenGrok REST responses

Seed corpus adapted from `.agents/skills/mcp-eval-harness/test_data_pack/`.
