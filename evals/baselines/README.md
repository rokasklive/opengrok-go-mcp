# Eval baselines (committed)

Previous CI snapshot for trajectory tracking. `scripts/update-eval-readme` and report writers compare current runs against these files.

| File | Source after each `main` push |
|---|---|
| `report.json` | `evals/report.json` (contract eval) |
| `token_report.json` | `evals/token_report.json` (token benchmark) |

README Δ columns and `evals/report.md` / `evals/token_report.md` show change vs these baselines.

**When updated**

- Every green push to `main` — bot PR auto-merged ([`ci.yml`](../../.github/workflows/ci.yml))
- Each release tag — bot PR from tagged-commit reports ([`release.yml`](../../.github/workflows/release.yml))

Requires **Allow auto-merge** on the repository and branch rules that do not mandate human review on bot PRs.

Local refresh:

```bash
go test ./evals/ -run 'TestEvalSuite|TestTokenBenchmark' -count=1
cp evals/report.json evals/baselines/report.json
cp evals/token_report.json evals/baselines/token_report.json
go run ./scripts/update-eval-readme
```
