// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const gatewayDiscoverTool = "opengrok_discover"

// RunBenchmark replays all scenarios on each tool surface and returns token metrics.
func RunBenchmark(ctx context.Context, moduleRoot, testdataDir, reportDir string) (TokenBenchmarkResult, error) {
	scenarioDir := filepath.Join(testdataDir, "scenarios")
	scenarios, err := loadScenarios(scenarioDir)
	if err != nil {
		return TokenBenchmarkResult{}, err
	}

	result := TokenBenchmarkResult{
		BenchmarkName: "token-economy-hermetic",
		Mode:          "deterministic-replay",
		Timestamp:     time.Now(),
		Surfaces:      append([]string{}, allSurfaces...),
	}

	for _, sc := range scenarios {
		result.ScenarioIDs = append(result.ScenarioIDs, sc.ID)
	}

	for _, surface := range allSurfaces {
		h, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{ToolSurface: surface})
		if err != nil {
			return TokenBenchmarkResult{}, fmt.Errorf("start harness %s: %w", surface, err)
		}

		listToolsBytes := countListToolsBytes(h.ListedTools())
		schemaByTool := countSchemaByTool(h.ListedTools())
		discoverBytes := 0
		if surface == surfaceGateway && h.hasTool(gatewayDiscoverTool) {
			out, callErr := h.Session().CallTool(ctx, &mcp.CallToolParams{Name: gatewayDiscoverTool})
			if callErr == nil {
				text, structured := countCallToolResponse(out)
				discoverBytes = text + structured
			}
		}

		for _, sc := range scenarios {
			run, err := runScenario(ctx, h, surface, sc, listToolsBytes, schemaByTool, discoverBytes)
			if err != nil {
				h.Stop()
				return TokenBenchmarkResult{}, fmt.Errorf("scenario %s surface %s: %w", sc.ID, surface, err)
			}
			result.Runs = append(result.Runs, run)
		}

		h.Stop()
	}

	if reportDir != "" {
		if err := WriteTokenReports(result, reportDir); err != nil {
			return result, err
		}
	}
	return result, nil
}

func runScenario(
	ctx context.Context,
	h *Harness,
	surface string,
	sc Scenario,
	listToolsBytes int,
	schemaByTool map[string]int,
	discoverBytes int,
) (SurfaceRun, error) {
	run := SurfaceRun{
		ScenarioID:             sc.ID,
		Surface:                surface,
		ListToolsBytes:         listToolsBytes,
		SchemaBytesByTool:      schemaByTool,
		DiscoverBytes:          discoverBytes,
		SkippedSteps:           []string{},
		LargestResponseStep:    "",
		LargestToolSchemaName:  "",
		LargestToolSchemaBytes: 0,
		LargestResponseBytes:   0,
	}
	run.LargestToolSchemaName, run.LargestToolSchemaBytes = largestSchema(schemaByTool)

	var stepReq, stepResp, stepText, stepStructured int

	for i, step := range sc.Steps {
		resolved, err := Resolve(surface, step.Op, step.Args)
		if err != nil {
			return run, err
		}
		if resolved.Skipped {
			run.SkippedSteps = append(run.SkippedSteps, step.Op+": "+resolved.SkipReason)
			continue
		}
		if !h.hasTool(resolved.Tool) {
			run.SkippedSteps = append(run.SkippedSteps, step.Op+": tool "+resolved.Tool+" not registered")
			continue
		}

		reqBytes := countCallToolRequest(resolved.Tool, resolved.Arguments)
		out, callErr := h.Session().CallTool(ctx, &mcp.CallToolParams{
			Name:      resolved.Tool,
			Arguments: resolved.Arguments,
		})
		if callErr != nil {
			return run, fmt.Errorf("step %d op %s: %w", i, step.Op, callErr)
		}
		if out != nil && out.IsError {
			return run, fmt.Errorf("step %d op %s: tool error %s", i, step.Op, contentText(out))
		}

		textBytes, structuredBytes := countCallToolResponse(out)
		respBytes := textBytes + structuredBytes

		stepReq += reqBytes
		stepResp += respBytes
		stepText += textBytes
		stepStructured += structuredBytes
		run.CallCount++

		if respBytes > run.LargestResponseBytes {
			run.LargestResponseBytes = respBytes
			run.LargestResponseStep = fmt.Sprintf("step:%d:%s", i, step.Op)
		}
	}

	run.RequestBytes = stepReq
	run.ResponseBytes = stepResp
	run.ResponseTextBytes = stepText
	run.ResponseStructuredBytes = stepStructured

	run.TotalColdBytes = totalColdBytes(listToolsBytes, discoverBytes, stepReq, stepResp)
	run.TotalWarmBytes = totalWarmBytes(listToolsBytes, stepReq, stepResp)
	run.EstTokensCold = estTokens(run.TotalColdBytes)
	run.EstTokensWarm = estTokens(run.TotalWarmBytes)

	return run, nil
}
