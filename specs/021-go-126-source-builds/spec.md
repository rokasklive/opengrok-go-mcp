# Go 1.26 Minimum for Source Builds

The v0.7.0 update to `golang.org/x/net` v0.63.0 raises the module minimum from
Go 1.25.0 to Go 1.26.0. Users who run `go run`, `go install`, or build from
source need Go 1.26.0 or later. Users can run a prebuilt release binary on
systems with an older or absent Go installation.

CI and release packaging use the separate patch in `.go-version`. Verify the
test suite on Go 1.26.0 and verify the race suite and vulnerability scan on
the release patch.
