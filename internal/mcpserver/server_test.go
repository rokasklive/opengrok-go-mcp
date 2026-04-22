package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

type fakeBackend struct {
	projects []string

	searchRequests []opengrok.SearchRequest
	searchResult   opengrok.SearchResult
	searchErr      error

	fileContent string
	fileErr     error
	fileProject string
	filePath    string
}

func (b *fakeBackend) ListProjects(context.Context) ([]string, error) {
	return b.projects, nil
}

func (b *fakeBackend) Search(_ context.Context, req opengrok.SearchRequest) (opengrok.SearchResult, error) {
	b.searchRequests = append(b.searchRequests, req)
	if b.searchErr != nil {
		return opengrok.SearchResult{Hits: []opengrok.Hit{}}, b.searchErr
	}

	return b.searchResult, nil
}

func (b *fakeBackend) FileContent(_ context.Context, project string, filePath string) (string, error) {
	b.fileProject = project
	b.filePath = filePath
	if b.fileErr != nil {
		return "", b.fileErr
	}

	return b.fileContent, nil
}

func TestReadFileResourceMatchesSlashContainingPath(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{
		fileContent: "final class Engine {}",
	}
	server := NewMCPServer(testConfig(), backend, "test")
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect returned error: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect returned error: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "opengrok://project/platform/files/src/services/Engine.swift",
	})
	if err != nil {
		t.Fatalf("ReadResource returned error: %v", err)
	}
	if backend.fileProject != "platform" {
		t.Fatalf("FileContent project = %q, want platform", backend.fileProject)
	}
	if backend.filePath != "src/services/Engine.swift" {
		t.Fatalf("FileContent path = %q, want src/services/Engine.swift", backend.filePath)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(result.Contents))
	}

	var output FileContextOutput
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &output); err != nil {
		t.Fatalf("resource JSON unmarshal returned error: %v", err)
	}
	if output.Content != "final class Engine {}" {
		t.Fatalf("content = %q, want file body", output.Content)
	}
}

func TestReadFileResourceLineFragmentSelectsContext(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\n",
	}
	server := NewMCPServer(testConfig(), backend, "test")
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect returned error: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect returned error: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "opengrok://project/platform/files/src/services/Engine.swift#L2",
	})
	if err != nil {
		t.Fatalf("ReadResource returned error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(result.Contents))
	}

	var output FileContextOutput
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &output); err != nil {
		t.Fatalf("resource JSON unmarshal returned error: %v", err)
	}
	if output.LineNumber != 2 {
		t.Fatalf("LineNumber = %d, want 2", output.LineNumber)
	}
	if output.Content != "one\ntwo\nthree" {
		t.Fatalf("Content = %q, want selected context around line 2", output.Content)
	}
}

func TestGetFileContextSlicesAroundLine(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\nfour\nfive\n",
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:    "platform",
		FilePath:   "src/Engine.swift",
		LineNumber: 3,
		Before:     1,
		After:      1,
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if output.StartLine != 2 {
		t.Fatalf("StartLine = %d, want 2", output.StartLine)
	}
	if output.EndLine != 4 {
		t.Fatalf("EndLine = %d, want 4", output.EndLine)
	}
	if output.Content != "two\nthree\nfour" {
		t.Fatalf("Content = %q, want selected lines", output.Content)
	}
	if output.DisplayURL != "https://grok.example.com/source/xref/platform/src/Engine.swift#3" {
		t.Fatalf("DisplayURL = %q, want line anchor", output.DisplayURL)
	}
	if output.ResourceURI != "opengrok://project/platform/files/src/Engine.swift#L3" {
		t.Fatalf("ResourceURI = %q, want line anchor", output.ResourceURI)
	}
}

func TestGetFileContextWithoutLineNumberReturnsFullFile(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\n",
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "src/Engine.swift",
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if output.StartLine != 1 {
		t.Fatalf("StartLine = %d, want 1", output.StartLine)
	}
	if output.EndLine != 3 {
		t.Fatalf("EndLine = %d, want 3", output.EndLine)
	}
	if output.Content != "one\ntwo\nthree\n" {
		t.Fatalf("Content = %q, want full file", output.Content)
	}
	if output.DisplayURL != "https://grok.example.com/source/xref/platform/src/Engine.swift" {
		t.Fatalf("DisplayURL = %q, want file URL without anchor", output.DisplayURL)
	}
	if output.ResourceURI != "opengrok://project/platform/files/src/Engine.swift" {
		t.Fatalf("ResourceURI = %q, want file resource without anchor", output.ResourceURI)
	}
}

