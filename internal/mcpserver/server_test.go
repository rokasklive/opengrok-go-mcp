// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

type fakeBackend struct {
	projects    []string
	projectsErr error

	searchRequests []opengrok.SearchRequest
	searchResult   opengrok.SearchResult
	searchErr      error

	fileContent   string
	fileContents  map[string]string // key: "project:filePath"
	fileErr       error
	fileErrors    map[string]error // key: "project:filePath"
	panicFileRead bool
	fileProject   string
	filePath      string
	fileCallCount int
	mu            sync.Mutex

	fileEntries       []opengrok.FileEntry
	fileListProject   string
	fileListPath      string
	fileListErr       error
	fileListTruncated bool

	projectOverview    opengrok.ProjectOverview
	projectOverviewErr error
}

func (b *fakeBackend) ListProjects(context.Context) ([]string, error) {
	if b.projectsErr != nil {
		return nil, b.projectsErr
	}
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
	if b.panicFileRead {
		panic("file decoder panic")
	}
	b.mu.Lock()
	b.fileProject = project
	b.filePath = filePath
	b.fileCallCount++
	b.mu.Unlock()
	key := project + ":" + filePath
	if b.fileErrors != nil {
		if err, ok := b.fileErrors[key]; ok {
			return "", err
		}
	}
	if b.fileErr != nil {
		return "", b.fileErr
	}
	if b.fileContents != nil {
		if content, ok := b.fileContents[key]; ok {
			return content, nil
		}
	}
	return b.fileContent, nil
}

func (b *fakeBackend) ListFiles(_ context.Context, project string, path string) ([]opengrok.FileEntry, error) {
	b.fileListProject = project
	b.fileListPath = path
	if b.fileListErr != nil {
		return nil, b.fileListErr
	}
	return b.fileEntries, nil
}

func (b *fakeBackend) ListFilesWithMetadata(ctx context.Context, project string, path string) ([]opengrok.FileEntry, bool, error) {
	entries, err := b.ListFiles(ctx, project, path)
	return entries, b.fileListTruncated, err
}

func (b *fakeBackend) GetProjectOverview(_ context.Context, project string) (opengrok.ProjectOverview, error) {
	if b.projectOverviewErr != nil {
		return opengrok.ProjectOverview{}, b.projectOverviewErr
	}
	return b.projectOverview, nil
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
	if output.Citation.URL != "https://grok.example.com/source/xref/platform/src/Engine.swift#3" {
		t.Fatalf("Citation.URL = %q, want display URL", output.Citation.URL)
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
	if output.Citation.URL != "https://grok.example.com/source/xref/platform/src/Engine.swift#2" {
		t.Fatalf("Citation.URL = %q, want display URL even when links are suppressed", output.Citation.URL)
	}
	if output.ResourceURI != "opengrok://project/platform/files/src/Engine.swift#L2" {
		t.Fatalf("ResourceURI = %q, want resource URI", output.ResourceURI)
	}
}

func TestGetFileContextReturnsProjectRequired(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultProject = ""
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

func TestGetFileContextProjectRequiredFalseStillRequiresProject(t *testing.T) {
	cfg := testConfig()
	cfg.ProjectRequired = false
	cfg.DefaultProject = ""
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
					Snippet:    strPtr("final class Engine {}"),
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
	if output.Results[0].Citation.URL != "https://grok.example.com/source/xref/platform/src/Engine.swift#42" {
		t.Fatalf("Citation.URL = %q, want display URL", output.Results[0].Citation.URL)
	}
}

func TestSearchCodeTreatsBlankCursorAsFirstPage(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)
	blankCursor := "   "

	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:  "Engine",
		Cursor: &blankCursor,
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if output.Diagnostics.OffsetUsed != 0 {
		t.Fatalf("OffsetUsed = %d, want first page offset", output.Diagnostics.OffsetUsed)
	}
	if backend.searchRequests[0].Offset != 0 {
		t.Fatalf("request offset = %d, want first page offset", backend.searchRequests[0].Offset)
	}
}

func TestSearchCodeRejectsUnknownConfiguredProject(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultProject = "bam-bam-default"
	cfg.Projects = []string{"bam-bam-default"}
	service := NewService(cfg, &fakeBackend{})

	_, err := service.SearchCode(context.Background(), SearchCodeInput{
		Project: "bam-balam-main",
		Query:   "TaskAdaptor",
	})
	if err == nil {
		t.Fatal("SearchCode error is nil, want UNKNOWN_PROJECT")
	}
	if !IsCode(err, "UNKNOWN_PROJECT") {
		t.Fatalf("SearchCode error = %v, want UNKNOWN_PROJECT", err)
	}
	if !strings.Contains(err.Error(), "Omit project to use the default project") {
		t.Fatalf("SearchCode error = %q, want corrective guidance", err.Error())
	}
}

func TestSearchCodeUsesDefaultWhenProjectOmittedWithConfiguredProjects(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}},
	}
	cfg := testConfig()
	cfg.DefaultProject = "bam-bam-default"
	cfg.Projects = []string{"bam-bam-default"}
	service := NewService(cfg, backend)

	_, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "TaskAdaptor",
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}
	if got := backend.searchRequests[0].Projects; len(got) != 1 || got[0] != "bam-bam-default" {
		t.Fatalf("backend projects = %#v, want default configured project", got)
	}
}

func TestSearchCodeReturnsProjectRequired(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultProject = ""
	service := NewService(cfg, &fakeBackend{})

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
					Snippet:    strPtr("final class Engine {}"),
				},
			},
		},
	}
	cfg := testConfig()
	cfg.ProjectRequired = false
	cfg.DefaultProject = ""
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
	cfg.DefaultProject = ""
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
		Cursor: firstPage.NextCursor,
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
		Cursor: firstPage.NextCursor,
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
		Cursor: &encodedCursor,
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
		Cursor:   firstPage.NextCursor,
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
					Snippet:    strPtr("final class Engine {}"),
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
	if output.Results[0].Kind != "" {
		t.Fatalf("kind = %q, want empty string (no ctags tag on this hit)", output.Results[0].Kind)
	}
	if output.Results[0].Symbol == nil || *output.Results[0].Symbol != "Engine" {
		t.Fatalf("symbol = %#v, want Engine", output.Results[0].Symbol)
	}
}

