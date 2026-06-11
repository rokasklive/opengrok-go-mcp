#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Run eval suites, patch README.md (with Δ vs evals/baselines/), refresh baselines.
# Used by CI (every main push) and Release (tag snapshot on main).
#
# Usage:
#   ./scripts/ci-update-eval-results.sh              # run tests + patch only
#   ./scripts/ci-update-eval-results.sh --commit "msg" # also git commit+push if changed
#   ./scripts/ci-update-eval-results.sh --skip-tests --commit "msg"  # reports already in evals/
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_TESTS=true
COMMIT_MSG=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--commit)
		COMMIT_MSG="${2:-}"
		shift 2
		;;
	--skip-tests)
		RUN_TESTS=false
		shift
		;;
	*)
		echo "unknown argument: $1" >&2
		exit 1
		;;
	esac
done

if [[ "$RUN_TESTS" == true ]]; then
	go test ./evals/ -run TestEvalSuite -count=1
	go test ./evals/ -run TestTokenBenchmark -count=1
fi

if [[ ! -f evals/report.json ]] || [[ ! -f evals/token_report.json ]]; then
	echo "missing evals/report.json or evals/token_report.json (run eval suites first)" >&2
	exit 1
fi

go run ./scripts/update-eval-readme

mkdir -p evals/baselines
cp evals/report.json evals/baselines/report.json
cp evals/token_report.json evals/baselines/token_report.json

if [[ -n "$COMMIT_MSG" ]]; then
	if git diff --quiet README.md evals/baselines/; then
		echo "README and baselines already up to date"
		exit 0
	fi
	git add README.md evals/baselines/
	git commit -m "$COMMIT_MSG"
	git push
fi
