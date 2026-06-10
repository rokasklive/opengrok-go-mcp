// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"strings"
	"testing"
	"time"
)

func TestReportDelta(t *testing.T) {
	cur := SuiteResult{
		SuiteName: "test",
		Mode:      "direct-call",
		Total:     2,
		PerTool:   map[string]float64{"search_code": 0.9},
		Results: []EvalResult{
			{CaseID: "a", Tool: "search_code", Passed: true, Score: 1, Checks: []CheckResult{{Type: "no_error", Passed: true}}},
		},
	}
	prev := SuiteResult{
		PerTool: map[string]float64{"search_code": 0.8},
	}

	md := renderMarkdown(cur, prev)
	if !strings.Contains(md, "+10.0%") && !strings.Contains(md, "+10") {
		t.Fatalf("expected positive delta in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "direct-call mode") {
		t.Fatalf("expected direct-call note in markdown")
	}
}

func TestReportDeltaAbsentBaseline(t *testing.T) {
	cur := SuiteResult{
		SuiteName: "test",
		Mode:      "direct-call",
		PerTool:   map[string]float64{"list_projects": 1},
	}
	md := renderMarkdown(cur, SuiteResult{})
	if !strings.Contains(md, "—") {
		t.Fatalf("expected em dash for missing baseline, got:\n%s", md)
	}
}

func TestAggregateFailedCount(t *testing.T) {
	results := []EvalResult{
		{Passed: true, Score: 1},
		{Passed: false, Score: 0},
		{Skipped: true},
	}
	s := aggregate("test", results)
	if s.Passed != 1 || s.Failed != 1 || s.Skipped != 1 {
		t.Fatalf("counts: passed=%d failed=%d skipped=%d", s.Passed, s.Failed, s.Skipped)
	}
	if s.CoverageK != 2.0/3.0 {
		t.Fatalf("coverage_k = %v, want 0.667", s.CoverageK)
	}
}

func TestPercentile(t *testing.T) {
	d := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 100 * time.Millisecond}
	if p := percentile(d, 50); p != 20*time.Millisecond {
		t.Fatalf("p50 = %s, want 20ms", p)
	}
}
