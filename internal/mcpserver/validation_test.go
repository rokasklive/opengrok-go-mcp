// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestErrValidationClasses(t *testing.T) {
	clientSession, _ := compactTestServer(t, allCapabilities())

	tests := []struct {
		name           string
		tool           string
		args           map[string]any
		wantCode       string
		wantMessage    string
		wantSuggestion string
	}{
		{
			name:           "unknown operation",
			tool:           "opengrok_read",
			args:           map[string]any{"operation": "read", "file_path": "src/Engine.go"},
			wantCode:       codeUnknownOperation,
			wantMessage:    "read",
			wantSuggestion: "enabled operations",
		},
		{
			name:           "missing required field",
			tool:           "opengrok_search",
			args:           map[string]any{"operation": "code"},
			wantCode:       codeMissingRequiredField,
			wantMessage:    "query",
			wantSuggestion: "requires query",
		},
		{
			name:           "invalid field type",
			tool:           "opengrok_read",
			args:           map[string]any{"operation": "context", "file_path": "src/Engine.go", "line_number": "abc"},
			wantCode:       codeInvalidFieldType,
			wantMessage:    "line_number",
			wantSuggestion: "must be integer",
		},
		{
			name:           "unknown field",
			tool:           "opengrok_search",
			args:           map[string]any{"operation": "code", "query": "Engine", "frobnicate": true},
			wantCode:       codeUnknownField,
			wantMessage:    "frobnicate",
			wantSuggestion: "not a recognized parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      tt.tool,
				Arguments: tt.args,
			})
			if err != nil {
				t.Fatalf("CallTool(%s) transport error = %v", tt.tool, err)
			}
			body := mustToolErrorBody(t, result)
			if body.ErrorCode != tt.wantCode {
				t.Fatalf("error_code = %q, want %q (body=%+v)", body.ErrorCode, tt.wantCode, body)
			}
			if !strings.Contains(body.Message, tt.wantMessage) {
				t.Fatalf("message = %q, want it to mention %q", body.Message, tt.wantMessage)
			}
			if !strings.Contains(body.Suggestion, tt.wantSuggestion) {
				t.Fatalf("suggestion = %q, want it to mention %q", body.Suggestion, tt.wantSuggestion)
			}
			if strings.Contains(body.Message, "oneOf") || strings.Contains(body.Suggestion, "oneOf") {
				t.Fatalf("validation error leaked opaque oneOf message: %+v", body)
			}
		})
	}
}

func mustToolErrorBody(t *testing.T, result *mcp.CallToolResult) ToolErrorBody {
	t.Helper()
	if result == nil {
		t.Fatal("CallTool result is nil, want structured error")
	}
	if !result.IsError {
		t.Fatalf("CallTool IsError = false, want true")
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var body ToolErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal ToolErrorBody from %s: %v", raw, err)
	}
	if body.Suggestion == "" {
		t.Fatalf("ToolErrorBody suggestion is empty: %+v", body)
	}
	return body
}
