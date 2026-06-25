// SPDX-License-Identifier: Apache-2.0

package evals

import "time"

// Scenario is a surface-agnostic replay workflow loaded from testdata.
type Scenario struct {
	ID          string         `json:"id"`
	Description string         `json:"description"`
	Steps       []ScenarioStep `json:"steps"`
}

// ScenarioStep is one canonical operation in a scenario.
type ScenarioStep struct {
	Op   string         `json:"op"`
	Args map[string]any `json:"args"`
}

// SurfaceRun metrics for one scenario replay on one tool surface.
type SurfaceRun struct {
	ScenarioID              string         `json:"scenario_id"`
	Surface                 string         `json:"surface"`
	AgentProfile            string         `json:"agent_profile,omitempty"`
	ListToolsBytes          int            `json:"list_tools_bytes"`
	SchemaBytesByTool       map[string]int `json:"schema_bytes_by_tool"`
	DiscoverBytes           int            `json:"discover_bytes"`
	RequestBytes            int            `json:"request_bytes"`
	ResponseBytes           int            `json:"response_bytes"`
	ResponseTextBytes       int            `json:"response_text_bytes"`
	ResponseStructuredBytes int            `json:"response_structured_bytes"`
	LargestResponseBytes    int            `json:"largest_response_bytes"`
	TotalColdBytes          int            `json:"total_cold_bytes"`
	TotalWarmBytes          int            `json:"total_warm_bytes"`
	EstTokensCold           int            `json:"est_tokens_cold"`
	EstTokensWarm           int            `json:"est_tokens_warm"`
	SuccessfulTask          bool           `json:"successful_task"`
	CostPerSuccessBytes     int            `json:"cost_per_success_bytes"`
	CostPerSuccessTokens    int            `json:"cost_per_success_tokens"`
	CallCount               int            `json:"call_count"`
	SkippedSteps            []string       `json:"skipped_steps,omitempty"`
	LargestToolSchemaName   string         `json:"largest_tool_schema_name"`
	LargestToolSchemaBytes  int            `json:"largest_tool_schema_bytes"`
	LargestResponseStep     string         `json:"largest_response_step"`
}

// TokenBenchmarkResult is the full benchmark output.
type TokenBenchmarkResult struct {
	BenchmarkName                string         `json:"benchmark_name"`
	Mode                         string         `json:"mode"`
	Timestamp                    time.Time      `json:"timestamp"`
	Surfaces                     []string       `json:"surfaces"`
	ScenarioIDs                  []string       `json:"scenario_ids"`
	CompactListToolsCeilingBytes int            `json:"compact_list_tools_ceiling_bytes,omitempty"`
	CompactSchemaCeilingBytes    map[string]int `json:"compact_schema_ceiling_bytes,omitempty"`
	Runs                         []SurfaceRun   `json:"runs"`
}
