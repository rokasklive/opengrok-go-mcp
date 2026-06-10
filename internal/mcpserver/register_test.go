// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestNewMCPServerRegistersOnlyEnabledTools(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{
		SearchCode:              true,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
		GetFileContext:          true,
		Memory:                  true,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	got := []string{}
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	want := []string{"search_code", "search_symbol_definitions", "search_symbol_references", "get_file_context", "read_file", "search_and_read", "find_symbol_and_references", "search_implementations", "search_cross_project_references", "memory_set", "memory_get", "memory_list", "memory_delete", "memory_clear"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestFullSurfaceRegistersProjectOverviewOnlyWithListFiles(t *testing.T) {
	tests := []struct {
		name         string
		capabilities config.Capabilities
		wantPresent  bool
	}{
		{
			name:         "list projects only",
			capabilities: config.Capabilities{ListProjects: true},
			wantPresent:  false,
		},
		{
			name:         "list files only",
			capabilities: config.Capabilities{ListFiles: true},
			wantPresent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.ToolSurface = config.ToolSurfaceFull
			cfg.Capabilities = tt.capabilities
			server := NewMCPServer(cfg, &fakeBackend{}, "test")
			clientSession, cleanup := connectMCPServer(t, server)
			defer cleanup()

			tools, err := clientSession.ListTools(context.Background(), nil)
			if err != nil {
				t.Fatalf("ListTools returned error: %v", err)
			}

			gotPresent := false
			for _, tool := range tools.Tools {
				if tool.Name == "get_project_overview" {
					gotPresent = true
					break
				}
			}
			if gotPresent != tt.wantPresent {
				t.Fatalf("get_project_overview present = %t, want %t", gotPresent, tt.wantPresent)
			}
		})
	}
}

func TestNewMCPServerSearchCursorIsOptionalInToolSchema(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{
		SearchSymbolDefinitions: true,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools.Tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(tools.Tools))
	}

	schema, ok := tools.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("InputSchema type = %T, want map", tools.Tools[0].InputSchema)
	}
	required, _ := schema["required"].([]any)
	for _, field := range required {
		if field == "cursor" || field == "include_links" {
			t.Fatalf("required fields = %#v, want cursor/include_links optional", required)
		}
	}
}

func TestNewMCPServerReturnsServer(t *testing.T) {
	server := NewMCPServer(testConfig(), &fakeBackend{}, "test")
	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func TestNewMCPServerRegistersListSymbolsWhenEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{ListSymbols: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "list_symbols") {
		t.Fatalf("tools = %#v, want list_symbols included", names)
	}
}

func TestNewMCPServerDoesNotRegisterListSymbolsWhenDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{ListSymbols: false}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	for _, tool := range tools.Tools {
		if tool.Name == "list_symbols" {
			t.Fatal("list_symbols tool registered, want absent when disabled")
		}
	}
}

func TestFullSearchCoercesStringEncodedBooleans(t *testing.T) {
	// Some MCP clients serialize scalar arguments as JSON strings, sending
	// include_links:"true" instead of include_links:true. The boolean fields
	// are *bool (schema ["null","boolean"]), so the SDK validator rejects the
	// string before the handler runs. The server coerces string-encoded
	// booleans for boolean-typed fields so these calls succeed.
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"links true / snippets false", map[string]any{"query": "Engine", "path_prefix": "", "file_type": "", "page_size": 10, "include_links": "true", "include_snippets": "false"}},
		{"links false / snippets true", map[string]any{"query": "Engine", "path_prefix": "", "file_type": "", "page_size": 10, "include_links": "false", "include_snippets": "true"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "search_code",
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("CallTool returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("CallTool result is an error: %+v", result.Content)
			}
		})
	}
}

func TestFullSearchCoercesStringEncodedNumbers(t *testing.T) {
	// The same clients that stringify booleans also send numeric arguments as
	// JSON strings, e.g. page_size:"10". page_size/max_hits_per_file are int
	// (schema "integer"), so the validator rejects a string before the handler
	// runs. The server coerces string-encoded numbers for numeric-typed fields.
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "search_code",
		Arguments: map[string]any{
			"query":             "Engine",
			"path_prefix":       "",
			"file_type":         "",
			"page_size":         "10", // string-encoded integer from a flaky client
			"max_hits_per_file": "5",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool result is an error: %+v", result.Content)
	}
}

func TestSearchCodeInputSchemaRequiredFields(t *testing.T) {
	schema, err := jsonschema.For[SearchCodeInput](nil)
	if err != nil {
		t.Fatalf("infer SearchCodeInput schema: %v", err)
	}
	if !slices.Contains(schema.Required, "query") {
		t.Errorf("query should be required, required=%v", schema.Required)
	}
	for _, field := range []string{"file_type", "path_prefix", "page_size"} {
		if slices.Contains(schema.Required, field) {
			t.Errorf("%s should NOT be required, required=%v", field, schema.Required)
		}
	}
	if _, ok := schema.Properties["tokenized"]; !ok {
		t.Errorf("tokenized property missing from schema")
	}
	if _, ok := schema.Properties["path_exclude"]; !ok {
		t.Errorf("path_exclude property missing from schema")
	}
}

func TestSearchAndReadInputSchemaRequiredFields(t *testing.T) {
	schema, err := jsonschema.For[SearchAndReadInput](nil)
	if err != nil {
		t.Fatalf("infer SearchAndReadInput schema: %v", err)
	}
	if !slices.Contains(schema.Required, "query") {
		t.Errorf("query should be required, required=%v", schema.Required)
	}
	for _, field := range []string{"file_type", "path_prefix", "page_size"} {
		if slices.Contains(schema.Required, field) {
			t.Errorf("%s should NOT be required, required=%v", field, schema.Required)
		}
	}
	if _, ok := schema.Properties["tokenized"]; !ok {
		t.Errorf("tokenized property missing from schema")
	}
	if _, ok := schema.Properties["path_exclude"]; !ok {
		t.Errorf("path_exclude property missing from schema")
	}
}

func TestSymbolSearchInputSchemaRequiredFields(t *testing.T) {
	schema, err := jsonschema.For[SymbolSearchInput](nil)
	if err != nil {
		t.Fatalf("infer SymbolSearchInput schema: %v", err)
	}
	if !slices.Contains(schema.Required, "symbol") {
		t.Errorf("symbol should be required, required=%v", schema.Required)
	}
	if slices.Contains(schema.Required, "page_size") {
		t.Errorf("page_size should NOT be required, required=%v", schema.Required)
	}
	if prop, ok := schema.Properties["symbol"]; !ok || prop.Description == "" {
		t.Errorf("symbol property should be present and documented; got ok=%v", ok)
	}
}

func TestSearchToolDescriptionsMentionQuoting(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true, GetFileContext: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	descs := map[string]string{}
	for _, tool := range tools.Tools {
		descs[tool.Name] = tool.Description
	}
	for _, name := range []string{"search_code", "search_and_read"} {
		d, ok := descs[name]
		if !ok {
			t.Fatalf("tool %q not registered", name)
		}
		if !strings.Contains(d, "quote") {
			t.Errorf("tool %q description should mention quoting; got: %s", name, d)
		}
		if !strings.Contains(d, "tokenized") {
			t.Errorf("tool %q description should mention tokenized opt-out", name)
		}
	}
}