func TestFindSymbolAndReferencesPaginatesReferencesOnly(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 18,
			Hits: []opengrok.Hit{
				{
					Project:    "platform",
					FilePath:   "src/Engine.swift",
					LineNumber: 42,
					Snippet:    strPtr("final class Engine {}"),
				},
			},
		},
		fileContent: "final class Engine {}\n",
	}
	service := NewService(testConfig(), backend)

	firstPage, err := service.FindSymbolAndReferences(context.Background(), FindSymbolAndReferencesInput{
		Symbol:   "Engine",
		PageSize: 7,
	})
	if err != nil {
		t.Fatalf("first FindSymbolAndReferences returned error: %v", err)
	}
	if firstPage.Definition == nil {
		t.Fatal("first Definition is nil, want repeated definition context")
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first NextCursor is nil, want reference pagination cursor")
	}

	secondPage, err := service.FindSymbolAndReferences(context.Background(), FindSymbolAndReferencesInput{
		Symbol: "Engine",
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("second FindSymbolAndReferences returned error: %v", err)
	}
	if secondPage.Definition == nil {
		t.Fatal("second Definition is nil, want repeated definition context")
	}

	want := []struct {
		mode   opengrok.Mode
		limit  int
		offset int
	}{
		{mode: opengrok.ModeDefinition, limit: 20, offset: 0},
		{mode: opengrok.ModeReference, limit: 7, offset: 0},
		{mode: opengrok.ModeDefinition, limit: 20, offset: 0},
		{mode: opengrok.ModeReference, limit: 7, offset: 7},
	}
	if len(backend.searchRequests) != len(want) {
		t.Fatalf("backend.Search calls = %d, want %d", len(backend.searchRequests), len(want))
	}
	for i, wantReq := range want {
		gotReq := backend.searchRequests[i]
		if gotReq.Mode != wantReq.mode || gotReq.Limit != wantReq.limit || gotReq.Offset != wantReq.offset {
			t.Fatalf(
				"request %d = mode %q limit %d offset %d, want mode %q limit %d offset %d",
				i,
				gotReq.Mode,
				gotReq.Limit,
				gotReq.Offset,
				wantReq.mode,
				wantReq.limit,
				wantReq.offset,
			)
		}
	}
}

func TestFindSymbolAndReferencesRejectsInvalidCursorBeforeDefinitionWork(t *testing.T) {
	invalidCursor := "not-a-valid-cursor!!!"
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{
					Project:    "platform",
					FilePath:   "src/Engine.swift",
					LineNumber: 42,
				},
			},
		},
		fileContent: "final class Engine {}\n",
	}
	service := NewService(testConfig(), backend)

	_, err := service.FindSymbolAndReferences(context.Background(), FindSymbolAndReferencesInput{
		Symbol: "Engine",
		Cursor: &invalidCursor,
	})
	if err == nil {
		t.Fatal("FindSymbolAndReferences error is nil, want INVALID_CURSOR")
	}
	if !IsCode(err, "INVALID_CURSOR") {
		t.Fatalf("FindSymbolAndReferences error = %v, want INVALID_CURSOR", err)
	}
	if len(backend.searchRequests) != 0 {
		t.Fatalf("backend.Search calls = %d, want 0", len(backend.searchRequests))
	}
	if backend.fileCallCount != 0 {
		t.Fatalf("backend.FileContent calls = %d, want 0", backend.fileCallCount)
	}
}

func TestFindSymbolAndReferencesRejectsResponseModeBeforeInvalidCursor(t *testing.T) {
	invalidCursor := "not-a-valid-cursor!!!"
	backend := &fakeBackend{}
	service := NewService(testConfig(), backend)

	_, err := service.FindSymbolAndReferences(context.Background(), FindSymbolAndReferencesInput{
		Symbol:       "Engine",
		ResponseMode: "invalid",
		Cursor:       &invalidCursor,
	})
	if err == nil {
		t.Fatal("FindSymbolAndReferences error is nil, want INVALID_RESPONSE_MODE")
	}
	if !IsCode(err, "INVALID_RESPONSE_MODE") {
		t.Fatalf("FindSymbolAndReferences error = %v, want INVALID_RESPONSE_MODE", err)
	}
	if len(backend.searchRequests) != 0 {
		t.Fatalf("backend.Search calls = %d, want 0", len(backend.searchRequests))
	}
	if backend.fileCallCount != 0 {
		t.Fatalf("backend.FileContent calls = %d, want 0", backend.fileCallCount)
	}
}

func TestFindSymbolAndReferencesRejectsMismatchedCursorBeforeDefinitionWork(t *testing.T) {
	mismatchedCursor, err := cursor.Encode(cursor.State{
		Project:  "platform",
		Projects: []string{"platform"},
		Query:    "DifferentEngine",
		Mode:     string(opengrok.ModeReference),
		Offset:   7,
		PageSize: 7,
	})
	if err != nil {
		t.Fatalf("cursor Encode returned error: %v", err)
	}

	backend := &fakeBackend{}
	service := NewService(testConfig(), backend)

	_, err = service.FindSymbolAndReferences(context.Background(), FindSymbolAndReferencesInput{
		Symbol: "Engine",
		Cursor: &mismatchedCursor,
	})
	if err == nil {
		t.Fatal("FindSymbolAndReferences error is nil, want INVALID_CURSOR")
	}
	if !IsCode(err, "INVALID_CURSOR") {
		t.Fatalf("FindSymbolAndReferences error = %v, want INVALID_CURSOR", err)
	}
	if len(backend.searchRequests) != 0 {
		t.Fatalf("backend.Search calls = %d, want 0", len(backend.searchRequests))
	}
	if backend.fileCallCount != 0 {
		t.Fatalf("backend.FileContent calls = %d, want 0", backend.fileCallCount)
	}
}

