// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestMapToolErrorServiceError(t *testing.T) {
	body := mapToolError(&Error{Code: "INVALID_CURSOR", Message: "cursor mismatch"})
	if body.ErrorCode != "INVALID_CURSOR" {
		t.Fatalf("ErrorCode = %q, want INVALID_CURSOR", body.ErrorCode)
	}
}

func TestMapToolErrorFileNotFound(t *testing.T) {
	body := mapToolError(&opengrok.StatusError{
		Code:   http.StatusNotFound,
		Path:   "/raw/platform/src/missing.go",
		Status: "404 Not Found",
	})
	if body.ErrorCode != codeFileNotFound {
		t.Fatalf("ErrorCode = %q, want %s", body.ErrorCode, codeFileNotFound)
	}
	if body.Details["http_status"] != http.StatusNotFound {
		t.Fatalf("details.http_status = %v, want 404", body.Details["http_status"])
	}
}

func TestMapFileContentError(t *testing.T) {
	err := mapFileContentError(&opengrok.StatusError{
		Code:   http.StatusNotFound,
		Path:   "/raw/platform/src/missing.go",
		Status: "404 Not Found",
	})
	var serviceErr *Error
	if !errors.As(err, &serviceErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if serviceErr.Code != codeFileNotFound {
		t.Fatalf("Code = %q, want %s", serviceErr.Code, codeFileNotFound)
	}
}

func TestErrQueryParser(t *testing.T) {
	backend := &fakeBackend{
		searchErr: &opengrok.StatusError{
			Code:   http.StatusBadRequest,
			Path:   "/search",
			Status: "400 Bad Request",
		},
	}
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_search",
		Arguments: map[string]any{
			"operation": "code",
			"query":     "class.*extends",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(search_code) transport error = %v", err)
	}
	body := mustToolErrorBody(t, result)
	if body.ErrorCode != codeQueryParserFailed {
		t.Fatalf("error_code = %q, want %q (body=%+v)", body.ErrorCode, codeQueryParserFailed, body)
	}
	if strings.Contains(body.ErrorCode, codeUpstreamHTTP) {
		t.Fatalf("error_code = %q, want query parser distinct from %q", body.ErrorCode, codeUpstreamHTTP)
	}
	for _, want := range []string{"class.*extends", "/.../", "quote"} {
		if !strings.Contains(body.Suggestion, want) && !strings.Contains(body.Message, want) {
			t.Fatalf("query parser body = %+v, want mention %q", body, want)
		}
	}
}
