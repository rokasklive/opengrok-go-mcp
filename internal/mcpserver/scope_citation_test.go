// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestSearchProjectScopeAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{config.ToolSurfaceFull, config.ToolSurfaceCompact, config.ToolSurfaceGateway} {
		t.Run(surface, func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				def      string
				args     map[string]any
				wantCode string
				want     []string
			}{
				{name: "unknown project", args: map[string]any{"project": "platfrom"}, wantCode: "UNKNOWN_PROJECT"},
				{name: "unknown project in list", def: "platform", args: map[string]any{"projects": []string{"platform", "platfrom"}}, wantCode: "UNKNOWN_PROJECT"},
				{name: "all projects flag and unknown project", args: map[string]any{"allow_all_projects": true, "project": "platfrom"}, wantCode: "UNKNOWN_PROJECT"},
				{name: "valid project", args: map[string]any{"project": "platform"}, want: []string{"platform"}},
				{name: "omitted project uses default", def: "platform", want: []string{"platform"}},
				{name: "omitted project without default"},
				{name: "explicit all projects", def: "platform", args: map[string]any{"allow_all_projects": true}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cfg := testConfig()
					cfg.ToolSurface = surface
					cfg.ProjectRequired = false
					cfg.DefaultProject = tc.def
					cfg.Projects = []string{"platform"}
					cfg.ProjectSource = config.ProjectSourceConfigured
					backend := &fakeBackend{}
					session, closeSession := connectMCPServer(t, NewMCPServer(cfg, backend, "test"))
					defer closeSession()

					args := map[string]any{"query": "Example"}
					maps.Copy(args, tc.args)
					result := callSurfaceSearch(t, session, surface, "search_code", "opengrok_search", "code", "search.code", args)
					if tc.wantCode != "" {
						var body ToolErrorBody
						decodeSurfaceResult(t, result, surface, &body)
						if !result.IsError || body.ErrorCode != tc.wantCode || len(backend.searchRequests) != 0 {
							t.Fatalf("want %s before backend search, got error=%t structured=%v text=%q calls=%d",
								tc.wantCode, result.IsError, result.StructuredContent, toolResultText(result), len(backend.searchRequests))
						}
						return
					}
					if result.IsError || len(backend.searchRequests) != 1 {
						t.Fatalf("want one successful search, got error=%t calls=%d", result.IsError, len(backend.searchRequests))
					}
					if got := backend.searchRequests[0].Projects; !slices.Equal(got, tc.want) {
						t.Fatalf("backend projects = %v, want %v", got, tc.want)
					}
				})
			}
		})
	}
}

