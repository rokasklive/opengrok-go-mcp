#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Run the eval suites and copy their reports into the committed baselines.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

go test ./evals/ -run '^(TestEvalSuite|TestTokenBenchmark)$' -count=1
mkdir -p evals/baselines
cp evals/report.json evals/baselines/report.json
cp evals/token_report.json evals/baselines/token_report.json
