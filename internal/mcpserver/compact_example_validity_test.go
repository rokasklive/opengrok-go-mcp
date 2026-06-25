// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

var jsonObjectExample = regexp.MustCompile(`\{[^{}]*\}`)

// TestCompactToolExamplesValidateAgainstSchema guards every JSON-object example
// embedded in a compact tool description: its fields must exist in that tool's
// schema and its operation must be one the tool actually enables. This catches
// the class of bug where one claim's example (e.g. a search-shaped
// {"operation":"code","query":...}) is rendered on a tool that has no such
// operation or field — a broken first-call an agent would copy verbatim.
func TestCompactToolExamplesValidateAgainstSchema(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = allCapabilities()
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	enabledOps := map[string][]string{
		"opengrok_projects": compactProjectsOperations(cfg),
		"opengrok_search":   compactSearchOperations(cfg),
		"opengrok_symbols":  compactSymbolsOperations(cfg),
		"opengrok_read":     compactReadOperations(cfg),
	}

	for _, tool := range tools.Tools {
		allowedFields := collectSchemaPropertyNames(t, tool.InputSchema)
		for _, raw := range jsonObjectExample.FindAllString(tool.Description, -1) {
			var obj map[string]any
			if err := json.Unmarshal([]byte(raw), &obj); err != nil {
				continue // a brace span that is not a JSON object example
			}
			for field := range obj {
				if !allowedFields[field] {
					t.Errorf("%s example %s uses field %q absent from the tool schema", tool.Name, raw, field)
				}
			}
			op, ok := obj["operation"].(string)
			if !ok {
				continue
			}
			if ops := enabledOps[tool.Name]; !operationEnabled(ops, op) {
				t.Errorf("%s example %s names operation %q not in enabled operations %v", tool.Name, raw, op, ops)
			}
		}
	}
}

// collectSchemaPropertyNames returns the union of property names declared at the
// schema root and inside every oneOf/anyOf/allOf branch (compact tools key their
// per-operation fields inside oneOf branches).
func collectSchemaPropertyNames(t *testing.T, schema any) map[string]bool {
	t.Helper()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal input schema: %v", err)
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatalf("unmarshal input schema: %v", err)
	}
	names := map[string]bool{}
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		if props, ok := n["properties"].(map[string]any); ok {
			for key := range props {
				names[key] = true
			}
		}
		for _, combinator := range []string{"oneOf", "anyOf", "allOf"} {
			branches, ok := n[combinator].([]any)
			if !ok {
				continue
			}
			for _, branch := range branches {
				if bm, ok := branch.(map[string]any); ok {
					walk(bm)
				}
			}
		}
	}
	walk(node)
	return names
}
