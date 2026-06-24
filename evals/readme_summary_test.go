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
	token := TokenBenchmarkResult{
		Mode:        "deterministic-replay",
		Timestamp:   time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		ScenarioIDs: []string{"text-search-and-read"},
		Runs: []SurfaceRun{
			{ScenarioID: "text-search-and-read", Surface: "full", ListToolsBytes: 40000, EstTokensWarm: 14000, SchemaBytesByTool: map[string]int{"search_code": 1, "read_file": 1}},
			{ScenarioID: "text-search-and-read", Surface: "compact", ListToolsBytes: 8000, EstTokensWarm: 3500, SchemaBytesByTool: map[string]int{"opengrok_search": 1, "opengrok_read": 1}},
			{ScenarioID: "text-search-and-read", Surface: "gateway", ListToolsBytes: 1000, EstTokensWarm: 1900, SchemaBytesByTool: map[string]int{"opengrok_discover": 1, "opengrok_call": 1}},
		},
	}
	prevToken := TokenBenchmarkResult{
		Runs: []SurfaceRun{
			{ScenarioID: "text-search-and-read", Surface: "full", EstTokensWarm: 13000},
		},
		Timestamp: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
	}
	summary := ReadmeSummary(suite, token, SuiteResult{}, prevToken)
	if err := PatchREADME(path, summary); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, readmeMarkerStart+"\nold\n"+readmeMarkerEnd) {
		t.Fatal("stale summary still present")
	}
	if !strings.Contains(text, "search_code") {
		t.Fatalf("expected contract tool row in readme: %s", text)
	}
	if !strings.Contains(text, "Token economy benchmark") {
		t.Fatalf("expected token section in readme: %s", text)
	}
	if !strings.Contains(text, "Δ +1.0k") {
		t.Fatalf("expected warm token delta in readme: %s", text)
	}
}

func TestFormatEstTokens(t *testing.T) {
	if formatEstTokens(253) != "253" {
		t.Fatal("small values unchanged")
	}
	if formatEstTokens(1911) != "1.9k" {
		t.Fatal("thousands with one decimal")
	}
	if formatEstTokens(14000) != "14k" {
		t.Fatal("ten thousands round")
	}
}
