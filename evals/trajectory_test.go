// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTrajectorySuite(t *testing.T) {
	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}

	cases, err := loadTrajectoryCases(filepath.Join(testdataDir, "trajectory"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 3 {
		t.Fatalf("cases = %d, want at least 3", len(cases))
	}

	graderCount := 0
	for _, tc := range cases {
		graderCount += len(tc.Graders)
	}
	if graderCount < 8 {
		t.Fatalf("graders = %d, want at least 8", graderCount)
	}

	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			surface := tc.ToolSurface
			if surface == "" {
				surface = surfaceCompact
			}
			h, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{
				ToolSurface:        surface,
				AgentProfile:       tc.AgentProfile,
				GateReferenceProbe: tc.GateReferenceProbe,
			})
			if err != nil {
				t.Fatalf("start harness: %v", err)
			}
			defer h.Stop()

			steps, err := replayTrajectory(ctx, h, tc)
			if err != nil {
				t.Fatalf("replay: %v", err)
			}
			if err := gradeTrajectory(tc, steps); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func loadTrajectoryCases(dir string) ([]TrajectoryCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var cases []TrajectoryCase
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var fileCases []TrajectoryCase
		if err := json.Unmarshal(raw, &fileCases); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		cases = append(cases, fileCases...)
	}
	return cases, nil
}

func replayTrajectory(ctx context.Context, h *Harness, tc TrajectoryCase) ([]trajectoryStepResult, error) {
	out := make([]trajectoryStepResult, 0, len(tc.Steps))
	for _, step := range tc.Steps {
		if step.Resource != "" {
			result, err := h.Session().ReadResource(ctx, &mcp.ReadResourceParams{URI: step.Resource})
			if err != nil {
				return out, err
			}
			if len(result.Contents) == 0 {
				return out, fmt.Errorf("empty resource %q", step.Resource)
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(result.Contents[0].Text), &payload); err != nil {
				return out, fmt.Errorf("parse resource %q: %w", step.Resource, err)
			}
			out = append(out, trajectoryStepResult{
				tool:     step.Resource,
				args:     step.Args,
				response: payload,
			})
			continue
		}
		if step.Tool == "" {
			return out, fmt.Errorf("trajectory step missing tool or resource")
		}
		if !h.hasTool(step.Tool) {
			return out, fmt.Errorf("tool %q not registered", step.Tool)
		}
		result, callErr := h.Session().CallTool(ctx, &mcp.CallToolParams{
			Name:      step.Tool,
			Arguments: step.Args,
		})
		if callErr != nil {
			return out, callErr
		}
		if result != nil && result.IsError {
			return out, fmt.Errorf("tool error: %s", contentText(result))
		}
		out = append(out, trajectoryStepResult{
			tool:     step.Tool,
			args:     step.Args,
			response: structured(result),
			rawText:  contentText(result),
		})
	}
	return out, nil
}

func TestCapabilitiesResourceEval(t *testing.T) {
	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}

	for _, surface := range []string{surfaceCompact, surfaceFull} {
		t.Run(surface, func(t *testing.T) {
			h, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{ToolSurface: surface})
			if err != nil {
				t.Fatalf("start harness: %v", err)
			}
			defer h.Stop()

			result, err := h.Session().ReadResource(ctx, &mcp.ReadResourceParams{URI: "opengrok://capabilities"})
			if err != nil {
				t.Fatalf("ReadResource: %v", err)
			}
			if len(result.Contents) == 0 {
				t.Fatal("empty resource contents")
			}
			var report map[string]any
			if err := json.Unmarshal([]byte(result.Contents[0].Text), &report); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, ok := report["tools"]; !ok {
				t.Fatal("tools missing from capability manifest")
			}
			if report["tool_surface"] != surface {
				t.Fatalf("tool_surface = %v, want %s", report["tool_surface"], surface)
			}
		})
	}
}
