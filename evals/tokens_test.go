// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTotalColdWarmGateway(t *testing.T) {
	list, discover, req, resp := 1000, 500, 200, 800
	cold := totalColdBytes(list, discover, req, resp)
	warm := totalWarmBytes(list, req, resp)
	if cold != 2500 {
		t.Fatalf("cold = %d, want 2500", cold)
	}
	if warm != 2000 {
		t.Fatalf("warm = %d, want 2000", warm)
	}
	if warm != cold-discover {
		t.Fatalf("warm should exclude discover_bytes")
	}
}

func TestEstTokens(t *testing.T) {
	if estTokens(100) != 25 {
		t.Fatalf("estTokens(100) = %d, want 25", estTokens(100))
	}
}

func TestCountCallToolResponse(t *testing.T) {
	out := &mcp.CallToolResult{
		StructuredContent: map[string]any{"total_hits": 2, "results": []any{}},
		Content: []mcp.Content{
			&mcp.TextContent{Text: "hello"},
		},
	}
	text, structured := countCallToolResponse(out)
	if text != 5 {
		t.Fatalf("text = %d, want 5", text)
	}
	if structured == 0 {
		t.Fatal("structured bytes should be non-zero")
	}
	if text+structured == 0 {
		t.Fatal("response bytes should be positive")
	}
}

func TestCountSchemaByToolListToolsOnly(t *testing.T) {
	tools := []*mcp.Tool{{
		Name:        "search_code",
		Description: "search",
		InputSchema: map[string]any{"type": "object"},
	}}
	byTool := countSchemaByTool(tools)
	if len(byTool) != 1 {
		t.Fatalf("len = %d, want 1", len(byTool))
	}
	if byTool["search_code"] == 0 {
		t.Fatal("schema bytes should be non-zero")
	}
}
