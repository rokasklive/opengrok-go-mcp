// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const estTokenDivisor = 4

func utf8Bytes(s string) int {
	return len(s)
}

func jsonBytes(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(b)
}

func countListToolsBytes(tools []*mcp.Tool) int {
	return jsonBytes(tools)
}

type toolSchemaView struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	InputSchema  any    `json:"inputSchema,omitempty"`
	OutputSchema any    `json:"outputSchema,omitempty"`
}

func countSchemaByTool(tools []*mcp.Tool) map[string]int {
	out := make(map[string]int, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		view := toolSchemaView{
			Name:         t.Name,
			Description:  t.Description,
			InputSchema:  t.InputSchema,
			OutputSchema: t.OutputSchema,
		}
		out[t.Name] = jsonBytes(view)
	}
	return out
}

func countCallToolRequest(name string, arguments map[string]any) int {
	return jsonBytes(map[string]any{
		"name":      name,
		"arguments": arguments,
	})
}

func countCallToolResponse(out *mcp.CallToolResult) (textBytes, structuredBytes int) {
	if out == nil {
		return 0, 0
	}
	for _, c := range out.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			textBytes += utf8Bytes(tc.Text)
		}
	}
	if out.StructuredContent != nil {
		structuredBytes = jsonBytes(out.StructuredContent)
	}
	return textBytes, structuredBytes
}

func totalColdBytes(listTools, discover, request, response int) int {
	return listTools + discover + request + response
}

func totalWarmBytes(listTools, request, response int) int {
	return listTools + request + response
}

func estTokens(bytes int) int {
	return bytes / estTokenDivisor
}

func largestSchema(schema map[string]int) (name string, bytes int) {
	for tool, n := range schema {
		if n > bytes {
			bytes = n
			name = tool
		}
	}
	return name, bytes
}
