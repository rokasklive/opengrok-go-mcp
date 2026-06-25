// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func init() {
	registerClaimCheck("projects-array", "TestProjectsArrayAcceptedAndApplied")
}

func TestProjectsArrayAcceptedAndApplied(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	cfg.Projects = []string{"platform", "infra"}
	backend := &fakeBackend{}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "search_code",
		Arguments: map[string]any{
			"projects": []string{"platform", "infra"},
			"query":    "Engine",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(search_code) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(search_code) IsError = true, text = %s", toolResultText(result))
	}
	if len(backend.searchRequests) != 1 {
		t.Fatalf("backend Search calls = %d, want 1", len(backend.searchRequests))
	}
	got := backend.searchRequests[0].Projects
	want := []string{"platform", "infra"}
	if !slices.Equal(got, want) {
		t.Fatalf("backend projects = %#v, want %#v", got, want)
	}
}
