// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func WriteReports(s SuiteResult, dir string) error {
	mdPath := filepath.Join(dir, "report.md")
	jsonPath := filepath.Join(dir, "report.json")
	prev := ReadBaseline(filepath.Join(dir, "baselines", "report.json"))
	if prev.Total == 0 {
		prev = ReadBaseline(filepath.Join(dir, "report.baseline.json"))
	}
	if prev.Total == 0 {
		prev = loadPrevious(jsonPath)
	}

	if err := os.WriteFile(jsonPath, mustJSON(s), 0o644); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	if err := os.WriteFile(mdPath, []byte(renderMarkdown(s, prev)), 0o644); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}
	return nil
}

func ReadBaseline(path string) SuiteResult {
	var prev SuiteResult
	raw, err := os.ReadFile(path)
	if err != nil {
		return prev
	}
	_ = json.Unmarshal(raw, &prev)
	return prev
}

func renderMarkdown(s, prev SuiteResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Eval Results — %s\n\n", s.SuiteName)
	fmt.Fprintf(&b, "> Mode: %s  ·  Coverage@%d: %.2f  ·  Score: %.1f%%  ·  P95: %s\n\n",
		s.Mode, s.Total, s.CoverageK, s.Score*100, s.LatencyP95.Round(time.Millisecond))
	fmt.Fprintf(&b, "_Tool-selection metrics: n/a (direct-call mode)_\n\n")

	if s.Failed > 0 {
		fmt.Fprintf(&b, "**Failed cases:** ")
		for _, r := range s.Results {
			if !r.Skipped && !r.Passed {
				fmt.Fprintf(&b, "%s ", r.CaseID)
			}
		}
		fmt.Fprintf(&b, "\n\n")
	}

	fmt.Fprintf(&b, "## Aggregate\n\n| Tool | Score | Δ (prev) | Cases |\n|---|---|---|---|\n")
	tools := sortedKeys(s.PerTool)
	for _, tool := range tools {
		fmt.Fprintf(&b, "| %s | %.1f%% | %s | %d |\n",
			tool, s.PerTool[tool]*100, deltaStr(s.PerTool[tool], prev.PerTool[tool], hasTool(prev, tool)), countTool(s, tool))
	}

	fmt.Fprintf(&b, "\n## Per-Tool\n")
	for _, tool := range tools {
		fmt.Fprintf(&b, "\n### %s\n| Eval | Score | Latency | Checks |\n|---|---|---|---|\n", tool)
		for _, r := range s.Results {
			if r.Tool != tool || r.Skipped {
				continue
			}
			passed := 0
			var glyphs strings.Builder
			for _, c := range r.Checks {
				if c.Passed {
					passed++
					glyphs.WriteString("PASS ")
				} else {
					glyphs.WriteString("FAIL ")
				}
			}
			fmt.Fprintf(&b, "| %s | %d/%d (%.0f%%) | %s | %s |\n",
				r.CaseID, passed, len(r.Checks), r.Score*100, r.Latency.Round(time.Millisecond), strings.TrimSpace(glyphs.String()))
		}
	}

	if s.Skipped > 0 {
		fmt.Fprintf(&b, "\n_%d case(s) skipped (capability gated)._\n", s.Skipped)
	}
	return b.String()
}

func loadPrevious(jsonPath string) SuiteResult {
	return ReadBaseline(jsonPath)
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func hasTool(s SuiteResult, tool string) bool {
	_, ok := s.PerTool[tool]
	return ok
}

func countTool(s SuiteResult, tool string) int {
	n := 0
	for _, r := range s.Results {
		if r.Tool == tool && !r.Skipped {
			n++
		}
	}
	return n
}

func deltaStr(cur, prev float64, had bool) string {
	if !had {
		return "—"
	}
	d := (cur - prev) * 100
	switch {
	case d > 0.05:
		return fmt.Sprintf("+%.1f%%", d)
	case d < -0.05:
		return fmt.Sprintf("%.1f%%", d)
	default:
		return "±0"
	}
}
