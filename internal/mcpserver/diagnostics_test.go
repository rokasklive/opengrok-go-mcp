// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestDiagnosticsOmittedByDefault(t *testing.T) {
	output := searchOutputForDiagnostics(t, testConfig())

	fields := marshalObject(t, output)
	if _, ok := fields["diagnostics"]; ok {
		t.Fatalf("diagnostics field is present by default: %#v", fields["diagnostics"])
	}
}

func TestDiagnosticsPresentWhenEnvEnabled(t *testing.T) {
	t.Setenv("OPENGROK_MCP_DIAGNOSTICS", "true")
	cfg := config.FromEnv()
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	cfg.DefaultProject = "platform"

	output := searchOutputForDiagnostics(t, cfg)

	fields := marshalObject(t, output)
	raw, ok := fields["diagnostics"]
	if !ok {
		t.Fatal("diagnostics field is absent when OPENGROK_MCP_DIAGNOSTICS=true")
	}
	diagnostics, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("diagnostics field = %#v, want object", raw)
	}
	if diagnostics["offset_used"] != float64(0) {
		t.Fatalf("offset_used = %#v, want 0", diagnostics["offset_used"])
	}
}

func searchOutputForDiagnostics(t *testing.T, cfg config.Config) SearchOutput {
	t.Helper()
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Start:     0,
			Hits:      []opengrok.Hit{},
		},
	}
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}
	return output
}

func marshalObject(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("Unmarshal object from %s: %v", raw, err)
	}
	return fields
}