func TestListProjectsReturnsResourceURIs(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	service := NewService(cfg, &fakeBackend{})

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

func TestListProjectsUsesConfiguredProjects(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	cfg.ProjectSource = config.ProjectSourceConfigured
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	got := []string{}
	for _, project := range output.Projects {
		got = append(got, project.Project)
	}
	if !slices.Equal(got, []string{"platform", "tools"}) {
		t.Fatalf("projects = %#v, want configured projects", got)
	}
}

func TestListProjectsSnapshotMatchesSearchValidation(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	cfg.ProjectSource = config.ProjectSourceAPI
	cfg.DefaultProject = "platform"
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	listed := make(map[string]bool)
	for _, item := range output.Projects {
		listed[item.Project] = true
		if err := service.validateConfiguredProjects([]string{item.Project}); err != nil {
			t.Fatalf("listed project %q rejected by validation: %v", item.Project, err)
		}
	}

	if err := service.validateConfiguredProjects([]string{"unknown"}); err == nil {
		t.Fatal("validateConfiguredProjects() error = nil, want UNKNOWN_PROJECT")
	} else if !IsCode(err, codeUnknownProject) {
		t.Fatalf("validateConfiguredProjects() error = %v, want UNKNOWN_PROJECT", err)
	}
	if listed["unknown"] {
		t.Fatal("unknown project appears in list_projects output")
	}
}

func TestNewMCPServerRegistersOnlyEnabledTools(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{
		SearchCode:              true,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
		GetFileContext:          true,
		Memory:                  true,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	got := []string{}
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	want := []string{"search_code", "search_symbol_definitions", "search_symbol_references", "get_file_context", "read_file", "search_and_read", "find_symbol_and_references", "search_implementations", "search_cross_project_references", "memory_set", "memory_get", "memory_list", "memory_delete", "memory_clear"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestFullSurfaceRegistersProjectOverviewOnlyWithListFiles(t *testing.T) {
	tests := []struct {
		name         string
		capabilities config.Capabilities
		wantPresent  bool
	}{
		{
			name:         "list projects only",
			capabilities: config.Capabilities{ListProjects: true},
			wantPresent:  false,
		},
		{
			name:         "list files only",
			capabilities: config.Capabilities{ListFiles: true},
			wantPresent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.ToolSurface = config.ToolSurfaceFull
			cfg.Capabilities = tt.capabilities
			server := NewMCPServer(cfg, &fakeBackend{}, "test")
			clientSession, cleanup := connectMCPServer(t, server)
			defer cleanup()

			tools, err := clientSession.ListTools(context.Background(), nil)
			if err != nil {
				t.Fatalf("ListTools returned error: %v", err)
			}

			gotPresent := false
			for _, tool := range tools.Tools {
				if tool.Name == "get_project_overview" {
					gotPresent = true
					break
				}
			}
			if gotPresent != tt.wantPresent {
				t.Fatalf("get_project_overview present = %t, want %t", gotPresent, tt.wantPresent)
			}
		})
	}
}

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

func TestGatewayRegistryDoesNotExposeContentOrMemoryWithoutCapabilities(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{
		SearchCode:              true,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
	}
	registry := buildGatewayRegistry(NewService(cfg, &fakeBackend{}), cfg)

	for _, operation := range []string{
		"compound.search_and_read",
		"compound.find_symbol_and_references",
		"memory.set",
		"memory.get",
		"memory.list",
		"memory.delete",
		"memory.clear",
	} {
		if _, ok := registry[operation]; ok {
			t.Fatalf("operation %q registered without required capability", operation)
		}
	}
}

func TestGatewayRegistryRegistersProjectOverviewOnlyWithListFiles(t *testing.T) {
	tests := []struct {
		name         string
		capabilities config.Capabilities
		wantPresent  bool
	}{
		{
			name:         "list projects only",
			capabilities: config.Capabilities{ListProjects: true},
			wantPresent:  false,
		},
		{
			name:         "list files only",
			capabilities: config.Capabilities{ListFiles: true},
			wantPresent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.ToolSurface = config.ToolSurfaceGateway
			cfg.Capabilities = tt.capabilities
			registry := buildGatewayRegistry(NewService(cfg, &fakeBackend{}), cfg)

			_, gotPresent := registry["project.overview"]
			if gotPresent != tt.wantPresent {
				t.Fatalf("project.overview present = %t, want %t", gotPresent, tt.wantPresent)
			}
		})
	}
}

func TestHTTPTransportDoesNotExposeProcessScopedMemory(t *testing.T) {
	cfg := testConfig()
	cfg.Transport = config.TransportHTTP
	cfg.Capabilities = config.Capabilities{Memory: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	for _, tool := range tools.Tools {
		if strings.HasPrefix(tool.Name, "memory_") {
			t.Fatalf("process-scoped memory tool %q exposed over HTTP", tool.Name)
		}
	}

	cfg.ToolSurface = config.ToolSurfaceGateway
	registry := buildGatewayRegistry(NewService(cfg, &fakeBackend{}), cfg)
	if _, ok := registry["memory.get"]; ok {
		t.Fatal("process-scoped memory gateway operation exposed over HTTP")
	}
}

func TestNewMCPServerSearchCursorIsOptionalInToolSchema(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{
		SearchSymbolDefinitions: true,
	}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools.Tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(tools.Tools))
	}

	schema, ok := tools.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("InputSchema type = %T, want map", tools.Tools[0].InputSchema)
	}
	required, _ := schema["required"].([]any)
	for _, field := range required {
		if field == "cursor" || field == "include_links" {
			t.Fatalf("required fields = %#v, want cursor/include_links optional", required)
		}
	}
}

func TestNewMCPServerReturnsServer(t *testing.T) {
	server := NewMCPServer(testConfig(), &fakeBackend{}, "test")
	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func connectMCPServer(t *testing.T, server *mcp.Server) (*mcp.ClientSession, func()) {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect returned error: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		serverSession.Close()
		t.Fatalf("client.Connect returned error: %v", err)
	}

	return clientSession, func() {
		clientSession.Close()
		serverSession.Close()
	}
}

func TestListProjectsFirstPageReturnsPageAndTotal(t *testing.T) {
	projects := make([]string, 120)
	for i := range projects {
		projects[i] = fmt.Sprintf("project-%03d", i)
	}
	cfg := testConfig()
	cfg.Projects = projects
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	if output.TotalProjects != 120 {
		t.Fatalf("TotalProjects = %d, want 120", output.TotalProjects)
	}
	if len(output.Projects) != 50 {
		t.Fatalf("projects count = %d, want 50 (first page)", len(output.Projects))
	}
	if output.NextCursor == nil {
		t.Fatal("NextCursor is nil, want non-nil (more pages remain)")
	}
	if output.Projects[0].Project != "project-000" {
		t.Fatalf("first project = %q, want project-000", output.Projects[0].Project)
	}
}

func TestListProjectsSecondPageViaCursorReturnsCorrectOffset(t *testing.T) {
	projects := make([]string, 120)
	for i := range projects {
		projects[i] = fmt.Sprintf("project-%03d", i)
	}
	cfg := testConfig()
	cfg.Projects = projects
	service := NewService(cfg, &fakeBackend{})

	firstPage, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects first page error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first page NextCursor is nil")
	}

	secondPage, err := service.ListProjects(context.Background(), ListProjectsInput{
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("ListProjects second page error: %v", err)
	}

	if len(secondPage.Projects) != 50 {
		t.Fatalf("second page count = %d, want 50", len(secondPage.Projects))
	}
	if secondPage.Projects[0].Project != "project-050" {
		t.Fatalf("second page first project = %q, want project-050", secondPage.Projects[0].Project)
	}
	if secondPage.NextCursor == nil {
		t.Fatal("second page NextCursor is nil, want non-nil (page 3 remains)")
	}
}

func TestListProjectsLastPageHasNoNextCursor(t *testing.T) {
	projects := make([]string, 60)
	for i := range projects {
		projects[i] = fmt.Sprintf("project-%03d", i)
	}
	cfg := testConfig()
	cfg.Projects = projects
	service := NewService(cfg, &fakeBackend{})

	firstPage, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("first ListProjects error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first page NextCursor is nil, expected more pages")
	}

	lastPage, err := service.ListProjects(context.Background(), ListProjectsInput{
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("last ListProjects error: %v", err)
	}

	if len(lastPage.Projects) != 10 {
		t.Fatalf("last page count = %d, want 10", len(lastPage.Projects))
	}
	if lastPage.NextCursor != nil {
		t.Fatal("last page NextCursor is non-nil, want nil (no more pages)")
	}
}

