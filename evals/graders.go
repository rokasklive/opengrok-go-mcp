// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TrajectoryCase is one deterministic agent-workflow replay with graders.
type TrajectoryCase struct {
	ID                 string             `json:"id"`
	Description        string             `json:"description"`
	ToolSurface        string             `json:"tool_surface"`
	AgentProfile       string             `json:"agent_profile,omitempty"`
	GateReferenceProbe bool               `json:"gate_reference_probe,omitempty"`
	Steps              []TrajectoryStep   `json:"steps"`
	Graders            []TrajectoryGrader `json:"graders"`
}

type TrajectoryStep struct {
	Tool     string         `json:"tool,omitempty"`
	Resource string         `json:"resource,omitempty"`
	Args     map[string]any `json:"args"`
}

type TrajectoryGrader struct {
	Type         string   `json:"type"`
	Tools        []string `json:"tools,omitempty"`
	Value        string   `json:"value,omitempty"`
	Field        string   `json:"field,omitempty"`
	StepIndex    int      `json:"step_index,omitempty"`
	Task         string   `json:"task,omitempty"`
	ExpectTool   string   `json:"expect_tool,omitempty"`
	Tool         string   `json:"tool,omitempty"`
	Operation    string   `json:"operation,omitempty"`
	Capability   string   `json:"capability,omitempty"`
	Substring    string   `json:"substring,omitempty"`
	ExpectAbsent bool     `json:"expect_absent,omitempty"`
}

type trajectoryStepResult struct {
	tool     string
	args     map[string]any
	response map[string]any
	rawText  string
}

func gradeTrajectory(tc TrajectoryCase, steps []trajectoryStepResult) error {
	for i, gr := range tc.Graders {
		if err := runGrader(gr, steps); err != nil {
			return fmt.Errorf("grader %d (%s): %w", i, gr.Type, err)
		}
	}
	return nil
}

func runGrader(gr TrajectoryGrader, steps []trajectoryStepResult) error {
	switch gr.Type {
	case "tool_sequence":
		if len(gr.Tools) == 0 {
			return fmt.Errorf("tools required")
		}
		if len(steps) < len(gr.Tools) {
			return fmt.Errorf("got %d steps, want prefix length %d", len(steps), len(gr.Tools))
		}
		for i, want := range gr.Tools {
			if steps[i].tool != want {
				return fmt.Errorf("step %d tool = %q, want %q", i, steps[i].tool, want)
			}
		}
		return nil
	case "warning_code":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		codes := warningCodes(steps[idx].response)
		for _, code := range codes {
			if code == gr.Value {
				return nil
			}
		}
		return fmt.Errorf("warnings %v missing code %q", codes, gr.Value)
	case "citation_present":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		field := gr.Field
		if field == "" {
			field = "results.citation.url"
		}
		if !allItemsHavePath(steps[idx].response, field) {
			return fmt.Errorf("not all items have %q", field)
		}
		return nil
	case "field_present":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		if _, ok := getPath(steps[idx].response, gr.Field); !ok {
			return fmt.Errorf("field %q missing at step %d", gr.Field, idx)
		}
		return nil
	case "field_eq":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		got, ok := getPath(steps[idx].response, gr.Field)
		if !ok {
			return fmt.Errorf("field %q missing", gr.Field)
		}
		gotStr := fmt.Sprint(got)
		if gr.ExpectAbsent {
			if gotStr == gr.Value {
				return fmt.Errorf("%s = %q, want absent or different", gr.Field, gotStr)
			}
			return nil
		}
		if gotStr != gr.Value {
			return fmt.Errorf("%s = %v, want %q", gr.Field, got, gr.Value)
		}
		return nil
	case "description_cuj":
		got := ResolveDescriptionCUJ(gr.Task)
		if got != gr.ExpectTool {
			return fmt.Errorf("task %q resolved to %q, want %q", gr.Task, got, gr.ExpectTool)
		}
		return nil
	case "manifest_operation_absent":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		if manifestHasOperation(steps[idx].response, gr.Tool, gr.Operation) {
			return fmt.Errorf("tool %q still lists operation %q", gr.Tool, gr.Operation)
		}
		return nil
	case "gated_capability":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		if !manifestHasGatedCapability(steps[idx].response, gr.Capability) {
			return fmt.Errorf("gated[] missing capability %q", gr.Capability)
		}
		return nil
	case "summary_excludes":
		idx := gr.StepIndex
		if idx < 0 || idx >= len(steps) {
			return fmt.Errorf("step_index %d out of range", idx)
		}
		summary := manifestToolSummary(steps[idx].response, gr.Tool)
		if strings.Contains(strings.ToLower(summary), strings.ToLower(gr.Substring)) {
			return fmt.Errorf("summary for %q contains %q: %q", gr.Tool, gr.Substring, summary)
		}
		return nil
	default:
		return fmt.Errorf("unknown grader type %q", gr.Type)
	}
}

func manifestTools(m map[string]any) []map[string]any {
	raw, ok := m["tools"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if ok {
			out = append(out, obj)
		}
	}
	return out
}

func manifestToolByName(m map[string]any, name string) (map[string]any, bool) {
	for _, tool := range manifestTools(m) {
		if toolName, _ := tool["name"].(string); toolName == name {
			return tool, true
		}
	}
	return nil, false
}

func manifestHasOperation(m map[string]any, toolName, operation string) bool {
	tool, ok := manifestToolByName(m, toolName)
	if !ok {
		return false
	}
	raw, ok := tool["operations"]
	if !ok {
		return false
	}
	arr, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		if op, ok := item.(string); ok && op == operation {
			return true
		}
	}
	return false
}

func manifestHasGatedCapability(m map[string]any, capability string) bool {
	raw, ok := m["gated"]
	if !ok {
		return false
	}
	arr, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if capName, _ := obj["capability"].(string); capName == capability {
			return true
		}
	}
	return false
}

func manifestToolSummary(m map[string]any, toolName string) string {
	tool, ok := manifestToolByName(m, toolName)
	if !ok {
		return ""
	}
	summary, _ := tool["summary"].(string)
	return summary
}

func warningCodes(m map[string]any) []string {
	raw, ok := m["warnings"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if code, ok := obj["code"].(string); ok && code != "" {
			out = append(out, code)
		}
	}
	return out
}

func allItemsHavePath(m map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	if len(parts) < 2 || parts[0] != "results" {
		v, ok := getPath(m, path)
		return ok && fmt.Sprint(v) != ""
	}
	subPath := strings.Join(parts[1:], ".")
	raw, ok := m["results"]
	if !ok {
		return false
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return false
	}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			return false
		}
		v, ok := getPath(obj, subPath)
		if !ok || fmt.Sprint(v) == "" {
			return false
		}
	}
	return true
}

func structuredFromToolResult(out any) map[string]any {
	switch v := out.(type) {
	case map[string]any:
		return v
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			return nil
		}
		return m
	}
}
