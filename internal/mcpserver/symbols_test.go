// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

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

func TestListSymbolsKindFilterMetadataPresentWhenKindSet(t *testing.T) {
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
	if !output.KindFilterActive {
		t.Fatal("KindFilterActive = false, want true when kind is set")
	}
	if output.KindMatchesOnPage != len(output.Symbols) {
		t.Fatalf("KindMatchesOnPage = %d, want %d", output.KindMatchesOnPage, len(output.Symbols))
	}
	if output.TotalHitsScope != "pre_kind_filter" {
		t.Fatalf("TotalHitsScope = %q, want pre_kind_filter", output.TotalHitsScope)
	}
}

func TestListSymbolsKindFilterMetadataAbsentWhenKindOmitted(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits:      []opengrok.Hit{{Project: "platform", FilePath: "src/Foo.java", LineNumber: 10, Tag: "class"}},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListSymbols(context.Background(), ListSymbolsInput{})
	if err != nil {
		t.Fatalf("ListSymbols returned error: %v", err)
	}
	if output.KindFilterActive {
		t.Fatal("KindFilterActive should be false when kind omitted")
	}
	if output.KindMatchesOnPage != 0 {
		t.Fatalf("KindMatchesOnPage = %d, want 0 when absent", output.KindMatchesOnPage)
	}
	if output.TotalHitsScope != "" {
		t.Fatalf("TotalHitsScope = %q, want empty when kind omitted", output.TotalHitsScope)
	}
}