func TestListProjectsInvalidCursorReturnsError(t *testing.T) {
	service := NewService(testConfig(), &fakeBackend{})
	badCursor := "not-a-valid-cursor!!!"

	_, err := service.ListProjects(context.Background(), ListProjectsInput{
		Cursor: &badCursor,
	})
	if err == nil {
		t.Fatal("ListProjects error is nil, want INVALID_CURSOR")
	}
	if !IsCode(err, "INVALID_CURSOR") {
		t.Fatalf("ListProjects error = %v, want INVALID_CURSOR", err)
	}
}

func TestGetFileContextFullFileUnderCapReturnsAllContent(t *testing.T) {
	// 3 lines — well under 500-line cap
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

	if output.TotalLines != 3 {
		t.Fatalf("TotalLines = %d, want 3", output.TotalLines)
	}
	if output.Truncated {
		t.Fatal("Truncated = true, want false for file under cap")
	}
	if output.NextCursor != nil {
		t.Fatal("NextCursor is non-nil, want nil for file under cap")
	}
	if output.Hint != nil {
		t.Fatal("Hint is non-nil, want nil for file under cap")
	}
	if output.StartLine != 1 {
		t.Fatalf("StartLine = %d, want 1", output.StartLine)
	}
	if output.EndLine != 3 {
		t.Fatalf("EndLine = %d, want 3", output.EndLine)
	}
	if output.Content != "one\ntwo\nthree\n" {
		t.Fatalf("Content = %q, want full file with trailing newline", output.Content)
	}
}

func TestGetFileContextFullFileOverCapTruncatesAndReturnsCursor(t *testing.T) {
	// Build a 600-line file
	lineSlice := make([]string, 600)
	for i := range lineSlice {
		lineSlice[i] = fmt.Sprintf("line %d", i+1)
	}
	content := strings.Join(lineSlice, "\n") + "\n"

	backend := &fakeBackend{fileContent: content}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "src/Engine.swift",
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}

	if output.TotalLines != 600 {
		t.Fatalf("TotalLines = %d, want 600", output.TotalLines)
	}
	if !output.Truncated {
		t.Fatal("Truncated = false, want true for file over cap")
	}
	if output.StartLine != 1 {
		t.Fatalf("StartLine = %d, want 1", output.StartLine)
	}
	if output.EndLine != 500 {
		t.Fatalf("EndLine = %d, want 500", output.EndLine)
	}
	if output.NextCursor == nil {
		t.Fatal("NextCursor is nil, want cursor for truncated file")
	}
	if output.Hint == nil {
		t.Fatal("Hint is nil, want hint text for truncated file")
	}
	if !strings.Contains(*output.Hint, "600") {
		t.Fatalf("Hint = %q, want mention of total line count", *output.Hint)
	}
}

func TestGetFileContextNextPageViaCursorReturnsCorrectLines(t *testing.T) {
	lineSlice := make([]string, 600)
	for i := range lineSlice {
		lineSlice[i] = fmt.Sprintf("line %d", i+1)
	}
	content := strings.Join(lineSlice, "\n") + "\n"

	backend := &fakeBackend{fileContent: content}
	service := NewService(testConfig(), backend)

	firstPage, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "src/Engine.swift",
	})
	if err != nil {
		t.Fatalf("first GetFileContext error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first page NextCursor is nil")
	}

	secondPage, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "src/Engine.swift",
		Cursor:   firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("second GetFileContext error: %v", err)
	}

	if secondPage.StartLine != 501 {
		t.Fatalf("StartLine = %d, want 501", secondPage.StartLine)
	}
	if secondPage.EndLine != 600 {
		t.Fatalf("EndLine = %d, want 600", secondPage.EndLine)
	}
	if secondPage.Truncated {
		t.Fatal("Truncated = true, want false for last page")
	}
	if secondPage.NextCursor != nil {
		t.Fatal("NextCursor is non-nil, want nil for last page")
	}
}

func TestGetFileContextCursorFromDifferentFileReturnsError(t *testing.T) {
	backend := &fakeBackend{fileContent: "one\ntwo\n"}
	service := NewService(testConfig(), backend)

	// Get a valid cursor for a different file
	lineSlice := make([]string, 600)
	for i := range lineSlice {
		lineSlice[i] = fmt.Sprintf("line %d", i+1)
	}
	otherContent := strings.Join(lineSlice, "\n") + "\n"
	otherBackend := &fakeBackend{fileContent: otherContent}
	otherService := NewService(testConfig(), otherBackend)
	otherPage, err := otherService.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "src/Other.swift",
	})
	if err != nil || otherPage.NextCursor == nil {
		t.Fatal("could not get cursor from other file")
	}

	// Use that cursor with a different file
	_, err = service.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "src/Engine.swift",
		Cursor:   otherPage.NextCursor,
	})
	if err == nil {
		t.Fatal("GetFileContext error is nil, want INVALID_CURSOR")
	}
	if !IsCode(err, "INVALID_CURSOR") {
		t.Fatalf("GetFileContext error = %v, want INVALID_CURSOR", err)
	}
}

func TestSearchCodeWarningFiresAboveThreshold(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 501,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if output.Warning == nil {
		t.Fatal("Warning is nil, want non-nil for total_hits > 500")
	}
	if !strings.Contains(*output.Warning, "501") {
		t.Fatalf("Warning = %q, want mention of hit count", *output.Warning)
	}
}

func TestSearchCodeWarningSilentBelowThreshold(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 499,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if output.Warning != nil {
		t.Fatalf("Warning = %q, want nil for total_hits <= 500", *output.Warning)
	}
}

func TestSearchCodeWarningSilentAtExactThreshold(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 500,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if output.Warning != nil {
		t.Fatalf("Warning = %q, want nil for total_hits exactly at threshold", *output.Warning)
	}
}

func TestExpandResultContextsAttachesWindow(t *testing.T) {
	lines := []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8", "L9", "L10"}
	backend := &fakeBackend{
		fileContent: strings.Join(lines, "\n"),
	}
	cfg := testConfig()
	cfg.ContextBefore = 5
	cfg.ContextAfter = 10
	service := NewService(cfg, backend)

	results := []Result{
		{Project: "platform", FilePath: "src/Foo.java", LineNumber: 6},
	}
	got := service.expandResultContexts(context.Background(), results, cfg.BudgetTiers.Default)
	if got[0].Context == nil {
		t.Fatal("Context is nil, want non-nil")
	}
	if got[0].Context.StartLine != 1 {
		t.Fatalf("StartLine = %d, want 1", got[0].Context.StartLine)
	}
	if got[0].Context.EndLine != 10 {
		t.Fatalf("EndLine = %d, want 10", got[0].Context.EndLine)
	}
	want := strings.Join(lines, "\n")
	if got[0].Context.Content != want {
		t.Fatalf("Content = %q, want %q", got[0].Context.Content, want)
	}
}

