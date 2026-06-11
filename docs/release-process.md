# Release Process

Pull requests and `main` run [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)
(`go test -race ./...`, including the `evals/` harness). Every green push to `main`
also auto-updates the eval summary in `README.md` and committed baselines in
`evals/baselines/` (Δ trajectory vs the previous baseline).

Tagged releases (`v*`) run [`.github/workflows/release.yml`](../.github/workflows/release.yml):
tests, eval snapshot on the tagged commit, then [GoReleaser](https://goreleaser.com/) cross-compiles binaries for
linux/darwin/windows on amd64 and arm64, uploads archives to the GitHub Release,
generates `checksums.txt`, and SPDX SBOMs for each archive. Changelog text in
the release description is still maintained manually from `CHANGELOG.md` (GoReleaser
changelog generation is disabled). After a successful release, CI commits an eval
baseline snapshot on `main` from the tagged commit (`chore: eval snapshot for release vX.Y.Z`).

Optional follow-up: cosign/Sigstore signing of `checksums.txt` and artifacts.

## Versioning

Tags follow `vMAJOR.MINOR.PATCH`. The project is pre-1.0, so minor versions may
carry breaking changes — but each break requires a spec and a migration note
(see [constitution Principle V](../.specify/memory/constitution.md)).

Beta tags follow `vX.Y.Z-beta.N`.

## Beta Releases

1. Update `CHANGELOG.md` and README install pins if needed.
2. Push an annotated tag:
   ```
   git tag -a vX.Y.Z-beta.N -m "vX.Y.Z-beta.N"
   git push origin vX.Y.Z-beta.N
   ```
3. Wait for the Release workflow to finish. GoReleaser marks semver prerelease
   tags as GitHub prereleases automatically.
4. Edit the GitHub Release description: paste the matching `[vX.Y.Z-beta.N]`
   section from `CHANGELOG.md`.

Beta behavior may change between beta iterations. README install snippets must
pin the exact beta tag (e.g. `@v0.3.0-beta.2`) when using `go run`.

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
4. Wait for the Release workflow. Binaries, `checksums.txt`, and SBOMs attach to
   the GitHub Release automatically.
5. Edit the release description on GitHub: paste the matching `[vX.Y.Z] - DATE`
   section from `CHANGELOG.md`.

### Release artifacts

Each tag produces archives named
`opengrok-go-mcp_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) containing
the `opengrok-go-mcp` binary, plus `checksums.txt` and SPDX SBOM files. Verify
before use:

```bash
sha256sum -c checksums.txt
```

Point MCP client configs at the unpacked binary instead of `go run` when Go is
not installed (see README Client Setup).

## Changelog Rules

`CHANGELOG.md` in the repository root is the source of truth. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

> For a full minor release, the changelog summarizes changes since the previous stable minor release, not only since the latest beta. So v0.4.0 is compared against v0.3.0; beta notes are referenced separately.

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
