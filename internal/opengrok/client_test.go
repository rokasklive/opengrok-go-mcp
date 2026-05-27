// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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
	if *hit.Snippet != "func simulateDepletion() {}" {
		t.Fatalf("Hit.Snippet = %q, want %q", *hit.Snippet, "func simulateDepletion() {}")
	}
	if hit.Tag != "function" {
		t.Fatalf("Hit.Tag = %q, want %q", hit.Tag, "function")
	}
}

func TestSearchAllProjectsDerivesProjectFromResultPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/search")
		}
		if got := r.URL.Query()["projects"]; len(got) != 0 {
			t.Fatalf("projects query = %#v, want absent", got)
		}

		writeJSON(t, w, map[string]any{
			"resultCount":   1,
			"startDocument": 0,
			"endDocument":   1,
			"results": map[string]any{
				"/platform/src/Engine.swift": []map[string]any{
					{
						"line":       "final class Engine {}",
						"lineNumber": 42,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	result, err := client.Search(context.Background(), SearchRequest{
		Query: "Engine",
		Mode:  ModeFullText,
	})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if len(result.Hits) != 1 {
		t.Fatalf("len(Hits) = %d, want 1", len(result.Hits))
	}

	hit := result.Hits[0]
	if hit.Project != "platform" {
		t.Fatalf("Hit.Project = %q, want %q", hit.Project, "platform")
	}
	if hit.FilePath != "src/Engine.swift" {
		t.Fatalf("Hit.FilePath = %q, want %q", hit.FilePath, "src/Engine.swift")
	}
}

func TestNormalizePathUsesLongestProjectPrefix(t *testing.T) {
	project, filePath, source, uncertain := normalizePath("/app-tools/src/Main.go", []string{"app", "app-tools"}, "app")
	if project != "app-tools" || filePath != "src/Main.go" || source != "matched_prefix" || uncertain {
		t.Fatalf("normalizePath() = (%q, %q, %q, %v), want app-tools/src/Main.go matched_prefix certain", project, filePath, source, uncertain)
	}
}

func TestNormalizePathMarksMultiProjectUnknownPathUncertain(t *testing.T) {
	project, filePath, source, uncertain := normalizePath("src/Main.go", []string{"app", "infra"}, "app")
	if project != "" || filePath != "src/Main.go" || source != "unknown" || !uncertain {
		t.Fatalf("normalizePath() = (%q, %q, %q, %v), want uncertain unknown unassigned path", project, filePath, source, uncertain)
	}
}

func TestNormalizePathAllowsSingleProjectUnprefixedPath(t *testing.T) {
	project, filePath, source, uncertain := normalizePath("src/Main.go", []string{"app"}, "fallback")
	if project != "app" || filePath != "src/Main.go" || source != "first_project_fallback" || uncertain {
		t.Fatalf("normalizePath() = (%q, %q, %q, %v), want single project first_project_fallback attribution", project, filePath, source, uncertain)
	}
}

func TestNormalizePathReturnsSource(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		projects       []string
		defaultProject string
		wantProject    string
		wantFilePath   string
		wantSource     string
		wantUncertain  bool
	}{
		{
			name:           "matched_prefix",
			path:           "/platform/src/main.go",
			projects:       []string{"platform"},
			defaultProject: "",
			wantProject:    "platform",
			wantFilePath:   "src/main.go",
			wantSource:     "matched_prefix",
			wantUncertain:  false,
		},
		{
			name:           "path_first_segment",
			path:           "/platform/src/main.go",
			projects:       []string{},
			defaultProject: "",
			wantProject:    "platform",
			wantFilePath:   "src/main.go",
			wantSource:     "path_first_segment",
			wantUncertain:  false,
		},
		{
			name:           "default_project_fallback",
			path:           "main.go",
			projects:       []string{},
			defaultProject: "platform",
			wantProject:    "platform",
			wantFilePath:   "main.go",
			wantSource:     "default_project_fallback",
			wantUncertain:  false,
		},
		{
			name:           "first_project_fallback",
			path:           "src/main.go",
			projects:       []string{"platform"},
			defaultProject: "",
			wantProject:    "platform",
			wantFilePath:   "src/main.go",
			wantSource:     "first_project_fallback",
			wantUncertain:  false,
		},
		{
			name:           "unknown_multi_project",
			path:           "src/main.go",
			projects:       []string{"platform", "infra"},
			defaultProject: "",
			wantProject:    "",
			wantFilePath:   "src/main.go",
			wantSource:     "unknown",
			wantUncertain:  true,
		},
		{
			name:           "unknown_no_projects_no_default",
			path:           "main.go",
			projects:       []string{},
			defaultProject: "",
			wantProject:    "",
			wantFilePath:   "main.go",
			wantSource:     "unknown",
			wantUncertain:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, filePath, source, uncertain := normalizePath(tt.path, tt.projects, tt.defaultProject)
			if project != tt.wantProject {
				t.Fatalf("project = %q, want %q", project, tt.wantProject)
			}
			if filePath != tt.wantFilePath {
				t.Fatalf("filePath = %q, want %q", filePath, tt.wantFilePath)
			}
			if source != tt.wantSource {
				t.Fatalf("source = %q, want %q", source, tt.wantSource)
			}
			if uncertain != tt.wantUncertain {
				t.Fatalf("uncertain = %v, want %v", uncertain, tt.wantUncertain)
			}
		})
	}
}

func TestSearchIncludesAttributionSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/search")
		}

		writeJSON(t, w, map[string]any{
			"resultCount":   1,
			"startDocument": 0,
			"endDocument":   1,
			"results": map[string]any{
				"/platform/src/Engine.swift": []map[string]any{
					{
						"line":       "final class Engine {}",
						"lineNumber": 42,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	result, err := client.Search(context.Background(), SearchRequest{
		Projects: []string{"platform"},
		Query:    "Engine",
		Mode:     ModeFullText,
	})
	if err != nil {
		t.Fatalf("Search() error = %v, want nil", err)
	}
	if len(result.Hits) != 1 {
		t.Fatalf("len(Hits) = %d, want 1", len(result.Hits))
	}

	hit := result.Hits[0]
	if hit.AttributionSource != "matched_prefix" {
		t.Fatalf("AttributionSource = %q, want %q", hit.AttributionSource, "matched_prefix")
	}
}

func TestSearchModeMapping(t *testing.T) {
	tests := []struct {
		name  string
		mode  Mode
		param string
	}{
		{name: "definition", mode: ModeDefinition, param: "def"},
		{name: "reference", mode: ModeReference, param: "refs"},
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
				if _, ok := query["project"]; ok {
					t.Fatalf("query project = %#v, want absent", query["project"])
				}
				assertQueryValue(t, query, "path", "platform/src/Engine.swift")

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

func TestFileContentFallsBackToRawWebURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/file/content":
			http.Error(w, "forbidden", http.StatusUnauthorized)
		case "/source/raw/platform/src/Engine.swift":
			if gotAuth := r.Header.Get("Authorization"); gotAuth != "Basic basic-token-value" {
				t.Fatalf("Authorization = %q, want Basic header", gotAuth)
			}
			if _, err := w.Write([]byte("one\ntwo\n")); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
		default:
			t.Fatalf("path = %q, want file API or raw web URL", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithWebBaseURL(server.URL+"/source"),
		WithBasicAuthToken("basic-token-value"),
	)

	got, err := client.FileContent(context.Background(), "platform", "src/Engine.swift")
	if err != nil {
		t.Fatalf("FileContent() error = %v, want nil", err)
	}
	if got != "one\ntwo\n" {
		t.Fatalf("FileContent() = %q, want raw content", got)
	}
}

func TestDoGETAcceptsResponseAtMaximumSize(t *testing.T) {
	body := bytes.Repeat([]byte("x"), maxResponseBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(body); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	got, _, err := client.doGET(context.Background(), server.URL, "/", "api")
	if err != nil {
		t.Fatalf("doGET() error = %v, want nil", err)
	}
	if len(got) != maxResponseBytes {
		t.Fatalf("doGET() body length = %d, want %d", len(got), maxResponseBytes)
	}
}

func TestListProjectsRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(bytes.Repeat([]byte("x"), maxResponseBytes+1)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())
	_, err := client.ListProjects(context.Background())
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("ListProjects() error = %v, want response limit error", err)
	}
}

func TestFileContentRawFallbackRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/file/content":
			http.Error(w, "not found", http.StatusNotFound)
		case "/source/raw/platform/src/Engine.swift":
			if _, err := w.Write(bytes.Repeat([]byte("x"), maxResponseBytes+1)); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
		default:
			t.Fatalf("path = %q, want API or raw web path", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithWebBaseURL(server.URL+"/source"),
	)
	content, err := client.FileContent(context.Background(), "platform", "src/Engine.swift")
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("FileContent() error = %v, want response limit error", err)
	}
	if content != "" {
		t.Fatalf("FileContent() = %q, want empty content on oversized response", content)
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

func TestBasicAuthTokenOptionAcceptsCompleteAuthorizationHeader(t *testing.T) {
	server := authHeaderServer(t, "Basic basic-token-value")
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client(), WithBasicAuthToken("Basic basic-token-value"))

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func TestBasicAuthTokenOptionTrimsWhitespace(t *testing.T) {
	server := authHeaderServer(t, "Basic basic-token-value")
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client(), WithBasicAuthToken(" basic-token-value\n"))

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func TestDebugOptionLogsAPICalls(t *testing.T) {
	server := authHeaderServer(t, "")
	defer server.Close()

	logs := []string{}
	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithDebugLogger(func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		}),
	)

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}

	joinedLogs := strings.Join(logs, "\n")
	for _, want := range []string{
		"opengrok api request method=GET",
		"/api/v1/projects/indexed",
		"opengrok api response method=GET",
		"status=200 OK",
	} {
		if !strings.Contains(joinedLogs, want) {
			t.Fatalf("debug logs = %q, want to contain %q", joinedLogs, want)
		}
	}
	if strings.Contains(joinedLogs, "Authorization") {
		t.Fatalf("debug logs = %q, want no Authorization header", joinedLogs)
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

func TestListFilesSendsRequestAndDecodesEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/v1/list" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/list")
		}
		assertQueryValue(t, r.URL.Query(), "path", "platform/src")

		writeJSON(t, w, []FileEntry{
			{
				Path:            "src/main.go",
				NumLines:        100,
				Loc:             80,
				Date:            1700000000,
				Description:     "main package",
				PathDescription: "Entry point",
				IsDirectory:     false,
				Size:            nil,
			},
			{
				Path:            "src/util",
				NumLines:        0,
				Loc:             0,
				Date:            1700000001,
				Description:     "",
				PathDescription: "",
				IsDirectory:     true,
				Size:            nil,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	entries, err := client.ListFiles(context.Background(), "platform", "src")
	if err != nil {
		t.Fatalf("ListFiles() error = %v, want nil", err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	if entries[0].Path != "src/main.go" {
		t.Fatalf("entries[0].Path = %q, want %q", entries[0].Path, "src/main.go")
	}
	if entries[0].NumLines != 100 {
		t.Fatalf("entries[0].NumLines = %d, want 100", entries[0].NumLines)
	}
	if entries[0].Loc != 80 {
		t.Fatalf("entries[0].Loc = %d, want 80", entries[0].Loc)
	}
	if entries[0].Date != 1700000000 {
		t.Fatalf("entries[0].Date = %d, want 1700000000", entries[0].Date)
	}
	if entries[0].Description != "main package" {
		t.Fatalf("entries[0].Description = %q, want %q", entries[0].Description, "main package")
	}
	if entries[0].PathDescription != "Entry point" {
		t.Fatalf("entries[0].PathDescription = %q, want %q", entries[0].PathDescription, "Entry point")
	}
	if entries[0].IsDirectory {
		t.Fatal("entries[0].IsDirectory = true, want false")
	}
	if entries[0].Size != nil {
		t.Fatalf("entries[0].Size = %v, want nil", *entries[0].Size)
	}

	if !entries[1].IsDirectory {
		t.Fatal("entries[1].IsDirectory = false, want true")
	}
}

func TestListFilesErrorReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	_, err := client.ListFiles(context.Background(), "platform", "src")
	if err == nil {
		t.Fatal("ListFiles() error = nil, want error")
	}
}

func TestRetryAfterHTTP500Succeeds(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count := callCount
		callCount++
		mu.Unlock()
		if count == 0 {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, []string{"platform"})
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}),
	)

	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v, want nil after retry", err)
	}
	if len(projects) != 1 || projects[0] != "platform" {
		t.Fatalf("ListProjects() = %#v, want [platform]", projects)
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2 (one failure + one retry)", callCount)
	}
}

func TestRetryAfterHTTP429Succeeds(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count := callCount
		callCount++
		mu.Unlock()
		if count == 0 {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, []string{"platform"})
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}),
	)

	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v, want nil after retry", err)
	}
	if len(projects) != 1 || projects[0] != "platform" {
		t.Fatalf("ListProjects() = %#v, want [platform]", projects)
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2 (one failure + one retry)", callCount)
	}
}