func TestExpandResultContextsDeduplicatesFileByProject(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\nfour\nfive\n",
	}
	cfg := testConfig()
	cfg.ContextBefore = 1
	cfg.ContextAfter = 1
	cfg.BudgetTiers.Default.ContextBefore = 1
	cfg.BudgetTiers.Default.ContextAfter = 1
	service := NewService(cfg, backend)

	results := []Result{
		{Project: "platform", FilePath: "src/Foo.java", LineNumber: 2},
		{Project: "platform", FilePath: "src/Foo.java", LineNumber: 4},
	}
	got := service.expandResultContexts(context.Background(), results, cfg.BudgetTiers.Default)

	if backend.fileCallCount != 1 {
		t.Fatalf("FileContent called %d times, want 1", backend.fileCallCount)
	}
	if got[0].Context == nil || got[1].Context == nil {
		t.Fatal("expected both results to have Context set")
	}
	if got[0].Context.StartLine != 1 {
		t.Fatalf("result[0].Context.StartLine = %d, want 1", got[0].Context.StartLine)
	}
	if got[1].Context.StartLine != 3 {
		t.Fatalf("result[1].Context.StartLine = %d, want 3", got[1].Context.StartLine)
	}
}

func TestExpandResultContextsDifferentProjectsSamePath(t *testing.T) {
	backend := &fakeBackend{
		fileContents: map[string]string{
			"alpha:src/Foo.java": "alpha-one\nalpha-two\nalpha-three\n",
			"beta:src/Foo.java":  "beta-one\nbeta-two\nbeta-three\n",
		},
	}
	cfg := testConfig()
	cfg.ContextBefore = 0
	cfg.ContextAfter = 0
	cfg.BudgetTiers.Default.ContextBefore = 0
	cfg.BudgetTiers.Default.ContextAfter = 0
	cfg.BudgetTiers.Default.MaxExpandedResults = 10
	cfg.BudgetTiers.Default.MaxExpandedFiles = 5
	service := NewService(cfg, backend)

	results := []Result{
		{Project: "alpha", FilePath: "src/Foo.java", LineNumber: 2},
		{Project: "beta", FilePath: "src/Foo.java", LineNumber: 2},
	}
	got := service.expandResultContexts(context.Background(), results, cfg.BudgetTiers.Default)

	if backend.fileCallCount != 2 {
		t.Fatalf("FileContent called %d times, want 2", backend.fileCallCount)
	}
	if got[0].Context == nil || got[1].Context == nil {
		t.Fatal("expected both results to have Context set")
	}
	if got[0].Context.Content != "alpha-two" {
		t.Fatalf("result[0] content = %q, want alpha-two", got[0].Context.Content)
	}
	if got[1].Context.Content != "beta-two" {
		t.Fatalf("result[1] content = %q, want beta-two", got[1].Context.Content)
	}
}

func TestExpandResultContextsFetchErrorLeavesContextNil(t *testing.T) {
	backend := &fakeBackend{
		fileContents: map[string]string{
			"platform:src/Good.java": "one\ntwo\nthree\n",
		},
		fileErrors: map[string]error{
			"platform:src/Bad.java": errors.New("not found"),
		},
	}
	cfg := testConfig()
	cfg.ContextBefore = 1
	cfg.ContextAfter = 1
	service := NewService(cfg, backend)

	results := []Result{
		{Project: "platform", FilePath: "src/Bad.java", LineNumber: 1},
		{Project: "platform", FilePath: "src/Good.java", LineNumber: 2},
	}
	got := service.expandResultContexts(context.Background(), results, cfg.BudgetTiers.Default)

	if got[0].Context != nil {
		t.Fatalf("result[0].Context = %+v, want nil (fetch failed)", got[0].Context)
	}
	if got[1].Context == nil {
		t.Fatal("result[1].Context is nil, want non-nil")
	}
}

func TestExpandResultContextsWindowClampsAtFileBoundary(t *testing.T) {
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\n",
	}
	cfg := testConfig()
	cfg.ContextBefore = 100
	cfg.ContextAfter = 100
	service := NewService(cfg, backend)

	results := []Result{
		{Project: "platform", FilePath: "src/Foo.java", LineNumber: 2},
	}
	got := service.expandResultContexts(context.Background(), results, cfg.BudgetTiers.Default)

	if got[0].Context == nil {
		t.Fatal("Context is nil")
	}
	if got[0].Context.StartLine != 1 {
		t.Fatalf("StartLine = %d, want 1", got[0].Context.StartLine)
	}
	if got[0].Context.EndLine != 3 {
		t.Fatalf("EndLine = %d, want 3", got[0].Context.EndLine)
	}
}

func TestSearchCodeExpandContextDefaultOn(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 2, Snippet: strPtr("foo")},
			},
		},
		fileContent: "one\ntwo\nthree\n",
	}
	cfg := testConfig()
	cfg.AutoExpandContext = true
	cfg.ContextBefore = 1
	cfg.ContextAfter = 1
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "foo",
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if output.Results[0].Context == nil {
		t.Fatal("Context is nil, want non-nil when AutoExpandContext=true")
	}
}

func TestSearchCodeExpandContextFalseSkipsExpand(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 2, Snippet: strPtr("foo")},
			},
		},
		fileContent: "one\ntwo\nthree\n",
	}
	cfg := testConfig()
	cfg.AutoExpandContext = true
	service := NewService(cfg, backend)

	expandContext := false
	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:         "foo",
		ExpandContext: &expandContext,
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if output.Results[0].Context != nil {
		t.Fatal("Context is non-nil, want nil when ExpandContext=false")
	}
	backend.mu.Lock()
	callCount := backend.fileCallCount
	backend.mu.Unlock()
	if callCount != 0 {
		t.Fatalf("FileContent called %d times, want 0 when ExpandContext=false", callCount)
	}
}

func TestSearchCodeExpandContextTrueOverridesConfigFalse(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 2, Snippet: strPtr("foo")},
			},
		},
		fileContent: "one\ntwo\nthree\n",
	}
	cfg := testConfig()
	cfg.AutoExpandContext = false
	cfg.ContextBefore = 1
	cfg.ContextAfter = 1
	service := NewService(cfg, backend)

	expandContext := true
	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:         "foo",
		ExpandContext: &expandContext,
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if output.Results[0].Context == nil {
		t.Fatal("Context is nil, want non-nil when ExpandContext=true overrides config false")
	}
}

