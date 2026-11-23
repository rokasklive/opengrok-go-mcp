# Dependency Security Checks

Dependabot alerts and security updates are enabled on the GitHub repository.
Its version updates check root Go modules and GitHub Actions weekly.

Pull request CI and tagged release verification run the same pinned
`govulncheck` command with the Go patch in `.go-version`. Automatic Go toolchain
switching is disabled. Release packaging uses that same Go patch. The
module still declares Go 1.25.0 as its minimum.

Verify workflow syntax, the weekly schedules, and the scan's reported Go and
scanner versions. Run the full race suite on the release patch.
