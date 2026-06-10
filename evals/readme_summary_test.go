// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPatchREADME(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	initial := "before\n" + readmeMarkerStart + "\nold\n" + readmeMarkerEnd + "\nafter\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	suite := SuiteResult{
		Mode:      "direct-call",
		Timestamp: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		Total:     2,
		Passed:    2,
		Score:     1,
		CoverageK: 1,
		PerTool:   map[string]float64{"search_code": 1},
		Results: []EvalResult{
			{CaseID: "a", Tool: "search_code", Passed: true, Score: 1},
		},
	}
	summary := ReadmeSummary(suite)
	if err := PatchREADME(path, summary); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "old") {
		t.Fatal("stale summary still present")
	}
	if !strings.Contains(text, "search_code") {
		t.Fatalf("expected tool row in readme: %s", text)
	}
}
