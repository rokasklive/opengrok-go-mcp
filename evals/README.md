# MCP eval harness

Dataset-driven stdio subprocess tests for `opengrok-go-mcp`. Builds the real binary, talks MCP over stdio, and hits a hermetic OpenGrok fake.

**Latest results:** root [README.md](../README.md#evaluation) (CI-updated).

## Run

```bash
go test ./evals/ -count=1
```

| Test | What it checks |
|---|---|
| `TestEvalSuite` | MCP contract — tool outputs, errors, pagination fields |
| `TestTokenBenchmark` | Token economy — UTF-8 bytes at MCP boundaries per surface |

Reports (gitignored locally; CI uploads artifacts):

| Report | Files |
|---|---|
| Contract | `evals/report.json`, `evals/report.md` |
| Token benchmark | `evals/token_report.json`, `evals/token_report.md` |

Refresh README locally (same steps as CI):

```bash
./scripts/ci-update-eval-results.sh
```

## CI and release automation

| Event | Tests | README / baselines | Artifacts |
|---|---|---|---|
| Pull request | `go test -race ./...` (gate) | no auto-commit | `eval-reports` |
| Push to `main` | same | opens PR → auto-merge | `eval-reports` |
| Release tag `v*` | on tagged commit | opens PR → auto-merge on `main` | `eval-reports-<tag>` |

Every green **main** push opens a bot PR with README + baseline updates (`--pr`); it auto-merges when CI passes (no direct push to `main`). **Release tags** use eval reports from the **tagged commit** with message `chore: eval snapshot for release vX.Y.Z`.

**Repo settings (owner):**

1. **Settings → General → Pull Requests** → **Allow auto-merge**
2. **Settings → Actions → General → Workflow permissions**
   - **Read and write permissions**
   - **Allow GitHub Actions to create and approve pull requests** (required for `gh pr create`; error `createPullRequest` if off)
3. **Rules for `main`:** require PR + status checks; **no** required human reviewers

If your org blocks token PR creation, add a fine-scoped PAT as secret `EVAL_UPDATE_TOKEN` (contents + pull requests) and set `GH_TOKEN` to it in the workflow.

Re-runs use branch `chore/eval-results-<run_id>-<attempt>`. A failed run that pushed but did not open a PR may leave `chore/eval-results-<run_id>` on the remote — open that PR manually or delete the branch before re-running.

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

Committed baselines live in [`evals/baselines/`](baselines/). Reports and the root README show **Δ vs baseline**.

```bash
./scripts/ci-update-eval-results.sh
```

Optional local-only baseline (gitignored): `evals/report.baseline.json` or `evals/token_report.baseline.json`.
