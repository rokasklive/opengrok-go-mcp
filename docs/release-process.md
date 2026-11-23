# Release Process

Pull requests and pushes to `main` run [CI](../.github/workflows/ci.yml)
and upload the raw eval reports in the `eval-reports` workflow artifact.

The [release workflow](../.github/workflows/release.yml) runs the pinned
vulnerability scan and race suite once on the tagged commit. It requires
all four files in the `eval-reports-<tag>` workflow artifact before packaging.
[GoReleaser](https://goreleaser.com/) stages six archives, checksums, and an
SPDX SBOM for each archive. Native smoke jobs unpack the macOS arm64 and Windows
amd64 archives and check the server version and tool list over MCP stdio.
The publish job attaches the same staged assets after both smoke jobs pass.
Release notes come from the matching section of `CHANGELOG.md`.

Optional follow-up: cosign/Sigstore signing of `checksums.txt` and artifacts.

## Dependency security

CI and tagged releases run [the pinned vulnerability scanner](../scripts/check-vulnerabilities.sh)
with the exact Go patch in [`.go-version`](../.go-version). Release packaging uses
that same patch. Update `.go-version` when adopting a newer compiler patch.
The minimum toolchain for source builds is declared in `go.mod`.

Dependabot checks Go modules and GitHub Actions weekly. Repository security
settings control Dependabot alerts and security updates.

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
