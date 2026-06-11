// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"encoding/json"
	"os"
)

// ReadTokenBaseline loads a prior TokenBenchmarkResult from path (empty if missing).
func ReadTokenBaseline(path string) TokenBenchmarkResult {
	var prev TokenBenchmarkResult
	raw, err := os.ReadFile(path)
	if err != nil {
		return prev
	}
	_ = json.Unmarshal(raw, &prev)
	return prev
}

func findSurfaceRun(runs []SurfaceRun, scenarioID, surface string) (SurfaceRun, bool) {
	for _, run := range runs {
		if run.ScenarioID == scenarioID && run.Surface == surface {
			return run, true
		}
	}
	return SurfaceRun{}, false
}

func warmEstTokens(run SurfaceRun) int {
	if run.EstTokensWarm > 0 {
		return run.EstTokensWarm
	}
	return estTokens(run.TotalWarmBytes)
}

func listToolsEstTokens(run SurfaceRun) int {
	return estTokens(run.ListToolsBytes)
}
