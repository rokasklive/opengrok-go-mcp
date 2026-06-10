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

// ReadmeSummary is a stable markdown block for README.md (scores and counts, not raw latencies).
func ReadmeSummary(s SuiteResult) string {
	var b strings.Builder
	ts := s.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	fmt.Fprintf(&b, "Hermetic stdio subprocess eval suite (`go test ./evals/`). ")
	fmt.Fprintf(&b, "Last CI run: %s. ", ts.Format("2006-01-02"))
	fmt.Fprintf(&b, "Mode: %s. Details: [evals/README.md](evals/README.md).\n\n", s.Mode)
	fmt.Fprintf(&b, "| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Cases | %d total, %d judged, %d skipped |\n", s.Total, s.Total-s.Skipped, s.Skipped)
	fmt.Fprintf(&b, "| Score (judged) | %.1f%% |\n", s.Score*100)
	fmt.Fprintf(&b, "| Coverage@K | %.0f%% |\n", s.CoverageK*100)
	fmt.Fprintf(&b, "| Passed / failed | %d / %d |\n", s.Passed, s.Failed)

	if len(s.PerTool) > 0 {
		b.WriteString("\n| Tool | Score | Judged cases |\n|---|---|---|\n")
		tools := make([]string, 0, len(s.PerTool))
		for t := range s.PerTool {
			tools = append(tools, t)
		}
		sort.Strings(tools)
		for _, tool := range tools {
			fmt.Fprintf(&b, "| %s | %.1f%% | %d |\n", tool, s.PerTool[tool]*100, countTool(s, tool))
		}
	}

	if s.Failed > 0 {
		b.WriteString("\n_Failing cases: ")
		for _, r := range s.Results {
			if !r.Skipped && !r.Passed {
				b.WriteString(r.CaseID + " ")
			}
		}
		b.WriteString("_\n")
	}

	return strings.TrimSpace(b.String())
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