func TestNoRetryForHTTP401(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond}),
	)

	_, err := client.ListProjects(context.Background())
	if err == nil {
		t.Fatal("ListProjects() error = nil, want error for 401")
	}
	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1 (no retry for 4xx except 429)", callCount)
	}
}

func TestContextCancellationStopsBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 5, BaseDelay: 10 * time.Second}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.ListProjects(ctx)
	if err == nil {
		t.Fatal("ListProjects() error = nil, want context error")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestRetryDebugLoggingDoesNotLeakAuthToken(t *testing.T) {
	const apiToken = "secret-api-token-12345"

	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count := callCount
		callCount++
		mu.Unlock()
		// Return 500 on first call to trigger retry logging
		if count == 0 {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, []string{"platform"})
	}))
	defer server.Close()

	logs := []string{}
	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithAPIToken(apiToken),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}),
		WithDebugLogger(func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		}),
	)

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil after retry", err)
	}

	joinedLogs := strings.Join(logs, "\n")
	if strings.Contains(joinedLogs, apiToken) {
		t.Fatalf("debug logs leaked API token:\n%s", joinedLogs)
	}
	if !strings.Contains(joinedLogs, "retry") {
		t.Fatalf("debug logs = %q, want retry log entries", joinedLogs)
	}
}

func TestListFilesEmptyPathListsProjectRoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/list" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/list")
		}
		// path should be just "platform" without trailing slash
		got := r.URL.Query().Get("path")
		if got != "platform" {
			t.Fatalf("query path = %q, want %q", got, "platform")
		}

		writeJSON(t, w, []FileEntry{})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	entries, err := client.ListFiles(context.Background(), "platform", "")
	if err != nil {
		t.Fatalf("ListFiles() error = %v, want nil", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

func TestListFilesTruncatesAtMaxEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/v1/list" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/list")
		}

		entries := make([]FileEntry, 5005)
		for i := range entries {
			entries[i] = FileEntry{
				Path:     fmt.Sprintf("file_%d.go", i),
				NumLines: i,
			}
		}
		writeJSON(t, w, entries)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())

	entries, err := client.ListFiles(context.Background(), "testproject", "")
	if err != nil {
		t.Fatalf("ListFiles() error = %v, want nil", err)
	}

	if len(entries) != 5000 {
		t.Fatalf("len(entries) = %d, want 5000", len(entries))
	}
}

func TestListFilesWithMetadataReportsTruncation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := make([]FileEntry, maxFileListEntries+1)
		for i := range entries {
			entries[i] = FileEntry{Path: fmt.Sprintf("file_%d.go", i)}
		}
		writeJSON(t, w, entries)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())
	entries, truncated, err := client.ListFilesWithMetadata(context.Background(), "testproject", "")
	if err != nil {
		t.Fatalf("ListFilesWithMetadata() error = %v, want nil", err)
	}
	if len(entries) != maxFileListEntries {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxFileListEntries)
	}
	if !truncated {
		t.Fatal("truncated = false, want true")
	}
}

func TestListFilesTruncationDoesNotPanicWithoutDebugLogging(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ListFiles panicked with debugLogf=nil: %v", r)
		}
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/list" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/api/v1/list")
		}

		entries := make([]FileEntry, 5005)
		for i := range entries {
			entries[i] = FileEntry{
				Path:     fmt.Sprintf("file_%d.go", i),
				NumLines: i,
			}
		}
		writeJSON(t, w, entries)
	}))
	defer server.Close()

	// No WithDebug(true) — debugLogf is nil
	client := NewClient(server.URL+"/api/v1", server.Client())

	entries, err := client.ListFiles(context.Background(), "testproject", "")
	if err != nil {
		t.Fatalf("ListFiles() error = %v, want nil", err)
	}

	if len(entries) != 5000 {
		t.Fatalf("len(entries) = %d, want 5000", len(entries))
	}
}

func TestDoGETNon2xxReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client())
	_, err := client.ListProjects(context.Background())
	if err == nil {
		t.Fatal("ListProjects() error = nil, want error")
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error type = %T, want *StatusError", err)
	}
	if statusErr.Code != http.StatusUnauthorized {
		t.Fatalf("StatusError.Code = %d, want %d", statusErr.Code, http.StatusUnauthorized)
	}
}
