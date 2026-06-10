// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func loadCases(dir string) ([]EvalCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read testdata dir: %w", err)
	}

	var cases []EvalCase
	seen := map[string]struct{}{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.Name() == "manifest.json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var fileCases []EvalCase
		if err := json.Unmarshal(raw, &fileCases); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}

		for i := range fileCases {
			tc := fileCases[i]
			if err := validateCase(tc, path); err != nil {
				return nil, err
			}
			if _, ok := seen[tc.ID]; ok {
				return nil, fmt.Errorf("%s: duplicate case id %q", path, tc.ID)
			}
			seen[tc.ID] = struct{}{}
			cases = append(cases, tc)
		}
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no eval cases found in %s", dir)
	}
	return cases, nil
}

func validateCase(tc EvalCase, path string) error {
	if strings.TrimSpace(tc.ID) == "" {
		return fmt.Errorf("%s: case missing id", path)
	}
	if strings.TrimSpace(tc.Tool) == "" {
		return fmt.Errorf("%s: case %q missing tool", path, tc.ID)
	}
	if len(tc.Expected.ResultChecks) == 0 {
		return fmt.Errorf("%s: case %q missing result_checks", path, tc.ID)
	}
	for _, chk := range tc.Expected.ResultChecks {
		switch chk.Type {
		case "no_error", "tool_called", "arg_match", "field_present", "has_results", "latency_ms":
		default:
			return fmt.Errorf("%s: case %q unknown check type %q", path, tc.ID, chk.Type)
		}
	}
	return nil
}

func (h *Harness) RunSuite(ctx context.Context, name string, cases []EvalCase) SuiteResult {
	results := make([]EvalResult, 0, len(cases))
	for _, tc := range cases {
		results = append(results, h.RunCase(ctx, tc))
	}
	return aggregate(name, results)
}

func (h *Harness) RunCase(ctx context.Context, tc EvalCase) EvalResult {
	res := EvalResult{CaseID: tc.ID, Tool: tc.Tool}

	if !h.hasTool(tc.Tool) {
		res.Skipped = true
		return res
	}

	start := time.Now()
	out, callErr := h.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tc.Tool,
		Arguments: tc.Input,
	})
	res.Latency = time.Since(start)
	res.ToolCalled = tc.Tool

	if callErr != nil {
		res.Errors = append(res.Errors, callErr.Error())
	}

	res.Checks = runChecks(tc, out, callErr, res.Latency)
	passed := callErr == nil
	for _, c := range res.Checks {
		if !c.Passed {
			passed = false
		}
	}
	res.Passed = passed
	res.Score = scoreChecks(res.Checks)
	return res
}

func scoreChecks(cs []CheckResult) float64 {
	if len(cs) == 0 {
		return 0
	}
	var ok int
	for _, c := range cs {
		if c.Passed {
			ok++
		}
	}
	return float64(ok) / float64(len(cs))
}
