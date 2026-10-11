// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"errors"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestSearchAndReadResultsOnPageExcludesFailedReads(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 2,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 1, Snippet: strPtr("a")},
				{Project: "platform", FilePath: "src/Gone.swift", LineNumber: 1, Snippet: strPtr("b")},
			},
		},
		fileContent: "a\n",
		fileErrors:  map[string]error{"platform:src/Gone.swift": errors.New("not found")},
	}
	service := NewService(testConfig(), backend)

	out, err := service.SearchAndRead(context.Background(), SearchAndReadInput{Query: "Engine"})
	if err != nil {
		t.Fatalf("SearchAndRead error: %v", err)
	}
	if out.ResultsOnPage != len(out.Results) || len(out.Results) != 1 {
		t.Fatalf("results_on_page = %d, len(results) = %d, want both 1", out.ResultsOnPage, len(out.Results))
	}
	if !hasWarning(out.WarningFields, warnFileReadFailed) {
		t.Fatalf("warnings = %+v, want %s", out.Warnings, warnFileReadFailed)
	}
}

func TestSearchCrossProjectReferencesSetsResultsOnPage(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 3,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/A.swift", LineNumber: 1},
				{Project: "platform", FilePath: "src/B.swift", LineNumber: 2},
				{Project: "apps", FilePath: "src/C.swift", LineNumber: 3},
			},
		},
	}
	cfg := testConfig()
	cfg.Projects = []string{"platform", "apps"}
	service := NewService(cfg, backend)

	out, err := service.SearchCrossProjectReferences(context.Background(), CrossProjectReferencesInput{
		Symbol:   "Engine",
		Projects: []string{"platform", "apps"},
	})
	if err != nil {
		t.Fatalf("SearchCrossProjectReferences error: %v", err)
	}
	if out.ResultsOnPage != 3 {
		t.Fatalf("results_on_page = %d, want 3", out.ResultsOnPage)
	}
}

func TestFindSymbolAndReferencesReportsPaginationCounts(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 18,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 42, Snippet: strPtr("final class Engine {}")},
			},
		},
		fileContent: "final class Engine {}\n",
	}
	service := NewService(testConfig(), backend)

	out, err := service.FindSymbolAndReferences(context.Background(), FindSymbolAndReferencesInput{Symbol: "Engine"})
	if err != nil {
		t.Fatalf("FindSymbolAndReferences error: %v", err)
	}
	if out.TotalHits != 18 || out.TotalRefs != 18 {
		t.Fatalf("total_hits = %d, total_references = %d, want both 18", out.TotalHits, out.TotalRefs)
	}
	if out.ResultsOnPage != len(out.References) || out.ResultsOnPage == 0 {
		t.Fatalf("results_on_page = %d, len(references) = %d, want equal and non-zero", out.ResultsOnPage, len(out.References))
	}
}
