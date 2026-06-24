// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

var evalCases []EvalCase

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
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

	return m.Run()
}

func TestEvalSuite(t *testing.T) {
	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(evalCases) == 0 {
		cases, err := loadCases(testdataDir)
		if err != nil {
			t.Fatal(err)
		}
		evalCases = cases
	}

	for _, surface := range []string{surfaceFull, surfaceCompact} {
		t.Run(surface, func(t *testing.T) {
			h, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{ToolSurface: surface})
			if err != nil {
				t.Fatalf("start harness: %v", err)
			}
			defer h.Stop()

			cases := casesForSurface(surface, evalCases)
			suite := h.RunSuite(ctx, "direct-call-hermetic-"+surface, cases)

			if surface == surfaceFull {
				if err := WriteReports(suite, "."); err != nil {
					t.Fatalf("write reports: %v", err)
				}
			}

			t.Logf("[%s] score %.1f%% passed=%d failed=%d skipped=%d",
				surface, suite.Score*100, suite.Passed, suite.Failed, suite.Skipped)

			if suite.Failed > 0 {
				for _, r := range suite.Results {
					if !r.Skipped && !r.Passed {
						t.Errorf("case %s (%s) failed: errors=%v checks=%v", r.CaseID, r.Tool, r.Errors, r.Checks)
					}
				}
			}
		})
	}
}

func casesForSurface(surface string, base []EvalCase) []EvalCase {
	if surface == surfaceFull {
		return base
	}
	out := make([]EvalCase, 0, len(base))
	for _, tc := range base {
		adapted, ok := adaptEvalCaseForCompact(tc)
		if ok {
			out = append(out, adapted)
		}
	}
	return out
}

func TestEvalSuiteLive(t *testing.T) {
	if os.Getenv("OPENGROK_MCP_LIVE_EVAL") != "1" {
		t.Skip("set OPENGROK_MCP_LIVE_EVAL=1 to run against a live OpenGrok instance")
	}
	if os.Getenv("OPENGROK_MCP_BASE_URL") == "" {
		t.Skip("OPENGROK_MCP_BASE_URL required for live eval")
	}
}
