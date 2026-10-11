// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/cache"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func testRejection() *opengrok.AuthRejectedError {
	return &opengrok.AuthRejectedError{
		Method: "GET",
		Path:   "/source/api/v1/search",
		At:     time.Date(2026, 10, 6, 9, 53, 8, 0, time.UTC),
	}
}

// rejectingBackend reports a credential rejection like the OpenGrok client
// does after its first rejected request.
type rejectingBackend struct {
	*fakeBackend
	rejection *opengrok.AuthRejectedError
}

func (b rejectingBackend) AuthRejection() *opengrok.AuthRejectedError { return b.rejection }

func TestMapToolErrorReportsRejectedCredentials(t *testing.T) {
	// The client surfaces the rejection wrapped in a *url.Error and call-site context.
	err := fmt.Errorf("search: %w", &url.Error{Op: "Get", URL: "https://grok.example.com/source/api/v1/search", Err: testRejection()})

	body := mapToolError(err)
	if body.ErrorCode != codeAuthRejected {
		t.Fatalf("ErrorCode = %q, want %s", body.ErrorCode, codeAuthRejected)
	}
	for _, want := range []string{"Tell the user", "restart", "password", "Do not retry"} {
		if !strings.Contains(body.Suggestion, want) {
			t.Errorf("Suggestion should mention %q: %s", want, body.Suggestion)
		}
	}
	if body.Details["rejected_at"] != "2026-10-06T09:53:08Z" || body.Details["path"] != "/source/api/v1/search" {
		t.Fatalf("Details = %v, want rejected_at and path", body.Details)
	}
}

func TestCompactSearchReportsRejectedCredentials(t *testing.T) {
	clientSession, backend := compactTestServer(t, allCapabilities())
	backend.searchErr = &url.Error{Op: "Get", URL: "https://grok.example.com/source/api/v1/search", Err: testRejection()}

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "opengrok_search",
		Arguments: map[string]any{"operation": "code", "query": "Engine"},
	})
	if err != nil {
		t.Fatalf("CallTool(opengrok_search) error = %v", err)
	}
	if !result.IsError {
		t.Fatal("IsError = false, want an AUTH_REJECTED tool error")
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var body ToolErrorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal ToolErrorBody: %v", err)
	}
	if body.ErrorCode != codeAuthRejected {
		t.Fatalf("error_code = %q, want %s", body.ErrorCode, codeAuthRejected)
	}
	// Clients that only forward text to the model must still carry the remediation.
	if text := toolResultText(result); !strings.Contains(text, "restart") {
		t.Fatalf("text content = %q, want the remediation included", text)
	}
}

// The project list is served from the startup snapshot without a request, so
// it must report the rejection itself — it is usually the agent's first call.
func TestListProjectsReportsRejectedCredentials(t *testing.T) {
	cfg := testConfig()
	backends := map[string]Backend{
		"client":  rejectingBackend{fakeBackend: &fakeBackend{}, rejection: testRejection()},
		"caching": NewCachingBackend(rejectingBackend{fakeBackend: &fakeBackend{}, rejection: testRejection()}, cache.New(time.Minute), 10),
	}
	for name, backend := range backends {
		t.Run(name, func(t *testing.T) {
			service := NewService(cfg, backend)
			_, err := service.ListProjects(context.Background(), ListProjectsInput{})
			if body := mapToolError(err); body.ErrorCode != codeAuthRejected {
				t.Fatalf("ListProjects error = %v (code %q), want %s", err, body.ErrorCode, codeAuthRejected)
			}
		})
	}

	healthy := NewService(cfg, rejectingBackend{fakeBackend: &fakeBackend{}})
	if _, err := healthy.ListProjects(context.Background(), ListProjectsInput{}); err != nil {
		t.Fatalf("ListProjects error = %v, want success while credentials are accepted", err)
	}
}
