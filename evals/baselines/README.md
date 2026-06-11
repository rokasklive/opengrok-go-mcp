# Eval baselines (committed)

Previous snapshot for trajectory tracking. `scripts/update-eval-readme` and report writers compare current runs against these files.

| File | Source after a local refresh |
|---|---|
| `report.json` | `evals/report.json` (contract eval) |
| `token_report.json` | `evals/token_report.json` (token benchmark) |

README Δ columns and `evals/report.md` / `evals/token_report.md` show change vs these baselines.

**When updated**

- Locally via `./scripts/update-eval-results.sh`
- On push when the pre-push hook is installed (`./scripts/install-githooks.sh`) — commit README + baselines before the push succeeds

```bash
go test ./evals/ -run 'TestEvalSuite|TestTokenBenchmark' -count=1
cp evals/report.json evals/baselines/report.json
cp evals/token_report.json evals/baselines/token_report.json
go run ./scripts/update-eval-readme
```

Or simply:

```bash
./scripts/update-eval-results.sh
```
