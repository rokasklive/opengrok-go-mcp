// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestCompactResultsOmitsRedundantFieldsPreservesCitation(t *testing.T) {
	raw := "https://grok.example.com/raw/platform/src/a.go"
	results := []Result{{
		DisplayTitle: "a.go:1",
		DisplayURL:   "https://grok.example.com/source/xref/platform/src/a.go#1",
		RawURL:       &raw,
		Metadata:     map[string]any{"unused": true},
		Citation: Citation{
			Title: "a.go:1",
			URL:   "https://grok.example.com/source/xref/platform/src/a.go#1",
			Line:  1,
		},
		Context: &ResultContext{Content: "package main", StartLine: 1, EndLine: 1},
	}}

	got := compactResults(results)
	if got[0].DisplayTitle != "" || got[0].DisplayURL != "" || got[0].RawURL != nil {
		t.Fatalf("redundant URL/title fields not cleared: %+v", got[0])
	}
	if got[0].Metadata != nil {
		t.Fatalf("metadata = %#v, want nil", got[0].Metadata)
	}
	if got[0].Citation.URL == "" {
		t.Fatal("citation.url cleared, want preserved")
	}
	if got[0].Context == nil || got[0].Context.Content != "package main" {
		t.Fatalf("context = %+v, want preserved when already present", got[0].Context)
	}
}

func TestSearchCodeCompactResponseModePreservesCitationOmitsDisplayURL(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits: []opengrok.Hit{{
				FilePath:   "src/Engine.swift",
				LineNumber: 42,
				Tag:        "class",
			}},
		},
	}
	service := NewService(testConfig(), backend)

	output, err := service.SearchCode(context.Background(), SearchCodeInput{
		Query:        "Engine",
		ResponseMode: "compact",
	})
	if err != nil {
		t.Fatalf("SearchCode returned error: %v", err)
	}
	if len(output.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(output.Results))
	}
	r := output.Results[0]
	if r.DisplayURL != "" {
		t.Fatalf("display_url = %q, want empty in compact response_mode", r.DisplayURL)
	}
	if r.Citation.URL == "" {
		t.Fatal("citation.url empty, want preserved in compact response_mode")
	}
	if output.Expansion != nil {
		t.Fatalf("expansion = %+v, want nil when response_mode is compact", output.Expansion)
	}
}