func TestGetFileContextIncludeLinksFalseSuppressesBrowserLinks(t *testing.T) {
	includeLinks := false
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\n",
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:      "platform",
		FilePath:     "src/Engine.swift",
		LineNumber:   2,
		IncludeLinks: &includeLinks,
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if output.DisplayURL != "" {
		t.Fatalf("DisplayURL = %q, want empty", output.DisplayURL)
	}
	if output.RawURL != nil {
		t.Fatalf("RawURL = %q, want nil", *output.RawURL)
	}
	if output.ResourceURI != "opengrok://project/platform/files/src/Engine.swift#L2" {
		t.Fatalf("ResourceURI = %q, want resource URI", output.ResourceURI)
	}
}

func TestGetFileContextReturnsProjectRequired(t *testing.T) {
	service := NewService(testConfig(), &fakeBackend{})

	_, err := service.GetFileContext(context.Background(), FileContextInput{
		FilePath: "src/Engine.swift",
	})
	if err == nil {
		t.Fatal("GetFileContext error is nil, want PROJECT_REQUIRED")
	}
	if !IsCode(err, "PROJECT_REQUIRED") {
		t.Fatalf("GetFileContext error = %v, want PROJECT_REQUIRED", err)
	}
}

func TestGetFileContextProjectRequiredFalseStillRequiresProject(t *testing.T) {
	cfg := testConfig()
	cfg.ProjectRequired = false
	service := NewService(cfg, &fakeBackend{})

	_, err := service.GetFileContext(context.Background(), FileContextInput{
		FilePath: "src/Engine.swift",
	})
	if err == nil {
		t.Fatal("GetFileContext error is nil, want PROJECT_REQUIRED")
	}
	if !IsCode(err, "PROJECT_REQUIRED") {
		t.Fatalf("GetFileContext error = %v, want PROJECT_REQUIRED", err)
	}
}

func TestGetFileContextUsesDefaultProject(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\n",
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		FilePath: "src/Engine.swift",
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if backend.fileProject != "platform" {
		t.Fatalf("backend project = %q, want platform", backend.fileProject)
	}
	if output.Project != "platform" {
		t.Fatalf("Project = %q, want platform", output.Project)
	}
}

func TestGetFileContextNegativeBeforeAfterClampToZero(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\n",
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:    "platform",
		FilePath:   "src/Engine.swift",
		LineNumber: 2,
		Before:     -10,
		After:      -20,
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if output.StartLine != 2 {
		t.Fatalf("StartLine = %d, want 2", output.StartLine)
	}
	if output.EndLine != 2 {
		t.Fatalf("EndLine = %d, want 2", output.EndLine)
	}
	if output.Content != "two" {
		t.Fatalf("Content = %q, want selected line", output.Content)
	}
}

func TestGetFileContextLineBeyondEOFAnchorsToClampedLine(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\ntwo\n",
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:    "platform",
		FilePath:   "src/Engine.swift",
		LineNumber: 99,
		Before:     1,
		After:      1,
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if output.StartLine != 2 {
		t.Fatalf("StartLine = %d, want 2", output.StartLine)
	}
	if output.EndLine != 2 {
		t.Fatalf("EndLine = %d, want 2", output.EndLine)
	}
	if output.Content != "two" {
		t.Fatalf("Content = %q, want selected line", output.Content)
	}
	if output.LineNumber != 2 {
		t.Fatalf("LineNumber = %d, want 2", output.LineNumber)
	}
	if output.DisplayURL != "https://grok.example.com/source/xref/platform/src/Engine.swift#2" {
		t.Fatalf("DisplayURL = %q, want clamped line anchor", output.DisplayURL)
	}
	if output.ResourceURI != "opengrok://project/platform/files/src/Engine.swift#L2" {
		t.Fatalf("ResourceURI = %q, want clamped line anchor", output.ResourceURI)
	}
}

