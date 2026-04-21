package opengrok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestListProjectsSendsRequestAndDecodesProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/v1/projects/indexed" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/projects/indexed")
		}

		writeJSON(t, w, []string{"platform", "infra"})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1/", server.Client())

	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}

	want := []string{"platform", "infra"}
	if !slices.Equal(projects, want) {
		t.Fatalf("ListProjects() = %#v, want %#v", projects, want)
	}
}

func TestSearchSendsQueryAndDecodesResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/search")
		}

		query := r.URL.Query()
		assertQueryValue(t, query, "full", "simulateDepletion")
		assertQueryValue(t, query, "projects", "platform")
		assertQueryValue(t, query, "maxresults", "20")
		assertQueryValue(t, query, "start", "0")

		response := map[string]any{
			"resultCount":   1,
			"startDocument": 3,
			"endDocument":   4,
			"results": map[string]any{
				"/platform/src/services/Engine.swift": []map[string]string{
					{
						"line":       "func simulateDepletion() {}",
						"lineNumber": "42",
						"tag":        "function",
					},
				},
			},
		}
		writeJSON(t, w, response)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	result, err := client.Search(context.Background(), SearchRequest{
		Projects: []string{"platform"},
		Query:    "simulateDepletion",
		Mode:     ModeFullText,
		Limit:    20,
		Offset:   0,
	})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}

	if result.TotalHits != 1 {
		t.Fatalf("TotalHits = %d, want %d", result.TotalHits, 1)
	}
	if result.Start != 3 {
		t.Fatalf("Start = %d, want %d", result.Start, 3)
	}
	if result.End != 4 {
		t.Fatalf("End = %d, want %d", result.End, 4)
	}
	if len(result.Hits) != 1 {
		t.Fatalf("len(Hits) = %d, want %d", len(result.Hits), 1)
	}

	hit := result.Hits[0]
	if hit.Project != "platform" {
		t.Fatalf("Hit.Project = %q, want %q", hit.Project, "platform")
	}
	if hit.FilePath != "src/services/Engine.swift" {
		t.Fatalf("Hit.FilePath = %q, want %q", hit.FilePath, "src/services/Engine.swift")
	}
	if hit.LineNumber != 42 {
		t.Fatalf("Hit.LineNumber = %d, want %d", hit.LineNumber, 42)
	}
	if hit.Snippet != "func simulateDepletion() {}" {
		t.Fatalf("Hit.Snippet = %q, want %q", hit.Snippet, "func simulateDepletion() {}")
	}
	if hit.Tag != "function" {
		t.Fatalf("Hit.Tag = %q, want %q", hit.Tag, "function")
	}
}

func TestSearchModeMapping(t *testing.T) {
	tests := []struct {
		name  string
		mode  Mode
		param string
	}{
		{name: "definition", mode: ModeDefinition, param: "def"},
		{name: "reference", mode: ModeReference, param: "symbol"},
		{name: "path", mode: ModePath, param: "path"},
		{name: "history", mode: ModeHistory, param: "hist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/search" {
					t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/search")
				}
				assertQueryValue(t, r.URL.Query(), tt.param, "needle")
				writeJSON(t, w, emptySearchResponse())
			}))
			defer server.Close()

			client := NewClient(server.URL+"/api/v1", server.Client())

			_, err := client.Search(context.Background(), SearchRequest{
				Projects: []string{"platform"},
				Query:    "needle",
				Mode:     tt.mode,
			})
			if err != nil {
				t.Fatalf("Search() error = %v, want nil", err)
			}
		})
	}
}

func TestSearchFileTypeAndPathPrefix(t *testing.T) {
	tests := []struct {
		name     string
		request  SearchRequest
		wantPath string
	}{
		{
			name: "path prefix sets path restriction for non-path mode",
			request: SearchRequest{
				Projects:   []string{"platform"},
				Query:      "simulateDepletion",
				Mode:       ModeFullText,
				PathPrefix: "src/services",
				FileType:   "swift",
			},
			wantPath: "src/services",
		},
		{
			name: "path mode query wins over path prefix",
			request: SearchRequest{
				Projects:   []string{"platform"},
				Query:      "Engine.swift",
				Mode:       ModePath,
				PathPrefix: "src/services",
				FileType:   "swift",
			},
			wantPath: "Engine.swift",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/search" {
					t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/search")
				}
				query := r.URL.Query()
				assertQueryValue(t, query, "type", "swift")
				assertQueryValue(t, query, "path", tt.wantPath)
				writeJSON(t, w, emptySearchResponse())
			}))
			defer server.Close()

			client := NewClient(server.URL+"/api/v1", server.Client())

			_, err := client.Search(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("Search() error = %v, want nil", err)
			}
		})
	}
}

