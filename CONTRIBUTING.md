# Contributing

Read [AGENTS.md](AGENTS.md) first. Go 1.26.0+ (CI and releases use the patch pinned
in [`.go-version`](.go-version)).

```bash
go test -race -count=1 ./...
git diff --check
```

Open a PR with the [template](.github/PULL_REQUEST_TEMPLATE.md) and link the
spec if you wrote one.

## Evals

`go test ./evals/` runs the stdio contract suite and token benchmark against a
fake OpenGrok. After accepting changed results, refresh the committed
baselines with `./scripts/update-eval-results.sh` and commit
`evals/baselines/`.

## Releasing

1. Move `[Unreleased]` in `CHANGELOG.md` under `[vX.Y.Z] - DATE`, bump the
   README `@vX.Y.Z` pins, commit `chore: release vX.Y.Z`.
2. `git tag -a vX.Y.Z -m vX.Y.Z && git push origin vX.Y.Z`
3. The release workflow tests, scans, and publishes via GoReleaser. Do not
   create the GitHub Release by hand. Paste the changelog section into its
   description afterwards.

Breaking changes need a changelog migration note. `-beta.N` tags publish as
prereleases.