func TestSearchSymbolDefinitionsExpandContext(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Bar.java", LineNumber: 1, Snippet: strPtr("class Bar")},
			},
		},
		fileContent: "class Bar {\n}\n",
	}
	cfg := testConfig()
	cfg.AutoExpandContext = true
	cfg.ContextBefore = 0
	cfg.ContextAfter = 1
	service := NewService(cfg, backend)

	output, err := service.SearchSymbolDefinitions(context.Background(), SymbolSearchInput{
		Symbol: "Bar",
	})
	if err != nil {
		t.Fatalf("SearchSymbolDefinitions error: %v", err)
	}
	if output.Results[0].Context == nil {
		t.Fatal("Context is nil, want non-nil")
	}
}

func TestNewMCPServerRegistersListSymbolsWhenEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{ListSymbols: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "list_symbols") {
		t.Fatalf("tools = %#v, want list_symbols included", names)
	}
}

func TestNewMCPServerDoesNotRegisterListSymbolsWhenDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{ListSymbols: false}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	for _, tool := range tools.Tools {
		if tool.Name == "list_symbols" {
			t.Fatal("list_symbols tool registered, want absent when disabled")
		}
	}
}

func TestListSymbolsFiltersHitsByKind(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 3,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 10, Snippet: strPtr("class Foo {}"), Tag: "class"},
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 20, Snippet: strPtr("void doIt() {}"), Tag: "function"},
				{Project: "platform", FilePath: "src/Bar.java", LineNumber: 5, Snippet: strPtr("class Bar {}"), Tag: "class"},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListSymbols(context.Background(), ListSymbolsInput{Kind: "class"})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}

	if output.TotalHits != 3 {
		t.Fatalf("TotalHits = %d, want 3 (raw count before kind filter)", output.TotalHits)
	}
	if len(output.Symbols) != 2 {
		t.Fatalf("symbols length = %d, want 2 (only class hits)", len(output.Symbols))
	}
	if output.Symbols[0].Kind != "class" {
		t.Fatalf("Symbols[0].Kind = %q, want class", output.Symbols[0].Kind)
	}
	if output.Symbols[1].Kind != "class" {
		t.Fatalf("Symbols[1].Kind = %q, want class", output.Symbols[1].Kind)
	}
}

func TestListSymbolsNoKindFilterReturnsAllHits(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 3,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 10, Snippet: strPtr("class Foo {}"), Tag: "class"},
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 20, Snippet: strPtr("void doIt() {}"), Tag: "function"},
				{Project: "platform", FilePath: "src/Bar.java", LineNumber: 5, Snippet: strPtr("class Bar {}"), Tag: "class"},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListSymbols(context.Background(), ListSymbolsInput{})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}

	if len(output.Symbols) != 3 {
		t.Fatalf("symbols length = %d, want 3 (no kind filter)", len(output.Symbols))
	}
}

func TestListSymbolsWarningIncludesCallEstimate(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 200,
			Hits:      []opengrok.Hit{},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListSymbols(context.Background(), ListSymbolsInput{})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}

	if output.Warning == nil {
		t.Fatal("Warning is nil, want non-nil for total_hits > 100")
	}
	if !strings.Contains(*output.Warning, "200") {
		t.Fatalf("Warning = %q, want mention of hit count", *output.Warning)
	}
	// At page_size 20, remaining pages = (200-1)/20 = 9
	if !strings.Contains(*output.Warning, "9") {
		t.Fatalf("Warning = %q, want mention of remaining call estimate", *output.Warning)
	}
}

func TestListSymbolsIncludeSnippetsFalseNullsSnippet(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 10, Snippet: strPtr("class Foo {}"), Tag: "class"},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	noSnippets := false
	output, err := service.ListSymbols(context.Background(), ListSymbolsInput{IncludeSnippets: &noSnippets})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}
	if len(output.Symbols) != 1 {
		t.Fatalf("symbols length = %d, want 1", len(output.Symbols))
	}
	if output.Symbols[0].Snippet != nil {
		t.Fatalf("Snippet = %q, want nil when include_snippets=false", *output.Symbols[0].Snippet)
	}

	yesSnippets := true
	output2, err := service.ListSymbols(context.Background(), ListSymbolsInput{IncludeSnippets: &yesSnippets})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}
	if output2.Symbols[0].Snippet == nil {
		t.Fatal("Snippet is nil, want non-nil when include_snippets=true")
	}
	if *output2.Symbols[0].Snippet != "class Foo {}" {
		t.Fatalf("Snippet = %q, want %q", *output2.Symbols[0].Snippet, "class Foo {}")
	}
}

func TestListSymbolsKindFilterWarnsAboutPrefilterTotal(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 200,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 10, Snippet: strPtr("class Foo {}"), Tag: "class"},
				{Project: "platform", FilePath: "src/Foo.java", LineNumber: 20, Snippet: strPtr("void doIt() {}"), Tag: "function"},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListSymbols(context.Background(), ListSymbolsInput{Kind: "class"})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}
	if output.Warning == nil {
		t.Fatal("Warning is nil, want kind-prefilter warning")
	}
	if !strings.Contains(*output.Warning, "ctags kind") {
		t.Fatalf("Warning = %q, want it to mention the ctags-kind prefilter limitation", *output.Warning)
	}
	if !output.HasMore {
		t.Fatal("HasMore = false, want true (200 hits, page size 20)")
	}
	if output.TotalPages != 10 {
		t.Fatalf("TotalPages = %d, want 10", output.TotalPages)
	}
}

func TestSearchCodeResultKindComesFromHitTag(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 2,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 1, Snippet: strPtr("class Engine {}"), Tag: "class"},
				{Project: "platform", FilePath: "src/run.swift", LineNumber: 5, Snippet: strPtr("func run() {}"), Tag: ""},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}

	if len(output.Results) != 2 {
		t.Fatalf("results length = %d, want 2", len(output.Results))
	}
	if output.Results[0].Kind != "class" {
		t.Fatalf("Results[0].Kind = %q, want %q", output.Results[0].Kind, "class")
	}
	if output.Results[1].Kind != "" {
		t.Fatalf("Results[1].Kind = %q, want empty string for hit with no tag", output.Results[1].Kind)
	}
}

func TestListFilesReportsBackendTruncation(t *testing.T) {
	backend := &fakeBackend{
		fileEntries:       []opengrok.FileEntry{{Path: "src/main.go"}},
		fileListTruncated: true,
	}
	service := NewService(testConfig(), backend)

	output, err := service.ListFiles(context.Background(), ListFilesInput{})
	if err != nil {
		t.Fatalf("ListFiles error = %v", err)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal ListFiles output: %v", err)
	}
	if !strings.Contains(string(data), `"truncated":true`) {
		t.Fatalf("ListFiles output = %s, want truncated=true", data)
	}
	if output.Warning == nil {
		t.Fatal("ListFiles warning is nil, want truncation warning")
	}
}

