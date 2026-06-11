#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Run eval suites, patch README.md (with Δ vs evals/baselines/), refresh baselines.
# Used locally and by the pre-push git hook — not CI.
#
# Usage:
#   ./scripts/update-eval-results.sh              # run tests + patch README + baselines
#   ./scripts/update-eval-results.sh --skip-tests # patch only (reports must exist)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_TESTS=true

while [[ $# -gt 0 ]]; do
	case "$1" in
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
