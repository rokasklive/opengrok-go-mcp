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

	# Include run attempt so re-runs do not collide with a leftover bot branch.
	RUN_SUFFIX="${GITHUB_RUN_ID:-local-$(date +%s)}"
	if [[ -n "${GITHUB_RUN_ATTEMPT:-}" ]]; then
		RUN_SUFFIX="${RUN_SUFFIX}-${GITHUB_RUN_ATTEMPT}"
	fi
	BRANCH="chore/eval-results-${RUN_SUFFIX}"

	git fetch origin "$BRANCH" 2>/dev/null || true

	PR_STATE="$(gh pr view "$BRANCH" --json state -q .state 2>/dev/null || true)"
	if [[ "$PR_STATE" == "OPEN" ]]; then
		echo "PR already open for $BRANCH"
		gh pr merge "$BRANCH" --auto --squash || true
		exit 0
	fi

	git checkout -B "$BRANCH"
	git commit -m "$COMMIT_MSG"

	if ! git push -u origin "$BRANCH"; then
		echo "push rejected for $BRANCH; updating bot branch with force-with-lease" >&2
		git push -u origin "$BRANCH" --force-with-lease
	fi

	if ! gh pr create \
		--base main \
		--head "$BRANCH" \
		--title "$COMMIT_MSG" \
		--body "Automated eval README and baseline update from CI. Auto-merge when checks pass."; then
		if gh pr view "$BRANCH" >/dev/null 2>&1; then
			echo "PR already exists for $BRANCH"
		else
			cat >&2 <<'EOF'
gh pr create failed. Common fix (repo owner):

  Settings → Actions → General → Workflow permissions
    - Read and write permissions
    - Enable "Allow GitHub Actions to create and approve pull requests"

If your org blocks that for GITHUB_TOKEN, add a fine-scoped PAT as repo secret
EVAL_UPDATE_TOKEN and set GH_TOKEN to it in the workflow.

Or open a PR manually from the pushed branch above.
EOF
			exit 1
		fi
	fi

	if ! gh pr merge "$BRANCH" --auto --squash; then
		echo "gh pr merge --auto failed; open the PR in GitHub and merge manually" >&2
		exit 1
	fi
	echo "PR opened for $BRANCH and marked for auto-merge"
	exit 0
fi

git commit -m "$COMMIT_MSG"
git push