func TestListFilesPopulatesPagination(t *testing.T) {
	backend := &fakeBackend{
		fileEntries: []opengrok.FileEntry{
			{Path: "src/a.go"},
			{Path: "src/b.go"},
			{Path: "src/c.go"},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListFiles(context.Background(), ListFilesInput{PageSize: 2})
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if output.TotalHits != 3 {
		t.Fatalf("TotalHits = %d, want 3", output.TotalHits)
	}
	if output.TotalPages != 2 {
		t.Fatalf("TotalPages = %d, want 2 (3 files / page size 2)", output.TotalPages)
	}
	if !output.HasMore {
		t.Fatal("HasMore = false, want true")
	}
}

func TestProjectOverviewReportsBackendTruncation(t *testing.T) {
	backend := &fakeBackend{
		fileEntries:       []opengrok.FileEntry{{Path: "src/main.go"}},
		fileListTruncated: true,
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetProjectOverview(context.Background(), ProjectOverviewInput{})
	if err != nil {
		t.Fatalf("GetProjectOverview error = %v", err)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal project overview output: %v", err)
	}
	if !strings.Contains(string(data), `"truncated":true`) {
		t.Fatalf("ProjectOverview output = %s, want truncated=true", data)
	}
	if output.Warning == nil {
		t.Fatal("ProjectOverview warning is nil, want truncation warning")
	}
}

func TestExpandResultContextsRecoversBackendPanic(t *testing.T) {
	service := NewService(testConfig(), &fakeBackend{panicFileRead: true})
	results := []Result{{Project: "platform", FilePath: "src/main.go", LineNumber: 1}}
	done := make(chan *ExpansionDiagnostics, 1)

	go func() {
		_, diagnostics := service.expandResultContextsWithDiagnostics(
			context.Background(),
			results,
			testConfig().BudgetTiers.Default,
		)
		done <- diagnostics
	}()

	select {
	case diagnostics := <-done:
		if diagnostics.ExpandedResults != 0 {
			t.Fatalf("ExpandedResults = %d, want 0 after recovered panic", diagnostics.ExpandedResults)
		}
	case <-time.After(time.Second):
		t.Fatal("context expansion hung after backend panic")
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

func TestFullSearchCoercesStringEncodedBooleans(t *testing.T) {
	// Some MCP clients serialize scalar arguments as JSON strings, sending
	// include_links:"true" instead of include_links:true. The boolean fields
	// are *bool (schema ["null","boolean"]), so the SDK validator rejects the
	// string before the handler runs. The server coerces string-encoded
	// booleans for boolean-typed fields so these calls succeed.
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"links true / snippets false", map[string]any{"query": "Engine", "path_prefix": "", "file_type": "", "page_size": 10, "include_links": "true", "include_snippets": "false"}},
		{"links false / snippets true", map[string]any{"query": "Engine", "path_prefix": "", "file_type": "", "page_size": 10, "include_links": "false", "include_snippets": "true"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "search_code",
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("CallTool returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("CallTool result is an error: %+v", result.Content)
			}
		})
	}
}

func TestFullSearchCoercesStringEncodedNumbers(t *testing.T) {
	// The same clients that stringify booleans also send numeric arguments as
	// JSON strings, e.g. page_size:"10". page_size/max_hits_per_file are int
	// (schema "integer"), so the validator rejects a string before the handler
	// runs. The server coerces string-encoded numbers for numeric-typed fields.
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	server := NewMCPServer(cfg, backend, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "search_code",
		Arguments: map[string]any{
			"query":             "Engine",
			"path_prefix":       "",
			"file_type":         "",
			"page_size":         "10", // string-encoded integer from a flaky client
			"max_hits_per_file": "5",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool result is an error: %+v", result.Content)
	}
}

func TestSearchCodePopulatesPagination(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 45,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.java", LineNumber: 3, Snippet: strPtr("class Engine {}")},
			},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}
	if output.Page != 1 {
		t.Fatalf("Page = %d, want 1", output.Page)
	}
	if output.TotalPages != 3 {
		t.Fatalf("TotalPages = %d, want 3 (45 hits / page size 20)", output.TotalPages)
	}
	if !output.HasMore {
		t.Fatal("HasMore = false, want true")
	}
	if output.PageSize != 20 {
		t.Fatalf("PageSize = %d, want 20", output.PageSize)
	}
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	cfg.DefaultProject = "platform"
	return cfg
}

func strPtr(s string) *string { return &s }

func TestSearchCodeInputSchemaRequiredFields(t *testing.T) {
	schema, err := jsonschema.For[SearchCodeInput](nil)
	if err != nil {
		t.Fatalf("infer SearchCodeInput schema: %v", err)
	}
	if !slices.Contains(schema.Required, "query") {
		t.Errorf("query should be required, required=%v", schema.Required)
	}
	for _, field := range []string{"file_type", "path_prefix", "page_size"} {
		if slices.Contains(schema.Required, field) {
			t.Errorf("%s should NOT be required, required=%v", field, schema.Required)
		}
	}
	if _, ok := schema.Properties["tokenized"]; !ok {
		t.Errorf("tokenized property missing from schema")
	}
	if _, ok := schema.Properties["path_exclude"]; !ok {
		t.Errorf("path_exclude property missing from schema")
	}
}

func TestSearchAndReadInputSchemaRequiredFields(t *testing.T) {
	schema, err := jsonschema.For[SearchAndReadInput](nil)
	if err != nil {
		t.Fatalf("infer SearchAndReadInput schema: %v", err)
	}
	if !slices.Contains(schema.Required, "query") {
		t.Errorf("query should be required, required=%v", schema.Required)
	}
	for _, field := range []string{"file_type", "path_prefix", "page_size"} {
		if slices.Contains(schema.Required, field) {
			t.Errorf("%s should NOT be required, required=%v", field, schema.Required)
		}
	}
	if _, ok := schema.Properties["tokenized"]; !ok {
		t.Errorf("tokenized property missing from schema")
	}
	if _, ok := schema.Properties["path_exclude"]; !ok {
		t.Errorf("path_exclude property missing from schema")
	}
}

func TestSymbolSearchInputSchemaRequiredFields(t *testing.T) {
	schema, err := jsonschema.For[SymbolSearchInput](nil)
	if err != nil {
		t.Fatalf("infer SymbolSearchInput schema: %v", err)
	}
	if !slices.Contains(schema.Required, "symbol") {
		t.Errorf("symbol should be required, required=%v", schema.Required)
	}
	if slices.Contains(schema.Required, "page_size") {
		t.Errorf("page_size should NOT be required, required=%v", schema.Required)
	}
	if prop, ok := schema.Properties["symbol"]; !ok || prop.Description == "" {
		t.Errorf("symbol property should be present and documented; got ok=%v", ok)
	}
}

func TestFileContextInputDocumentsWindowParams(t *testing.T) {
	schema, err := jsonschema.For[FileContextInput](nil)
	if err != nil {
		t.Fatalf("infer FileContextInput schema: %v", err)
	}
	if slices.Contains(schema.Required, "line_number") {
		t.Errorf("line_number should NOT be required, required=%v", schema.Required)
	}
	for _, field := range []string{"line_number", "before", "after"} {
		if prop, ok := schema.Properties[field]; !ok || prop.Description == "" {
			t.Errorf("%s should be present and documented; got ok=%v", field, ok)
		}
	}
}

func TestSearchCodeAutoQuotesMultiWordQuery(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	out, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "extends PaymentProcessor",
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if got := backend.searchRequests[0].Query; got != `"extends PaymentProcessor"` {
		t.Fatalf("backend query = %q, want auto-quoted phrase", got)
	}
	if out.Query != `"extends PaymentProcessor"` {
		t.Fatalf("output query = %q, want auto-quoted phrase", out.Query)
	}
}

func TestSearchCodeTokenizedOptOutKeepsBagOfWords(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	tokenized := true
	_, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:     "extends PaymentProcessor",
		Tokenized: &tokenized,
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if got := backend.searchRequests[0].Query; got != "extends PaymentProcessor" {
		t.Fatalf("backend query = %q, want unquoted", got)
	}
}

func TestSearchCodeAppendsPathExcludeAfterNormalization(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	_, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:       "extends PaymentProcessor",
		PathExclude: "legacy",
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if got := backend.searchRequests[0].Query; got != `"extends PaymentProcessor" -path:legacy` {
		t.Fatalf("backend query = %q, want phrase + path exclusion", got)
	}
}

func TestSearchCodeAppendsMultiplePathExcludes(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	_, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:       "extends PaymentProcessor",
		PathExclude: "service test",
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if got := backend.searchRequests[0].Query; got != `"extends PaymentProcessor" -path:service -path:test` {
		t.Fatalf("backend query = %q, want two -path: exclusions", got)
	}
}

func TestSearchSymbolDefinitionsDoesNotAutoQuote(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	_, err := service.SearchSymbolDefinitions(context.Background(), SymbolSearchInput{
		Symbol: "Payment Processor",
	})
	if err != nil {
		t.Fatalf("SearchSymbolDefinitions error: %v", err)
	}
	if got := backend.searchRequests[0].Query; got != "Payment Processor" {
		t.Fatalf("symbol query = %q, want unmodified (no auto-quote on symbol path)", got)
	}
}

func TestSearchCodeAutoQuoteWarning(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	out, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "extends PaymentProcessor"})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if out.Warning == nil || !strings.Contains(*out.Warning, "tokenized:true") {
		t.Fatalf("warning = %v, want auto-quote note mentioning tokenized:true", out.Warning)
	}
}