func TestSearchCodeUsesDefaultProjectAndBuildsNextCursor(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 25,
			Start:     0,
			End:       20,
			Hits: []opengrok.Hit{
				{
					Project:    "platform",
					FilePath:   "src/Engine.swift",
					LineNumber: 42,
					Snippet:    "final class Engine {}",
				},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "Engine",
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if len(backend.searchRequests) != 1 {
		t.Fatalf("backend.Search calls = %d, want 1", len(backend.searchRequests))
	}
	gotReq := backend.searchRequests[0]
	if gotReq.Projects[0] != "platform" {
		t.Fatalf("project = %q, want platform", gotReq.Projects[0])
	}
	if gotReq.Limit != 20 {
		t.Fatalf("limit = %d, want 20", gotReq.Limit)
	}
	if output.NextCursor == nil || *output.NextCursor == "" {
		t.Fatal("NextCursor is empty, want non-empty cursor")
	}
	if len(output.Results) != 1 {
		t.Fatalf("results length = %d, want 1", len(output.Results))
	}
	if output.Results[0].DisplayURL != "https://grok.example.com/source/xref/platform/src/Engine.swift#42" {
		t.Fatalf("display URL = %q", output.Results[0].DisplayURL)
	}
}

func TestSearchCodeReturnsProjectRequired(t *testing.T) {
	service := NewService(testConfig(), &fakeBackend{})

	_, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err == nil {
		t.Fatal("SearchCode error is nil, want PROJECT_REQUIRED")
	}
	if !IsCode(err, "PROJECT_REQUIRED") {
		t.Fatalf("SearchCode error = %v, want PROJECT_REQUIRED", err)
	}
}

func TestSearchCodeProjectRequiredFalseAllowsAllProjectSearch(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{
					Project:    "platform",
					FilePath:   "src/Engine.swift",
					LineNumber: 42,
					Snippet:    "final class Engine {}",
				},
			},
		},
	}
	cfg := testConfig()
	cfg.ProjectRequired = false
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "Engine",
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}
	if len(backend.searchRequests) != 1 {
		t.Fatalf("backend.Search calls = %d, want 1", len(backend.searchRequests))
	}
	if len(backend.searchRequests[0].Projects) != 0 {
		t.Fatalf("backend projects = %#v, want empty", backend.searchRequests[0].Projects)
	}
	if output.Project != "" {
		t.Fatalf("Project = %q, want empty", output.Project)
	}
	if len(output.Results) != 1 {
		t.Fatalf("results length = %d, want 1", len(output.Results))
	}
	if output.Results[0].Project != "platform" {
		t.Fatalf("result project = %q, want platform", output.Results[0].Project)
	}
}

func TestSearchCodeProjectRequiredFalseCursorKeepsAllProjectSearch(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 45,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.ProjectRequired = false
	service := NewService(cfg, backend)

	firstPage, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "Engine",
	})
	if err != nil {
		t.Fatalf("first SearchCode returned error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first SearchCode returned nil cursor, want next cursor")
	}

	secondPage, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:  "Engine",
		Cursor: *firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("second SearchCode returned error: %v", err)
	}

	gotReq := backend.searchRequests[1]
	if len(gotReq.Projects) != 0 {
		t.Fatalf("backend projects = %#v, want empty", gotReq.Projects)
	}
	if gotReq.Offset != 20 {
		t.Fatalf("offset = %d, want 20", gotReq.Offset)
	}
	if secondPage.Project != "" {
		t.Fatalf("Project = %q, want empty", secondPage.Project)
	}
}

func TestSearchCodeExplicitProjectBeatsDefault(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}},
	}
	cfg := testConfig()
	cfg.DefaultProject = "default"
	service := NewService(cfg, backend)

	_, err := service.SearchCode(context.Background(), SearchCodeInput{
		Project: "explicit",
		Query:   "Engine",
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	got := backend.searchRequests[0].Projects
	if len(got) != 1 || got[0] != "explicit" {
		t.Fatalf("projects = %#v, want [explicit]", got)
	}
}

func TestSearchCodeCursorSecondPageUsesCursorOffset(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 45,
			Start:     20,
			End:       40,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	firstPage, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "Engine",
	})
	if err != nil {
		t.Fatalf("first SearchCode returned error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first SearchCode returned nil cursor, want next cursor")
	}

	secondPage, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:  "Engine",
		Cursor: *firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("second SearchCode returned error: %v", err)
	}

	gotReq := backend.searchRequests[1]
	if gotReq.Offset != 20 {
		t.Fatalf("offset = %d, want 20", gotReq.Offset)
	}
	if secondPage.Diagnostics.OffsetUsed != 20 {
		t.Fatalf("diagnostics offset = %d, want 20", secondPage.Diagnostics.OffsetUsed)
	}
	if secondPage.Diagnostics.OpenGrokStart != 20 {
		t.Fatalf("diagnostics start = %d, want 20", secondPage.Diagnostics.OpenGrokStart)
	}
	if secondPage.Diagnostics.OpenGrokMaxResults != 20 {
		t.Fatalf("diagnostics max results = %d, want 20", secondPage.Diagnostics.OpenGrokMaxResults)
	}
}

