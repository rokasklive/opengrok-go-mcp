// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"fmt"
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

	if len(result.Runs) < 16 {
		t.Fatalf("runs = %d, want at least 16 (4 scenarios × compact rich+economy + full + gateway)", len(result.Runs))
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
		if !run.SuccessfulTask {
			t.Errorf("scenario %s surface %s: successful_task should be true", run.ScenarioID, run.Surface)
		}
		if run.CostPerSuccessBytes <= 0 || run.CostPerSuccessTokens <= 0 {
			t.Errorf("scenario %s surface %s: cost per successful task should be > 0", run.ScenarioID, run.Surface)
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
		want := 4
		if s == surfaceCompact {
			want = 8
		}
		if surfacesSeen[s] != want {
			t.Errorf("surface %s runs = %d, want %d", s, surfacesSeen[s], want)
		}
	}

	ceiling := result.CompactListToolsCeilingBytes
	if override := os.Getenv("OPENGROK_MCP_EVAL_LISTTOOLS_CEILING"); override != "" {
		fmt.Sscanf(override, "%d", &ceiling)
	}
	if ceiling <= 0 {
		ceiling = 14497 // current compact list_tools ~14213 + 2%
	}
	maxCompactListTools := 0
	for _, run := range result.Runs {
		if run.Surface != surfaceCompact {
			continue
		}
		if run.ListToolsBytes > maxCompactListTools {
			maxCompactListTools = run.ListToolsBytes
		}
	}
	if maxCompactListTools > ceiling {
		t.Logf("secondary anomaly: compact list_tools_bytes %d exceeds ceiling %d (delta %d)", maxCompactListTools, ceiling, maxCompactListTools-ceiling)
	}

	schemaCeilings := result.CompactSchemaCeilingBytes
	if len(schemaCeilings) == 0 {
		schemaCeilings = ReadTokenBaseline(filepath.Join(".", "baselines", "token_report.json")).CompactSchemaCeilingBytes
	}
	if len(schemaCeilings) > 0 {
		observed := map[string]int{}
		for _, run := range result.Runs {
			if run.Surface != surfaceCompact {
				continue
			}
			for tool, bytes := range run.SchemaBytesByTool {
				if bytes > observed[tool] {
					observed[tool] = bytes
				}
			}
		}
		for tool, ceilingBytes := range schemaCeilings {
			if got := observed[tool]; got > ceilingBytes {
				t.Logf("secondary anomaly: compact schema %s = %d bytes exceeds ceiling %d", tool, got, ceilingBytes)
			}
		}
	}

	richByScenario := map[string]int{}
	economyByScenario := map[string]int{}
	for _, run := range result.Runs {
		if run.Surface != surfaceCompact {
			continue
		}
		stepWarm := run.RequestBytes + run.ResponseBytes
		if run.AgentProfile == "rich" {
			richByScenario[run.ScenarioID] = stepWarm
		} else {
			economyByScenario[run.ScenarioID] = stepWarm
		}
	}
	for scenarioID, richWarm := range richByScenario {
		economyWarm, ok := economyByScenario[scenarioID]
		if !ok || richWarm <= 0 {
			continue
		}
		reduction := float64(richWarm-economyWarm) / float64(richWarm) * 100
		if reduction < 15 {
			t.Errorf("scenario %s economy warm reduction %.1f%% < 15%% (rich=%d economy=%d)", scenarioID, reduction, richWarm, economyWarm)
		}
	}

	for _, run := range result.Runs {
		if run.ScenarioID == "file-exploration" && run.Surface == surfaceCompact && run.AgentProfile == "" {
			if len(run.SkippedSteps) > 0 {
				t.Fatalf("unexpected skipped steps for compact file-exploration: %v", run.SkippedSteps)
			}
		}
	}
}
