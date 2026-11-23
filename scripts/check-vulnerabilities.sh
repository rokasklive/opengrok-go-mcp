#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

export GOTOOLCHAIN=local
bin_dir=$(mktemp -d)
trap 'rm -rf -- "$bin_dir"' EXIT

GOBIN="$bin_dir" go install golang.org/x/vuln/cmd/govulncheck@v1.10.0
CGO_ENABLED=0 "$bin_dir/govulncheck" -show version ./...
