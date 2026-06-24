// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

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

func TestSearchCodeTruncatesOverDeliveredHits(t *testing.T) {
	hits := make([]opengrok.Hit, 12)
	for i := range hits {
		hits[i] = opengrok.Hit{Project: "platform", FilePath: "src/Foo.java", LineNumber: i + 1}
	}
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{TotalHits: 28, Hits: hits},
	}
	service := NewService(testConfig(), backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:    "foo",
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}
	if len(output.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(output.Results))
	}
	if output.PageSize != 1 {
		t.Fatalf("page_size = %d, want 1", output.PageSize)
	}
	if output.Warning == nil || !strings.Contains(*output.Warning, "truncated") {
		t.Fatalf("warning = %v, want truncation notice", output.Warning)
	}
	if len(output.Warnings) == 0 || output.Warnings[0].Code != warnPageSizeTruncated {
		t.Fatalf("warnings = %+v, want PAGE_SIZE_TRUNCATED code", output.Warnings)
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
