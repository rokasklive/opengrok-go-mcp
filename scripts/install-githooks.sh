#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Point this repo's git hooks at .githooks/ (pre-push runs evals + README refresh).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

git config core.hooksPath .githooks
chmod +x .githooks/pre-push

echo "Installed git hooks from .githooks/ (core.hooksPath)."
echo "Pre-push runs: go test -race ./..., then README/baseline refresh (blocks if uncommitted)."