func TestSearchCodeDateWarningInFullText(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	out, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "date:[20230101 TO 20261231]",
		Mode:  string(opengrok.ModeFullText),
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if out.Warning == nil || !strings.Contains(*out.Warning, "history mode") {
		t.Fatalf("warning = %v, want date:-in-full_text note", out.Warning)
	}
}

func TestSearchCodeNoDateWarningInHistory(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	out, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "date:[20230101 TO 20261231]",
		Mode:  string(opengrok.ModeHistory),
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if out.Warning != nil && strings.Contains(*out.Warning, "history mode") {
		t.Fatalf("warning = %q, want no date: note in history mode", *out.Warning)
	}
}

func TestSearchCodeDateWarningIgnoresPathExclude(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	out, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:       "Engine",
		Mode:        string(opengrok.ModeFullText),
		PathExclude: "date:weird",
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if out.Warning != nil && strings.Contains(*out.Warning, "history mode") {
		t.Fatalf("warning = %q, want no date: note from path_exclude value", *out.Warning)
	}
}

func TestSearchCodeNoDateWarningForCandidateField(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	out, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query: "candidate:Foo",
		Mode:  string(opengrok.ModeFullText),
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if out.Warning != nil && strings.Contains(*out.Warning, "history mode") {
		t.Fatalf("warning = %q, want no date: note for candidate: field", *out.Warning)
	}
}

func TestSearchCodeLargeResultBackstopQuotesUserQuery(t *testing.T) {
	backend := &fakeBackend{searchResult: opengrok.SearchResult{TotalHits: 1200, Hits: []opengrok.Hit{}}}
	service := NewService(testConfig(), backend)

	tokenized := true
	out, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:       "extends PaymentProcessor",
		Tokenized:   &tokenized,
		PathExclude: "legacy",
	})
	if err != nil {
		t.Fatalf("SearchCode error: %v", err)
	}
	if out.Warning == nil || !strings.Contains(*out.Warning, `"extends PaymentProcessor"`) {
		t.Fatalf("warning = %v, want suggestion quoting the user query (not the path-excluded query)", out.Warning)
	}
	if strings.Contains(*out.Warning, "-path:legacy") {
		t.Fatalf("warning = %q, should not echo the -path: term", *out.Warning)
	}
}

func TestSearchCodeBareQueryCursorRoundTrips(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 100,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 1, Snippet: strPtr("x")},
			},
		},
	}
	service := NewService(testConfig(), backend)

	first, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "extends PaymentProcessor"})
	if err != nil {
		t.Fatalf("first SearchCode error: %v", err)
	}
	if first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatal("expected a next cursor on first page")
	}

	// Second request: SAME bare query (re-normalized deterministically) + cursor.
	second, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:  "extends PaymentProcessor",
		Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatalf("second SearchCode error: %v (cursor should validate after deterministic normalization)", err)
	}
	if second.Page <= first.Page {
		t.Fatalf("second page %d should advance past first page %d", second.Page, first.Page)
	}
}

func TestSearchToolDescriptionsMentionQuoting(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceFull
	cfg.Capabilities = config.Capabilities{SearchCode: true, GetFileContext: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	descs := map[string]string{}
	for _, tool := range tools.Tools {
		descs[tool.Name] = tool.Description
	}
	for _, name := range []string{"search_code", "search_and_read"} {
		d, ok := descs[name]
		if !ok {
			t.Fatalf("tool %q not registered", name)
		}
		if !strings.Contains(d, "quote") {
			t.Errorf("tool %q description should mention quoting; got: %s", name, d)
		}
		if !strings.Contains(d, "tokenized") {
			t.Errorf("tool %q description should mention tokenized opt-out", name)
		}
	}
}