func TestSearchCitationsAcrossSurfaces(t *testing.T) {
	for _, surface := range []string{config.ToolSurfaceFull, config.ToolSurfaceCompact, config.ToolSurfaceGateway} {
		t.Run(surface, func(t *testing.T) {
			cfg := testConfig()
			cfg.ToolSurface = surface
			backend := &fakeBackend{
				searchResult: opengrok.SearchResult{
					TotalHits: 3,
					Hits: []opengrok.Hit{
						{Project: "platform", FilePath: "example.go", LineNumber: 40, Snippet: strPtr("Example"), AttributionSource: "matched_prefix"},
						{Project: "platform", FilePath: "example.go", LineNumber: 2, AttributionSource: "matched_prefix"},
						{FilePath: "other.go", LineNumber: 1, AttributionUncertain: true, AttributionWarning: "Unmatched project prefix", AttributionSource: "unknown"},
						{Project: "platform", FilePath: "README.md"},
					},
				},
				fileContent: "package example\nfunc Example() {}\n",
			}
			session, closeSession := connectMCPServer(t, NewMCPServer(cfg, backend, "test"))
			defer closeSession()

			t.Run("compound text", func(t *testing.T) {
				result := callSurfaceSearch(t, session, surface, "search_and_read", "opengrok_search", "read", "compound.search_and_read", map[string]any{"query": "Example"})
				var out SearchAndReadOutput
				decodeSurfaceResult(t, result, surface, &out)
				if result.IsError || len(out.Results) != 4 || out.ResultsOnPage != 4 {
					t.Fatalf("want four retained hits, got error=%t response=%+v", result.IsError, out)
				}
				stale, valid, uncertain := out.Results[0], out.Results[1], out.Results[2]
				checkStaleCitation(t, out.WarningFields, stale.LineNumber, stale.Citation, stale.ResourceURI)
				if stale.Content != "" || stale.StartLine != 0 || stale.EndLine != 0 ||
					stale.Snippet == nil || *stale.Snippet != "Example" || stale.AttributionSource != "matched_prefix" {
					t.Errorf("stale hit must retain index metadata and have no current source window, got %+v", stale)
				}
				if valid.Citation.Line != 2 || !strings.HasSuffix(valid.Citation.URL, "#2") ||
					!strings.HasSuffix(valid.ResourceURI, "#L2") || !strings.Contains(valid.Content, "func Example() {}") {
					t.Errorf("valid hit must retain its source and line links, got %+v", valid)
				}
				if !uncertain.AttributionUncertain || uncertain.AttributionSource != "unknown" ||
					uncertain.AttributionWarning == nil || *uncertain.AttributionWarning != "Unmatched project prefix" ||
					uncertain.Content != "" || uncertain.Citation.URL != "" || uncertain.ResourceURI != "" ||
					!hasWarning(out.WarningFields, "ATTRIBUTION_UNCERTAIN") {
					t.Errorf("uncertain hit must retain attribution and omit unverifiable source and links, got %+v, warnings %+v", uncertain, out.Warnings)
				}
				file := out.Results[3]
				if file.Content != "" || file.LineNumber != 0 || !strings.HasSuffix(file.Citation.URL, "/xref/platform/README.md") {
					t.Errorf("unanchored hit must have a file link and no source window, got %+v", file)
				}
				if backend.fileCallCount != 1 {
					t.Errorf("file reads = %d, want one read for the attributable file", backend.fileCallCount)
				}
			})

			t.Run("expanded search", func(t *testing.T) {
				result := callSurfaceSearch(t, session, surface, "search_code", "opengrok_search", "code", "search.code", map[string]any{"query": "Example", "expand_context": true, "response_mode": "full"})
				var out SearchOutput
				decodeSurfaceResult(t, result, surface, &out)
				if result.IsError || len(out.Results) != 4 {
					t.Fatalf("want four retained hits, got error=%t response=%+v", result.IsError, out)
				}
				stale := out.Results[0]
				checkStaleCitation(t, out.WarningFields, stale.LineNumber, stale.Citation, stale.ResourceURI)
				if stale.Context != nil || !strings.HasSuffix(stale.DisplayURL, "/xref/platform/example.go") ||
					out.Results[1].Context == nil || out.Results[2].Context != nil {
					t.Errorf("want context only on the valid hit, got %+v", out.Results)
				}
			})

			t.Run("compound definition", func(t *testing.T) {
				result := callSurfaceSearch(t, session, surface, "find_symbol_and_references", "opengrok_symbols", "find", "compound.find_symbol_and_references", map[string]any{"symbol": "Example"})
				var out FindSymbolAndReferencesOutput
				decodeSurfaceResult(t, result, surface, &out)
				if result.IsError || out.Definition == nil || len(out.References) == 0 {
					t.Fatalf("want indexed definition and references, got error=%t response=%+v", result.IsError, out)
				}
				def := out.Definition
				checkStaleCitation(t, out.WarningFields, def.LineNumber, def.Citation, def.ResourceURI)
				if def.Content != "" || hasWarning(out.WarningFields, "NO_DEFINITION_FOUND") {
					t.Errorf("want stale indexed definition without current content, got %+v, warnings %+v", def, out.Warnings)
				}
			})
		})
	}
}

func checkStaleCitation(t *testing.T, warnings WarningFields, indexedLine int, citation Citation, resourceURI string) {
	t.Helper()
	if indexedLine != 40 || citation.Line != 0 || citation.URL != "https://grok.example.com/source/xref/platform/example.go" ||
		citation.Markdown != "[example.go](https://grok.example.com/source/xref/platform/example.go)" ||
		resourceURI != "opengrok://project/platform/files/example.go" || !hasWarning(warnings, "STALE_SEARCH_HIT") ||
		warnings.Warning == nil {
		t.Errorf("want index line 40, file-level links and a stale warning, got line %d citation %+v URI %q warnings %+v",
			indexedLine, citation, resourceURI, warnings)
	}
}

func callSurfaceSearch(t *testing.T, session *mcp.ClientSession, surface, full, compact, compactOp, gatewayOp string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	name := full
	switch surface {
	case config.ToolSurfaceCompact:
		name = compact
		args["operation"] = compactOp
	case config.ToolSurfaceGateway:
		name = "opengrok_call"
		args = map[string]any{"operation": gatewayOp, "payload": args}
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return result
}

func decodeSurfaceResult(t *testing.T, response *mcp.CallToolResult, surface string, out any) {
	t.Helper()
	raw, err := json.Marshal(response.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if surface == config.ToolSurfaceGateway && !response.IsError {
		var envelope struct {
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		raw = envelope.Result
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}
