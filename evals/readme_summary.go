// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const readmeMarkerStart = "<!-- EVAL-RESULTS START -->"
const readmeMarkerEnd = "<!-- EVAL-RESULTS END -->"

// ReadmeSummary is a compact markdown block for README.md (contract eval + token benchmark).
// prevSuite and prevToken are optional baselines for Δ columns (empty when no baseline exists).
func ReadmeSummary(cur SuiteResult, curToken TokenBenchmarkResult, prevSuite SuiteResult, prevToken TokenBenchmarkResult) string {
	var b strings.Builder
	b.WriteString(contractReadmeSection(cur, prevSuite))
	b.WriteString("\n\n")
	b.WriteString(tokenReadmeSection(curToken, prevToken))
	return strings.TrimSpace(b.String())
}

func contractReadmeSection(cur, prev SuiteResult) string {
	var b strings.Builder
	ts := cur.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	judged := cur.Total - cur.Skipped
	hadBaseline := prev.Total > 0 || len(prev.PerTool) > 0

	fmt.Fprintf(&b, "### Contract eval\n\n")
	fmt.Fprintf(&b, "Last run: **%s** · %s · [harness docs →](evals/README.md)\n\n", ts.Format("2006-01-02"), cur.Mode)
	fmt.Fprintf(&b, "**%d/%d passed** · %s · %.0f%% coverage@K",
		cur.Passed, judged, deltaScoreStr(cur.Score, prev.Score, hadBaseline), cur.CoverageK*100)
	if cur.Skipped > 0 {
		fmt.Fprintf(&b, " · %d skipped", cur.Skipped)
	}
	b.WriteString("\n")

	if len(cur.PerTool) > 0 {
		b.WriteString("\n<details>\n<summary>Per-tool scores</summary>\n\n")
		b.WriteString("| Tool | Score | Cases |\n|---|---|---|\n")
		tools := make([]string, 0, len(cur.PerTool))
		for t := range cur.PerTool {
			tools = append(tools, t)
		}
		sort.Strings(tools)
		for _, tool := range tools {
			prevScore, hadTool := prev.PerTool[tool]
			scoreCell := deltaScoreStr(cur.PerTool[tool], prevScore, hadBaseline && hadTool)
			fmt.Fprintf(&b, "| %s | %s | %d |\n", tool, scoreCell, countTool(cur, tool))
		}
		b.WriteString("\n</details>\n")
	}

	if cur.Failed > 0 {
		b.WriteString("\n_Failing: ")
		for _, r := range cur.Results {
			if !r.Skipped && !r.Passed {
				b.WriteString(r.CaseID + " ")
			}
		}
		b.WriteString("_\n")
	}

	if !hadBaseline {
		b.WriteString("\n_Baseline: none — copy `evals/report.json` to `evals/baselines/report.json` to track Δ._\n")
	}

	return strings.TrimSpace(b.String())
}

func tokenReadmeSection(cur, prev TokenBenchmarkResult) string {
	var b strings.Builder
	ts := cur.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	hadBaseline := len(prev.Runs) > 0

	fmt.Fprintf(&b, "### Token economy benchmark\n\n")
	fmt.Fprintf(&b, "Last run: **%s** · %s · est. tokens = bytes÷4 (heuristic, not model-exact)\n\n",
		ts.Format("2006-01-02"), cur.Mode)

	bySurface := surfaceWarmStats(cur.Runs)
	if len(bySurface) == 0 {
		b.WriteString("_No benchmark runs recorded._\n")
		return strings.TrimSpace(b.String())
	}

	prevBySurface := surfaceWarmStats(prev.Runs)

	b.WriteString("**ListTools** dominates session cost on the full surface (17 tools). Compact (6) and gateway (2) register far fewer schemas.\n\n")
	b.WriteString("| Surface | ListTools | Warm total (min–max) |\n|---|---|---|\n")
	surfaces := []string{"full", "compact", "gateway"}
	for _, surface := range surfaces {
		st, ok := bySurface[surface]
		if !ok {
			continue
		}
		prevSt, hadSurface := prevBySurface[surface]
		listCell := deltaEstTokensStr(st.listTools, prevSt.listTools, hadBaseline && hadSurface)
		rangeCell := deltaEstRangeStr(st.minWarm, st.maxWarm, prevSt.minWarm, prevSt.maxWarm, hadBaseline && hadSurface)
		fmt.Fprintf(&b, "| %s | %s | %s |\n", surface, listCell, rangeCell)
	}

	b.WriteString("\n_Gateway **warm** excludes `discover` (~864 est. tokens cold). Compact **file-exploration** skips `files.list`._\n")

	b.WriteString("\n<details>\n<summary>Per-scenario warm totals (est. tokens)</summary>\n\n")
	b.WriteString("| Scenario | full | compact | gateway |\n|---|---|---|---|\n")
	for _, id := range cur.ScenarioIDs {
		cells := warmTokensByScenarioWithDelta(cur.Runs, prev.Runs, id, hadBaseline)
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", scenarioLabel(id), cells.full, cells.compact, cells.gateway)
	}
	b.WriteString("\n</details>\n")

	if !hadBaseline {
		b.WriteString("\n_Baseline: none — copy `evals/token_report.json` to `evals/baselines/token_report.json` to track Δ._\n")
	} else if !prev.Timestamp.IsZero() {
		fmt.Fprintf(&b, "\n_Δ vs baseline from %s._\n", prev.Timestamp.Format("2006-01-02"))
	}

	return strings.TrimSpace(b.String())
}

