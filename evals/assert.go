// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func contentText(out *mcp.CallToolResult) string {
	if out == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range out.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func structured(out *mcp.CallToolResult) map[string]any {
	if out == nil || out.StructuredContent == nil {
		return nil
	}
	raw, err := json.Marshal(out.StructuredContent)
	if err != nil {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func getPath(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			if len(v) == 0 {
				return nil, false
			}
			mm, ok := v[0].(map[string]any)
			if !ok {
				return nil, false
			}
			next, ok := mm[part]
			if !ok {
				return nil, false
			}
			cur = next
		default:
			return nil, false
		}
	}
	return cur, true
}

func arrayLen(v any) (int, bool) {
	if a, ok := v.([]any); ok {
		return len(a), true
	}
	return 0, false
}

func runChecks(tc EvalCase, out *mcp.CallToolResult, callErr error, latency time.Duration) []CheckResult {
	var results []CheckResult
	for _, chk := range tc.Expected.ResultChecks {
		results = append(results, runOne(tc, chk, out, callErr, latency))
	}
	if tc.Expected.LatencyMS > 0 {
		results = append(results, runLatencyBudget(tc, tc.Expected.LatencyMS, latency))
	}
	return results
}

func runOne(tc EvalCase, chk ResultCheck, out *mcp.CallToolResult, callErr error, latency time.Duration) CheckResult {
	r := CheckResult{Type: chk.Type}
	switch chk.Type {
	case "no_error":
		r.Passed = callErr == nil && (out == nil || !out.IsError)
		if !r.Passed {
			r.Message = fmt.Sprintf("tool returned an error: callErr=%v isError=%v text=%s", callErr, out != nil && out.IsError, contentText(out))
		}
	case "tool_called":
		r.Passed = tc.Tool == tc.Expected.ToolCalled || tc.Expected.ToolCalled == ""
		if !r.Passed {
			r.Message = fmt.Sprintf("called %s, expected %s", tc.Tool, tc.Expected.ToolCalled)
		}
	case "arg_match":
		r.Passed = argsEqual(tc.Input, tc.Expected.Arguments)
		if !r.Passed {
			r.Message = "input arguments do not match expected.arguments"
		}
	case "field_present":
		_, ok := getPath(structured(out), chk.Field)
		r.Passed = ok
		if !ok {
			r.Message = fmt.Sprintf("field %q not present", chk.Field)
		}
	case "has_results":
		field := chk.Field
		if field == "" {
			field = "results"
		}
		v, ok := getPath(structured(out), field)
		n, isArr := arrayLen(v)
		min := chk.Min
		if min == 0 {
			min = 1
		}
		r.Passed = ok && isArr && n >= min
		if !r.Passed {
			r.Message = fmt.Sprintf("%s: got %d results, want >= %d", field, n, min)
		}
	case "latency_ms":
		max := chk.Max
		if max == 0 {
			max = 5000
		}
		r.Passed = latency <= time.Duration(max)*time.Millisecond
		if !r.Passed {
			r.Message = fmt.Sprintf("latency %s exceeds budget %dms", latency.Round(time.Millisecond), max)
		}
	default:
		r.Message = fmt.Sprintf("unknown check type %q", chk.Type)
	}
	return r
}

func runLatencyBudget(tc EvalCase, budgetMS int, latency time.Duration) CheckResult {
	r := CheckResult{Type: "latency_ms"}
	budget := time.Duration(budgetMS) * time.Millisecond
	r.Passed = latency <= budget
	if !r.Passed {
		r.Message = fmt.Sprintf("latency %s exceeds budget %s", latency.Round(time.Millisecond), budget)
	}
	return r
}

func argsEqual(a, b map[string]any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func ExpectNoError(t *testing.T, out *mcp.CallToolResult, callErr error) {
	t.Helper()
	if callErr != nil || (out != nil && out.IsError) {
		t.Fatalf("expected no error, got callErr=%v isError=%v: %s", callErr, out != nil && out.IsError, contentText(out))
	}
}

func ExpectFieldPresent(t *testing.T, out *mcp.CallToolResult, path string) {
	t.Helper()
	if _, ok := getPath(structured(out), path); !ok {
		t.Fatalf("field %q not present in structured output", path)
	}
}

func ExpectMinResults(t *testing.T, out *mcp.CallToolResult, field string, min int) {
	t.Helper()
	v, _ := getPath(structured(out), field)
	if n, ok := arrayLen(v); !ok || n < min {
		t.Fatalf("%s: got %d results, want >= %d", field, n, min)
	}
}