func TestSearchCodeCursorPageSizeIsCapped(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 100,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	cfg.PageSizeMax = 50
	service := NewService(cfg, backend)
	encodedCursor, err := cursor.Encode(cursor.State{
		Project:  "platform",
		Projects: []string{"platform"},
		Query:    "Engine",
		Mode:     "full_text",
		Offset:   20,
		PageSize: 500,
	})
	if err != nil {
		t.Fatalf("cursor Encode returned error: %v", err)
	}

	_, err = service.SearchCode(context.Background(), SearchCodeInput{
		Query:  "Engine",
		Cursor: encodedCursor,
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if backend.searchRequests[0].Limit != 50 {
		t.Fatalf("limit = %d, want capped max 50", backend.searchRequests[0].Limit)
	}
}

func TestSearchCodeCursorRejectsMismatchedProjects(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 45,
			Hits:      []opengrok.Hit{},
		},
	}
	service := NewService(testConfig(), backend)

	firstPage, err := service.SearchCode(context.Background(), SearchCodeInput{
		Projects: []string{"platform", "tools"},
		Query:    "Engine",
	})
	if err != nil {
		t.Fatalf("first SearchCode returned error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first SearchCode returned nil cursor, want next cursor")
	}

	_, err = service.SearchCode(context.Background(), SearchCodeInput{
		Projects: []string{"platform", "other"},
		Query:    "Engine",
		Cursor:   *firstPage.NextCursor,
	})
	if err == nil {
		t.Fatal("second SearchCode error is nil, want INVALID_CURSOR")
	}
	if !IsCode(err, "INVALID_CURSOR") {
		t.Fatalf("second SearchCode error = %v, want INVALID_CURSOR", err)
	}
}

func TestSearchSymbolDefinitionsUsesDefinitionModeAndSetsSymbolKind(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{
					Project:    "platform",
					FilePath:   "src/Engine.swift",
					LineNumber: 42,
					Snippet:    "final class Engine {}",
				},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchSymbolDefinitions(context.Background(), SymbolSearchInput{
		Symbol: "Engine",
	})
	if err != nil {
		t.Fatalf("SearchSymbolDefinitions returned error: %v", err)
	}

	gotReq := backend.searchRequests[0]
	if gotReq.Mode != opengrok.ModeDefinition {
		t.Fatalf("mode = %q, want definition", gotReq.Mode)
	}
	if gotReq.Query != "Engine" {
		t.Fatalf("query = %q, want Engine", gotReq.Query)
	}
	if output.Results[0].Kind != "definition" {
		t.Fatalf("kind = %q, want definition", output.Results[0].Kind)
	}
	if output.Results[0].Symbol == nil || *output.Results[0].Symbol != "Engine" {
		t.Fatalf("symbol = %#v, want Engine", output.Results[0].Symbol)
	}
}

func TestListProjectsReturnsResourceURIs(t *testing.T) {
	backend := &fakeBackend{
		projects: []string{"platform", "tools"},
	}
	service := NewService(testConfig(), backend)

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	if len(output.Projects) != 2 {
		t.Fatalf("projects length = %d, want 2", len(output.Projects))
	}
	if output.Projects[0].ResourceURI != "opengrok://project/platform" {
		t.Fatalf("resource URI = %q, want project URI", output.Projects[0].ResourceURI)
	}
	if output.Projects[0].ProjectURL == "" {
		t.Fatal("project URL is empty, want stable search URL")
	}
}

func TestNewMCPServerReturnsServer(t *testing.T) {
	server := NewMCPServer(testConfig(), &fakeBackend{}, "test")
	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	return cfg
}
