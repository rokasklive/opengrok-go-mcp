// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

var (
	harness   *Harness
	evalCases []EvalCase
)

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	ctx := context.Background()

	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		println("module root:", err.Error())
		return 1
	}
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		println("testdata dir:", err.Error())
		return 1
	}

	cases, err := loadCases(testdataDir)
	if err != nil {
		println("load cases:", err.Error())
		return 1
	}
	evalCases = cases

	h, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{ToolSurface: surfaceFull})
	if err != nil {
		println("harness start failed:", err.Error())
		return 1
	}
	defer h.Stop()
	harness = h

	return m.Run()
}

func TestEvalSuite(t *testing.T) {
	if harness == nil {
		t.Fatal("harness not initialized")
	}
	suite := harness.RunSuite(context.Background(), "direct-call-hermetic", evalCases)

	if err := WriteReports(suite, "."); err != nil {
		t.Fatalf("write reports: %v", err)
	}

	t.Logf("score %.1f%% coverage@%d %.2f p95 %s passed=%d failed=%d skipped=%d",
		suite.Score*100, suite.Total, suite.CoverageK, suite.LatencyP95, suite.Passed, suite.Failed, suite.Skipped)

	if suite.Failed > 0 {
		for _, r := range suite.Results {
			if !r.Skipped && !r.Passed {
				t.Errorf("case %s (%s) failed: errors=%v checks=%v", r.CaseID, r.Tool, r.Errors, r.Checks)
			}
		}
	}
}

func TestEvalSuiteLive(t *testing.T) {
	if os.Getenv("OPENGROK_MCP_LIVE_EVAL") != "1" {
		t.Skip("set OPENGROK_MCP_LIVE_EVAL=1 to run against a live OpenGrok instance")
	}
	if os.Getenv("OPENGROK_MCP_BASE_URL") == "" {
		t.Skip("OPENGROK_MCP_BASE_URL required for live eval")
	}
	t.Skip("live eval smoke not implemented; use hermetic TestEvalSuite for CI")
}
