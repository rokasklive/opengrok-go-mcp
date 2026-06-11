#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Run eval suites, patch README.md (with Δ vs evals/baselines/), refresh baselines.
# Used by CI (every main push) and Release (tag snapshot on main).
#
# Usage:
#   ./scripts/ci-update-eval-results.sh                    # run tests + patch only
#   ./scripts/ci-update-eval-results.sh --commit "msg"     # local: commit on current branch + push
#   ./scripts/ci-update-eval-results.sh --pr "msg"         # CI: branch + PR + auto-merge to main
#   ./scripts/ci-update-eval-results.sh --skip-tests --pr "msg"
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_TESTS=true
COMMIT_MSG=""
OPEN_PR=false

while [[ $# -gt 0 ]]; do
	case "$1" in
	--commit)
		COMMIT_MSG="${2:-}"
		shift 2
		;;
	--pr)
		COMMIT_MSG="${2:-}"
		OPEN_PR=true
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

if [[ -z "$COMMIT_MSG" ]]; then
	exit 0
fi

if git diff --quiet README.md evals/baselines/; then
	echo "README and baselines already up to date"
	exit 0
fi

git add README.md evals/baselines/

if [[ "$OPEN_PR" == true ]]; then
	if [[ -z "${GH_TOKEN:-}" ]]; then
		echo "GH_TOKEN is required for --pr (set to github.token in Actions)" >&2
		exit 1
	fi
	if ! command -v gh >/dev/null 2>&1; then
		echo "gh CLI is required for --pr" >&2
		exit 1
	fi

	BRANCH="chore/eval-results-${GITHUB_RUN_ID:-local-$(date +%s)}"
	git checkout -b "$BRANCH"
	git commit -m "$COMMIT_MSG"
	git push -u origin "$BRANCH"

	gh pr create \
		--base main \
		--head "$BRANCH" \
		--title "$COMMIT_MSG" \
		--body "Automated eval README and baseline update from CI. Auto-merge when checks pass."

	gh pr merge "$BRANCH" --auto --squash
	echo "PR opened for $BRANCH and marked for auto-merge"
	exit 0
fi

git commit -m "$COMMIT_MSG"
git push
