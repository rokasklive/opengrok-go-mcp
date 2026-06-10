// SPDX-License-Identifier: Apache-2.0

// update-eval-readme patches README.md from evals/report.json (run eval suite first).
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

	reportPath := filepath.Join(root, "evals", "report.json")
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v (run go test ./evals/ -run TestEvalSuite first)\n", reportPath, err)
		os.Exit(1)
	}

	var suite evals.SuiteResult
	if err := json.Unmarshal(raw, &suite); err != nil {
		fmt.Fprintf(os.Stderr, "parse report.json: %v\n", err)
		os.Exit(1)
	}

	readmePath := filepath.Join(root, "README.md")
	if err := evals.PatchREADME(readmePath, evals.ReadmeSummary(suite)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