type surfaceStat struct {
	listTools int
	minWarm   int
	maxWarm   int
}

func surfaceWarmStats(runs []SurfaceRun) map[string]surfaceStat {
	out := make(map[string]surfaceStat)
	for _, run := range runs {
		st := out[run.Surface]
		if st.listTools == 0 {
			st.listTools = listToolsEstTokens(run)
		}
		warm := warmEstTokens(run)
		if st.minWarm == 0 || warm < st.minWarm {
			st.minWarm = warm
		}
		if warm > st.maxWarm {
			st.maxWarm = warm
		}
		out[run.Surface] = st
	}
	return out
}

type scenarioWarmCells struct {
	full, compact, gateway string
}

func warmTokensByScenarioWithDelta(curRuns, prevRuns []SurfaceRun, scenarioID string, hadBaseline bool) scenarioWarmCells {
	var cells scenarioWarmCells
	for _, surface := range []string{"full", "compact", "gateway"} {
		curRun, ok := findSurfaceRun(curRuns, scenarioID, surface)
		if !ok {
			continue
		}
		curWarm := warmEstTokens(curRun)
		prevRun, hadPrev := findSurfaceRun(prevRuns, scenarioID, surface)
		prevWarm := 0
		if hadPrev {
			prevWarm = warmEstTokens(prevRun)
		}
		val := deltaEstTokensStr(curWarm, prevWarm, hadBaseline && hadPrev)
		switch surface {
		case "full":
			cells.full = val
		case "compact":
			cells.compact = val
		case "gateway":
			cells.gateway = val
		}
	}
	return cells
}

func scenarioLabel(id string) string {
	switch id {
	case "compound-symbol-investigation":
		return "Compound symbol"
	case "file-exploration":
		return "File exploration"
	case "symbol-investigation-granular":
		return "Symbol investigation (3 calls)"
	case "text-search-and-read":
		return "Search + read"
	default:
		return id
	}
}

func formatEstTokens(est int) string {
	if est >= 10000 {
		return fmt.Sprintf("%.0fk", float64(est)/1000)
	}
	if est >= 1000 {
		return fmt.Sprintf("%.1fk", float64(est)/1000)
	}
	return fmt.Sprintf("%d", est)
}

// PatchREADME replaces the block between EVAL-RESULTS markers in readmePath.
func PatchREADME(readmePath string, summary string) error {
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("read readme: %w", err)
	}
	content := string(raw)
	if !strings.Contains(content, readmeMarkerStart) || !strings.Contains(content, readmeMarkerEnd) {
		return fmt.Errorf("readme missing %s / %s markers", readmeMarkerStart, readmeMarkerEnd)
	}
	start := strings.Index(content, readmeMarkerStart)
	end := strings.Index(content, readmeMarkerEnd)
	if end < start {
		return fmt.Errorf("invalid readme marker order")
	}
	end += len(readmeMarkerEnd)

	var b strings.Builder
	b.WriteString(content[:start])
	b.WriteString(readmeMarkerStart)
	b.WriteString("\n\n")
	b.WriteString(summary)
	b.WriteString("\n\n")
	b.WriteString(readmeMarkerEnd)
	b.WriteString(content[end:])
	return os.WriteFile(readmePath, []byte(b.String()), 0o644)
}