func TestFileContentSendsRequestAndDecodesJSONOrRawText(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{
			name:        "JSON",
			body:        `{"contents":"one\ntwo\n"}`,
			contentType: "application/json",
		},
		{
			name:        "raw text",
			body:        "one\ntwo\n",
			contentType: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
				}
				if r.URL.Path != "/api/v1/file/content" {
					t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/file/content")
				}

				query := r.URL.Query()
				assertQueryValue(t, query, "project", "platform")
				assertQueryValue(t, query, "path", "src/Engine.swift")

				w.Header().Set("Content-Type", tt.contentType)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL+"/api/v1", server.Client())

			got, err := client.FileContent(context.Background(), "platform", "src/Engine.swift")
			if err != nil {
				t.Fatalf("FileContent() error = %v, want nil", err)
			}
			if got != "one\ntwo\n" {
				t.Fatalf("FileContent() = %q, want %q", got, "one\ntwo\n")
			}
		})
	}
}

func TestNon2xxResponseReturnsErrorWithoutLeakingAuthToken(t *testing.T) {
	const apiToken = "api-token-value"
	const basicToken = "basic-token-value"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithAPIToken(apiToken),
		WithBasicAuthToken(basicToken),
	)

	_, err := client.ListProjects(context.Background())
	if err == nil {
		t.Fatal("ListProjects() error = nil, want error")
	}

	if strings.Contains(err.Error(), apiToken) || strings.Contains(err.Error(), basicToken) {
		t.Fatalf("error %q leaked auth token", err.Error())
	}
}

func TestAPITokenOptionSetsBearerAuthorizationHeader(t *testing.T) {
	server := authHeaderServer(t, "Bearer api-token-value")
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client(), WithAPIToken("api-token-value"))

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func TestBasicAuthTokenOptionSetsBasicAuthorizationHeader(t *testing.T) {
	server := authHeaderServer(t, "Basic basic-token-value")
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client(), WithBasicAuthToken("basic-token-value"))

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}

func assertQueryValue(t *testing.T, values url.Values, key string, want string) {
	t.Helper()

	got := values[key]
	if !slices.Contains(got, want) {
		t.Fatalf("query %q = %#v, want to contain %q", key, got, want)
	}
}

func emptySearchResponse() map[string]any {
	return map[string]any{
		"resultCount":   0,
		"startDocument": 0,
		"endDocument":   0,
		"results":       map[string]any{},
	}
}

func authHeaderServer(t *testing.T, wantAuth string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/indexed" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/projects/indexed")
		}
		gotAuth := r.Header.Get("Authorization")
		if gotAuth != wantAuth {
			t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
		}

		writeJSON(t, w, []string{})
	}))
}

func TestBasicAuthTokenWinsWhenBothTokensAreConfigured(t *testing.T) {
	server := authHeaderServer(t, "Basic basic-token-value")
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithAPIToken("api-token-value"),
		WithBasicAuthToken("basic-token-value"),
	)

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func TestFlexibleLineNumberAcceptsJSONNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"resultCount":   1,
			"startDocument": 0,
			"endDocument":   0,
			"results": map[string]any{
				"/src/Engine.swift": []map[string]any{
					{
						"line":       "func run() {}",
						"lineNumber": 7,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	result, err := client.Search(context.Background(), SearchRequest{
		Projects: []string{"platform"},
		Query:    "run",
		Mode:     ModeFullText,
	})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if result.Hits[0].LineNumber != 7 {
		t.Fatalf("LineNumber = %d, want %d", result.Hits[0].LineNumber, 7)
	}
	if result.Hits[0].FilePath != "src/Engine.swift" {
		t.Fatalf("FilePath = %q, want %q", result.Hits[0].FilePath, "src/Engine.swift")
	}
	if result.Hits[0].Project != "platform" {
		t.Fatalf("Project = %q, want %q", result.Hits[0].Project, "platform")
	}
}

func TestNewClientDefaultsHTTPClient(t *testing.T) {
	client := NewClient("http://example.com", nil)

	if client == nil {
		t.Fatal("NewClient() = nil, want client")
	}
}
