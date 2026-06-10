// SPDX-License-Identifier: Apache-2.0

package evals

import "time"

// EvalCase is one row of a testdata file. In direct-call mode the harness calls Tool with
// Input verbatim; Expected describes what the response must satisfy.
type EvalCase struct {
	ID          string         `json:"id"`
	Tool        string         `json:"tool"`
	Description string         `json:"description"`
	Input       map[string]any `json:"input"`
	Capability  string         `json:"capability_gate"`

	Expected Expected `json:"expected"`
}

// Expected is promoted from the brief's anonymous struct so runner/report can pass it around.
type Expected struct {
	ToolCalled   string         `json:"tool_called"`
	Arguments    map[string]any `json:"arguments"`
	ResultChecks []ResultCheck  `json:"result_checks"`
	LatencyMS    int            `json:"latency_ms,omitempty"`
}

// ResultCheck is one assertion against a tool result.
type ResultCheck struct {
	Type  string `json:"type"`
	Field string `json:"field,omitempty"`
	Min   int    `json:"min,omitempty"`
	Max   int    `json:"max,omitempty"`
}

type CheckResult struct {
	Type    string `json:"type"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

type EvalResult struct {
	CaseID     string        `json:"case_id"`
	Tool       string        `json:"tool"`
	Passed     bool          `json:"passed"`
	Skipped    bool          `json:"skipped,omitempty"`
	Score      float64       `json:"score"`
	Latency    time.Duration `json:"latency"`
	ToolCalled string        `json:"tool_called"`
	Errors     []string      `json:"errors,omitempty"`
	Checks     []CheckResult `json:"checks"`
}

type SuiteResult struct {
	SuiteName  string             `json:"suite_name"`
	Mode       string             `json:"mode"`
	Timestamp  time.Time          `json:"timestamp"`
	Total      int                `json:"total"`
	Passed     int                `json:"passed"`
	Failed     int                `json:"failed"`
	Skipped    int                `json:"skipped"`
	Score      float64            `json:"score"`
	CoverageK  float64            `json:"coverage_k"`
	PerTool    map[string]float64 `json:"per_tool_score"`
	LatencyP50 time.Duration      `json:"latency_p50"`
	LatencyP95 time.Duration      `json:"latency_p95"`
	LatencyP99 time.Duration      `json:"latency_p99"`
	Results    []EvalResult       `json:"results"`
}
