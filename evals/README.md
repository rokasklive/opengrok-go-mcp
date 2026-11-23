# MCP eval harness

Dataset-driven stdio subprocess tests for `opengrok-go-mcp`. Builds the server once per test invocation and launches separate stdio processes
against local OpenGrok fixtures. With `-race`, the server binary also uses race
instrumentation and a subprocess race fails the suite.

## Run

```bash
go test ./evals/ -count=1
```

| Test | What it checks |
|---|---|
| `TestEvalSuite` | MCP contract — tool outputs, errors, pagination fields (**runs on `full` and `compact`**) |
| `TestCrossSurfaceEquivalenceOnSharedCases` | Shared cases produce equivalent outputs across surfaces |
| `TestTokenBenchmark` | Token economy — UTF-8 bytes at MCP boundaries per surface |

Reports (gitignored locally; CI uploads artifacts):

| Report | Files |
|---|---|
| Contract | `evals/report.json`, `evals/report.md` |
| Token benchmark | `evals/token_report.json`, `evals/token_report.md` |

Refresh the committed baselines in `evals/baselines/` after accepting changed results:

```bash
./scripts/update-eval-results.sh
```

## CI

| Event | Tests | Artifacts |
|---|---|---|
| Pull request | `go test -race ./...` | `eval-reports` |
| Release tag `v*` | on tagged commit | `eval-reports-<tag>` |

To smoke-test an unpacked release archive locally, point the test at the
executable. It initializes MCP and checks for the compact search and read
tools. Set `OPENGROK_MCP_EVAL_VERSION` to check the reported release version.

```bash
OPENGROK_MCP_EVAL_BINARY=/path/to/opengrok-go-mcp \
  go test ./evals/ -run '^TestReleaseArchiveSmoke$' -count=1
```

## Token economy benchmark

Four surface-agnostic scenarios replayed on **full**, **compact**, and **gateway** surfaces.
Counts bytes for `ListTools`, gateway `discover`, and each tool call (request + response, text vs structured split).

- Gateway **cold** includes `discover`; **warm** excludes it (amortized).
- Compact **file-exploration** skips `files.list` (no compact equivalent).
- v1 does **not** fail CI on byte thresholds.

Scenarios: `evals/testdata/scenarios/*.json` (canonical `op` + `args`). Surface mapping: `evals/surface.go`.

## Add a contract case (no Go changes)

1. Edit or create `evals/testdata/<tool>.json`.
2. Re-run `go test ./evals/ -run TestEvalSuite -count=1`.

<details>
<summary>Case schema and check types</summary>

| Field | Required | Notes |
|-------|----------|-------|
| `id` | yes | Unique within suite |
| `tool` | yes | MCP tool name |
| `description` | yes | Shown in report |
| `input` | yes | Tool arguments |
| `expected.tool_called` | yes | Same as `tool` |
| `expected.arguments` | yes | Mirror `input` |
| `expected.result_checks` | yes | Min 1 check |

| `type` | Fields | Meaning |
|--------|--------|---------|
| `no_error` | — | No transport error; tool not `IsError` |
| `has_results` | `field`, `min` | Array at dotted path length ≥ `min` |
| `field_present` | `field` | Dotted path exists |
| `latency_ms` | `max` optional | Call duration budget |

Output field names must match JSON tags on `internal/mcpserver` `*Output` types.

</details>

## Fixtures

- `evals/testdata/*.json` — contract eval cases
- `evals/testdata/scenarios/` — token benchmark scenarios
- `evals/testdata/manifest.json` + `opengrok/` — httptest fake routes

## Baselines and Δ trajectory

Committed baselines live in [`evals/baselines/`](baselines/). Raw reports show **Δ vs baseline**.

```bash
./scripts/update-eval-results.sh
```

Optional local-only baseline (gitignored): `evals/report.baseline.json` or `evals/token_report.baseline.json`.
