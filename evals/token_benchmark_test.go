// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTokenBenchmark(t *testing.T) {
	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}

	result, err := RunBenchmark(ctx, moduleRoot, testdataDir, ".")
	if err != nil {
		t.Fatalf("RunBenchmark: %v", err)
	}

	if len(result.Runs) < 12 {
		t.Fatalf("runs = %d, want at least 12 (4 scenarios × 3 surfaces)", len(result.Runs))
	}

	jsonPath := filepath.Join(".", "token_report.json")
	mdPath := filepath.Join(".", "token_report.md")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("token_report.json missing: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("token_report.md missing: %v", err)
	}

	surfacesSeen := map[string]int{}
	for _, run := range result.Runs {
		surfacesSeen[run.Surface]++
		if run.ListToolsBytes <= 0 {
			t.Errorf("scenario %s surface %s: list_tools_bytes should be > 0", run.ScenarioID, run.Surface)
		}
		if run.TotalColdBytes <= 0 {
			t.Errorf("scenario %s surface %s: total_cold_bytes should be > 0", run.ScenarioID, run.Surface)
		}
		if run.Surface == surfaceGateway && run.DiscoverBytes <= 0 {
			t.Errorf("gateway run %s: discover_bytes should be > 0", run.ScenarioID)
		}
		if run.Surface != surfaceGateway && run.TotalColdBytes != run.TotalWarmBytes {
			t.Errorf("non-gateway cold/warm should match for %s/%s", run.ScenarioID, run.Surface)
		}
		if run.Surface == surfaceGateway && run.TotalWarmBytes >= run.TotalColdBytes {
			t.Errorf("gateway warm should be less than cold for %s", run.ScenarioID)
		}
		if run.LargestToolSchemaName == "" || run.LargestToolSchemaBytes <= 0 {
			t.Errorf("missing largest schema for %s/%s", run.ScenarioID, run.Surface)
		}
	}

	for _, s := range allSurfaces {
		if surfacesSeen[s] != 4 {
			t.Errorf("surface %s runs = %d, want 4", s, surfacesSeen[s])
		}
	}

	// Compact file-exploration should resolve files.list onto opengrok_projects.files
	for _, run := range result.Runs {
		if run.ScenarioID == "file-exploration" && run.Surface == surfaceCompact {
			if len(run.SkippedSteps) > 0 {
				t.Fatalf("unexpected skipped steps for compact file-exploration: %v", run.SkippedSteps)
			}
		}
	}
}
