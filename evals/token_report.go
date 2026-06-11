// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func WriteTokenReports(result TokenBenchmarkResult, dir string) error {
	jsonPath := filepath.Join(dir, "token_report.json")
	mdPath := filepath.Join(dir, "token_report.md")

	prev := ReadTokenBaseline(filepath.Join(dir, "baselines", "token_report.json"))
	if len(prev.Runs) == 0 {
		prev = ReadTokenBaseline(filepath.Join(dir, "token_report.baseline.json"))
	}

	if err := os.WriteFile(jsonPath, mustJSON(result), 0o644); err != nil {
		return fmt.Errorf("write token json: %w", err)
	}
	if err := os.WriteFile(mdPath, []byte(renderTokenMarkdown(result, prev)), 0o644); err != nil {
		return fmt.Errorf("write token markdown: %w", err)
	}
	return nil
}

func renderTokenMarkdown(r, prev TokenBenchmarkResult) string {
	hadBaseline := len(prev.Runs) > 0
	var b strings.Builder
	fmt.Fprintf(&b, "# Token Economy Benchmark — %s\n\n", r.BenchmarkName)
	fmt.Fprintf(&b, "> Mode: %s  ·  Scenarios: %d  ·  Surfaces: %s\n\n",
		r.Mode, len(r.ScenarioIDs), strings.Join(r.Surfaces, ", "))
	fmt.Fprintf(&b, "_`est_tokens_*` = bytes ÷ %d (heuristic, not model-exact)._\n\n", estTokenDivisor)
	fmt.Fprintf(&b, "_Gateway **cold** includes `discover_bytes`; **warm** excludes discovery (amortized). ")
	fmt.Fprintf(&b, "Full/compact cold = warm in v1._\n\n")

	fmt.Fprintf(&b, "## Scenario × surface\n\n")
	if hadBaseline && !prev.Timestamp.IsZero() {
		fmt.Fprintf(&b, "_Δ warm vs baseline %s (est. tokens)._\n\n", prev.Timestamp.Format("2006-01-02"))
	}
	fmt.Fprintf(&b, "| Scenario | Surface | ListTools | Cold | Warm | Δ warm | Calls | Largest step |\n")
	fmt.Fprintf(&b, "|----------|---------|-----------|------|------|--------|-------|--------------|\n")
	for _, run := range r.Runs {
		warm := warmEstTokens(run)
		warmDelta := "—"
		if prevRun, ok := findSurfaceRun(prev.Runs, run.ScenarioID, run.Surface); ok && hadBaseline {
			warmDelta = deltaEstTokensStr(warm, warmEstTokens(prevRun), true)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %s | %d | %s (%d) |\n",
			run.ScenarioID, run.Surface, run.ListToolsBytes,
			run.TotalColdBytes, run.TotalWarmBytes, warmDelta, run.CallCount,
			run.LargestResponseStep, run.LargestResponseBytes)
	}

	fmt.Fprintf(&b, "\n## Top offenders (per row)\n\n")
	for _, run := range r.Runs {
		fmt.Fprintf(&b, "### %s / %s\n", run.ScenarioID, run.Surface)
		fmt.Fprintf(&b, "- Largest tool schema: **%s** (%d bytes)\n", run.LargestToolSchemaName, run.LargestToolSchemaBytes)
		fmt.Fprintf(&b, "- Largest response: **%s** (%d bytes)\n", run.LargestResponseStep, run.LargestResponseBytes)
		if len(run.SkippedSteps) > 0 {
			fmt.Fprintf(&b, "- Skipped steps: %s\n", strings.Join(run.SkippedSteps, "; "))
		}
		if run.ResponseStructuredBytes > run.ResponseTextBytes*2 && run.ResponseBytes > 0 {
			fmt.Fprintf(&b, "- Note: structured bytes dominate text — likely wrapper/metadata overhead\n")
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "\n_Generated %s_\n", r.Timestamp.Format(time.RFC3339))
	return b.String()
}
