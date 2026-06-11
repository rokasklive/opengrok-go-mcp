// SPDX-License-Identifier: Apache-2.0

// update-eval-readme patches README.md from evals/report.json and evals/token_report.json,
// showing Δ vs committed baselines in evals/baselines/.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rokasklive/opengrok-go-mcp/evals"
)

func main() {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	evalsDir := filepath.Join(root, "evals")
	baselineDir := filepath.Join(evalsDir, "baselines")

	suite, err := readSuite(filepath.Join(evalsDir, "report.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "contract eval: %v (run go test ./evals/ -run TestEvalSuite first)\n", err)
		os.Exit(1)
	}

	token, err := readTokenReport(filepath.Join(evalsDir, "token_report.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "token benchmark: %v (run go test ./evals/ -run TestTokenBenchmark first)\n", err)
		os.Exit(1)
	}

	prevSuite := evals.ReadBaseline(filepath.Join(baselineDir, "report.json"))
	prevToken := evals.ReadTokenBaseline(filepath.Join(baselineDir, "token_report.json"))

	readmePath := filepath.Join(root, "README.md")
	summary := evals.ReadmeSummary(suite, token, prevSuite, prevToken)
	if err := evals.PatchREADME(readmePath, summary); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readSuite(path string) (evals.SuiteResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return evals.SuiteResult{}, err
	}
	var suite evals.SuiteResult
	if err := json.Unmarshal(raw, &suite); err != nil {
		return evals.SuiteResult{}, fmt.Errorf("parse report.json: %w", err)
	}
	return suite, nil
}

func readTokenReport(path string) (evals.TokenBenchmarkResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return evals.TokenBenchmarkResult{}, err
	}
	var token evals.TokenBenchmarkResult
	if err := json.Unmarshal(raw, &token); err != nil {
		return evals.TokenBenchmarkResult{}, fmt.Errorf("parse token_report.json: %w", err)
	}
	return token, nil
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			wd, _ := os.Getwd()
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
