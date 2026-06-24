// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "testing"

func TestMaybeWarnExpansionBudget(t *testing.T) {
	warnings := newWarningSet()
	expansion := &ExpansionDiagnostics{ExpandedContextBytes: 600}
	results := []Result{
		{Snippet: strPtr("short"), Context: &ResultContext{Content: stringsRepeat("x", 600)}},
	}
	maybeWarnExpansionBudget(warnings, expansion, results)
	fields := warnings.fields()
	if len(fields.Warnings) == 0 {
		t.Fatal("expected EXPANSION_BUDGET_HIGH warning")
	}
	if fields.Warnings[0].Code != warnExpansionBudgetHigh {
		t.Fatalf("code = %q, want %q", fields.Warnings[0].Code, warnExpansionBudgetHigh)
	}
}

func TestMaybeWarnExpansionBudgetBelowThreshold(t *testing.T) {
	warnings := newWarningSet()
	expansion := &ExpansionDiagnostics{ExpandedContextBytes: 100}
	results := []Result{
		{Snippet: strPtr(stringsRepeat("y", 900)), Context: &ResultContext{Content: stringsRepeat("x", 100)}},
	}
	maybeWarnExpansionBudget(warnings, expansion, results)
	if len(warnings.fields().Warnings) != 0 {
		t.Fatalf("warnings = %+v, want none below threshold", warnings.fields().Warnings)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = s[0]
	}
	return string(out)
}
