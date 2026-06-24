// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func allCapabilities() config.Capabilities {
	return config.Capabilities{
		ListProjects:            true,
		ListFiles:               true,
		SearchCode:              true,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
		GetFileContext:          true,
		ListSymbols:             true,
	}
}

func compactTestServer(t *testing.T, caps config.Capabilities) (*mcp.ClientSession, *fakeBackend) {
	t.Helper()
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = caps
	backend := &fakeBackend{}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	t.Cleanup(cleanup)
	return clientSession, backend
}

func TestCompactSurfaceRegistersFourConsolidatedTools(t *testing.T) {
	clientSession, _ := compactTestServer(t, allCapabilities())

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	got := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	want := []string{"opengrok_projects", "opengrok_search", "opengrok_symbols", "opengrok_read"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("compact tools = %#v, want %#v", got, want)
	}
	for _, forbidden := range []string{"opengrok_compound", "opengrok_memory"} {
		if slices.Contains(got, forbidden) {
			t.Fatalf("forbidden tool %q registered", forbidden)
		}
	}
}

func operationEnumFromToolSchema(inputSchema any) ([]string, error) {
	data, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, err
	}
	var probe struct {
		Properties struct {
			Operation struct {
				Enum []any `json:"enum"`
			} `json:"operation"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(probe.Properties.Operation.Enum))
	for _, v := range probe.Properties.Operation.Enum {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func TestCompactSurfaceNoSemanticOverlapAcrossTools(t *testing.T) {
	clientSession, _ := compactTestServer(t, allCapabilities())

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	byTool := map[string][]string{}
	for _, tool := range tools.Tools {
		ops, err := operationEnumFromToolSchema(tool.InputSchema)
		if err != nil {
			t.Fatalf("tool %q schema: %v", tool.Name, err)
		}
		byTool[tool.Name] = ops
	}

	searchOps := sliceToSet(byTool["opengrok_search"])
	for _, moved := range []string{"definitions", "references"} {
		if searchOps[moved] {
			t.Fatalf("opengrok_search still exposes %q; symbol work belongs in opengrok_symbols", moved)
		}
	}
	if sliceToSet(byTool["opengrok_symbols"])["cross_project_references"] {
		t.Fatal("opengrok_symbols should use cross_project, not cross_project_references")
	}
}

func sliceToSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func TestCompactOperationRoutingAndErrors(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{{FilePath: "a.go", LineNumber: 1}}},
		fileContent:  "package main\n",
	}
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = allCapabilities()
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	cases := []struct {
		tool string
		args map[string]any
	}{
		{"opengrok_symbols", map[string]any{"operation": "find", "symbol": "Foo"}},
		{"opengrok_search", map[string]any{"operation": "read", "query": "Engine"}},
		{"opengrok_read", map[string]any{"operation": "file", "project": "platform", "file_path": "main.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.tool+"."+tc.args["operation"].(string), func(t *testing.T) {
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      tc.tool,
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

	unknown, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "opengrok_search",
		Arguments: map[string]any{"operation": "nope", "query": "x"},
	})
	if err == nil && (unknown == nil || !unknown.IsError) {
		t.Fatal("unknown operation should fail validation or return isError")
	}

	missing, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "opengrok_search",
		Arguments: map[string]any{"operation": "code"},
	})
	if err == nil && (missing == nil || !missing.IsError) {
		t.Fatal("missing required query should fail validation or return isError")
	}
}

func TestCompactSearchAcceptsFlattenedArguments(t *testing.T) {
	clientSession, _ := compactTestServer(t, config.Capabilities{SearchCode: true})

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_search",
		Arguments: map[string]any{
			"operation": "code",
			"query":     "Engine",
			"project":   "platform",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool result is an error: %+v", result.Content)
	}
}

func TestCompactSearchCoercesStringEncodedScalars(t *testing.T) {
	clientSession, _ := compactTestServer(t, config.Capabilities{SearchCode: true})

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_search",
		Arguments: map[string]any{
			"operation": "code",
			"query":     "Engine",
			"tokenized": "true",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool result is an error: %+v", result.Content)
	}
}

func TestCompactProjectsFilesAndOverview(t *testing.T) {
	backend := &fakeBackend{
		fileEntries: []opengrok.FileEntry{{Path: "main.go"}},
		projectOverview: opengrok.ProjectOverview{
			TotalFiles: 10,
			TotalDirs:  2,
		},
	}
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{ListFiles: true}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	filesResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_projects",
		Arguments: map[string]any{
			"operation": "files",
			"project":   "platform",
			"path":      "src",
		},
	})
	if err != nil || filesResult.IsError {
		t.Fatalf("files call failed: err=%v result=%+v", err, filesResult)
	}

	overviewResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_projects",
		Arguments: map[string]any{
			"operation": "overview",
			"project":   "platform",
		},
	})
	if err != nil || overviewResult.IsError {
		t.Fatalf("overview call failed: err=%v result=%+v", err, overviewResult)
	}
}

func TestCompactToolGatingOmitsProjectsWhenNoOps(t *testing.T) {
	clientSession, _ := compactTestServer(t, config.Capabilities{SearchCode: true})

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "opengrok_projects" {
			t.Fatal("opengrok_projects registered without list/files capabilities")
		}
	}
}

func TestCompactSymbolsListGatedOnCapability(t *testing.T) {
	clientSession, _ := compactTestServer(t, config.Capabilities{
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
		ListSymbols:             false,
	})

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	var symbolsOps []string
	for _, tool := range tools.Tools {
		if tool.Name == "opengrok_symbols" {
			var err error
			symbolsOps, err = operationEnumFromToolSchema(tool.InputSchema)
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			break
		}
	}
	if symbolsOps == nil {
		t.Fatal("opengrok_symbols not registered")
	}
	if slices.Contains(symbolsOps, "list") {
		t.Fatalf("list should be absent from enum when ListSymbols disabled: %#v", symbolsOps)
	}
}

func TestCompactSymbolsDescriptionMatchesEnabledOperations(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{
		SearchSymbolDefinitions: true,
		ListSymbols:             true,
		SearchSymbolReferences:  false,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	var symbolsTool *mcp.Tool
	for _, tool := range tools.Tools {
		if tool.Name == "opengrok_symbols" {
			symbolsTool = tool
			break
		}
	}
	if symbolsTool == nil {
		t.Fatal("opengrok_symbols not registered")
	}

	ops, err := operationEnumFromToolSchema(symbolsTool.InputSchema)
	if err != nil {
		t.Fatalf("schema enum: %v", err)
	}
	desc := symbolsTool.Description
	for _, forbidden := range []string{"references", "find", "implementations", "cross_project"} {
		if strings.Contains(desc, "operation="+forbidden) {
			t.Fatalf("description mentions disabled operation=%s; enum=%v", forbidden, ops)
		}
	}
	for _, required := range []string{"definitions", "list"} {
		if !strings.Contains(desc, "operation="+required) {
			t.Fatalf("description missing enabled operation=%s; enum=%v", required, ops)
		}
	}
}

func TestCompactProjectsDescriptionMatchesEnabledOperations(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{
		ListProjects: true,
		ListFiles:    false,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	var projectsTool *mcp.Tool
	for _, tool := range tools.Tools {
		if tool.Name == "opengrok_projects" {
			projectsTool = tool
			break
		}
	}
	if projectsTool == nil {
		t.Fatal("opengrok_projects not registered")
	}

	ops, err := operationEnumFromToolSchema(projectsTool.InputSchema)
	if err != nil {
		t.Fatalf("schema enum: %v", err)
	}
	desc := projectsTool.Description
	for _, forbidden := range []string{"files", "overview"} {
		if strings.Contains(desc, "operation="+forbidden) {
			t.Fatalf("description mentions disabled operation=%s; enum=%v", forbidden, ops)
		}
	}
	if !strings.Contains(desc, "operation=list") {
		t.Fatalf("description missing enabled operation=list; enum=%v", ops)
	}
}

func TestCompactSearchDescriptionMatchesEnabledOperations(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{SearchCode: true, GetFileContext: false}
	assertCompactDescriptionMatchesOps(t, cfg, "opengrok_search", []string{"code"}, []string{"read"})
}

func TestCompactReadDescriptionMatchesEnabledOperations(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{GetFileContext: true}
	assertCompactDescriptionMatchesOps(t, cfg, "opengrok_read", []string{"file", "context"}, nil)
}

func assertCompactDescriptionMatchesOps(t *testing.T, cfg config.Config, toolName string, wantPresent, wantAbsent []string) {
	t.Helper()
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	var tool *mcp.Tool
	for _, candidate := range tools.Tools {
		if candidate.Name == toolName {
			tool = candidate
			break
		}
	}
	if tool == nil {
		t.Fatalf("%s not registered", toolName)
	}
	ops, err := operationEnumFromToolSchema(tool.InputSchema)
	if err != nil {
		t.Fatalf("schema enum: %v", err)
	}
	desc := tool.Description
	for _, forbidden := range wantAbsent {
		if strings.Contains(desc, "operation="+forbidden) {
			t.Fatalf("description mentions disabled operation=%s; enum=%v", forbidden, ops)
		}
	}
	for _, required := range wantPresent {
		if !strings.Contains(desc, "operation="+required) {
			t.Fatalf("description missing enabled operation=%s; enum=%v", required, ops)
		}
	}
	if !strings.Contains(desc, compactProjectScopeNote(cfg)) {
		t.Fatalf("description missing project scope note")
	}
}

func toolResultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	raw, _ := json.Marshal(result.Content)
	return string(raw)
}

func TestCompactDescriptionsIncludeEconomyHintAndDisambiguation(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = allCapabilities()

	for name, desc := range map[string]string{
		"search":  compactSearchDescription(cfg),
		"symbols": compactSymbolsDescription(cfg),
		"read":    compactReadDescription(cfg),
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(desc, "OPENGROK_MCP_AGENT_PROFILE defaults to economy") {
				t.Fatalf("description missing agent profile economy hint: %s", desc)
			}
		})
	}
}
