# Release Process

Releases are manual: push a git tag, then create a GitHub Release whose body
you paste from `CHANGELOG.md`. There is no release automation or goreleaser
config. Pull requests and `main` run [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)
(`go test -race ./...`, including the `evals/` harness).

## Versioning

Tags follow `vMAJOR.MINOR.PATCH`. The project is pre-1.0, so minor versions may
carry breaking changes — but each break requires a spec and a migration note
(see [constitution Principle V](../.specify/memory/constitution.md)).

Beta tags follow `vX.Y.Z-beta.N`.

## Beta Releases

Tag the commit:

```
git tag vX.Y.Z-beta.N
git push origin vX.Y.Z-beta.N
```

Beta behavior may change between beta iterations. README install snippets must
pin the exact beta tag (e.g. `@v0.3.0-beta.2`).

## Full Releases

1. Update `CHANGELOG.md`: move the `[Unreleased]` entries under a new
   `[vX.Y.Z] - DATE` heading.
2. Commit the changelog change:
   ```
   git commit -m "chore: release vX.Y.Z"
   ```
3. Create and push an annotated tag:
   ```
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```
4. Create a GitHub Release for the tag. Copy the matching `[vX.Y.Z] - DATE`
   section from `CHANGELOG.md` into the release description (GitHub does not
   auto-import `CHANGELOG.md` unless you add release automation later).

## Changelog Rules

`CHANGELOG.md` in the repository root is the source of truth. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

> For a full minor release, the changelog summarizes changes since the previous stable minor release, not only since the latest beta. So v0.3.0 is compared against v0.2.0; beta notes are referenced separately.

## Compatibility Notes

Call out any change to a public default, tool schema field, or environment
variable in the changelog entry. Breaking changes and experimental surface
changes must also be noted in [`docs/tool-contracts.md`](tool-contracts.md).

## Migration Notes

Any breaking change requires a migration note that tells integrators exactly
what to update: which env var, field, or default changed, what value to use
instead, and whether existing behavior can be restored through configuration.

## Pre-Release Checklist

- `go test ./...` passes with no failures.
- README install snippets pin the new tag.
- `CHANGELOG.md` is updated with the release entry.
- [`docs/limitations.md`](limitations.md) reflects current behavior.
- `git diff --check` is clean (no trailing whitespace).
