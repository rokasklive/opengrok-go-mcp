// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestCompactSurfaceDoesNotExposeContentOrMemoryWithoutCapabilities(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{
		SearchCode:              true,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "opengrok_compound" || tool.Name == "opengrok_memory" {
			t.Fatalf("tool %q registered without required capability", tool.Name)
		}
	}
}

func TestCompactSearchAcceptsObjectPayload(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_search",
		Arguments: map[string]any{
			"operation": "code",
			"payload": map[string]any{
				"query":   "Engine",
				"project": "platform",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool result is an error: %+v", result.Content)
	}
}

func TestCompactMemoryAcceptsObjectPayloadAndOmittedPayload(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Transport = config.TransportStdio
	cfg.Capabilities = config.Capabilities{Memory: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	setResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_memory",
		Arguments: map[string]any{
			"operation": "set",
			"payload":   map[string]any{"key": "k", "value": "v"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(set) returned error: %v", err)
	}
	if setResult.IsError {
		t.Fatalf("CallTool(set) result is an error: %+v", setResult.Content)
	}

	// Operations such as list take no payload; an omitted payload must validate.
	listResult, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "opengrok_memory",
		Arguments: map[string]any{"operation": "list"},
	})
	if err != nil {
		t.Fatalf("CallTool(list) returned error: %v", err)
	}
	if listResult.IsError {
		t.Fatalf("CallTool(list) result is an error: %+v", listResult.Content)
	}
}
