// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func TestCompactSymbolsLeadOmitsReferencesWhenGated(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = allCapabilities()
	cfg.Capabilities.SearchSymbolReferences = false

	lead := compactSymbolsLead(cfg)
	if strings.Contains(strings.ToLower(lead), "reference") {
		t.Fatalf("lead = %q, should not mention references", lead)
	}
	summary := compactToolSummary("opengrok_symbols", compactSymbolsOperations(cfg))
	if strings.Contains(strings.ToLower(summary), "reference") {
		t.Fatalf("summary = %q, should not mention references", summary)
	}
}

func TestCompactSymbolsLeadIncludesReferencesWhenEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = allCapabilities()

	lead := compactSymbolsLead(cfg)
	if !strings.Contains(strings.ToLower(lead), "reference") {
		t.Fatalf("lead = %q, want references mentioned", lead)
	}
}

func TestCompactProjectScopeNoteNoDefault(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultProject = ""
	note := compactProjectScopeNote(cfg)
	if !strings.Contains(note, "no default project") {
		t.Fatalf("note = %q, want no-default guidance", note)
	}
}

func TestSummarizeSymbolsSummaryOnlyDefinitions(t *testing.T) {
	got := summarizeSymbolsSummary([]string{"definitions", "list"})
	if strings.Contains(strings.ToLower(got), "reference") {
		t.Fatalf("summary = %q, should not mention references", got)
	}
	if !strings.Contains(got, "definitions") || !strings.Contains(got, "structural listing") {
		t.Fatalf("summary = %q, want definitions and listing", got)
	}
}

func TestSummarizeSymbolsSummaryReferencesOnly(t *testing.T) {
	got := summarizeSymbolsSummary([]string{"references"})
	if !strings.Contains(got, "references") {
		t.Fatalf("summary = %q, want references", got)
	}
}

func TestInterfaceVersionConstant(t *testing.T) {
	if config.ErgonomicsInterfaceVersion == "" {
		t.Fatal("ErgonomicsInterfaceVersion is empty")
	}
}
