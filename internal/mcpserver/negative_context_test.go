// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestSearchAndReadNegativeContextDoesNotPanic(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 2,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 5, Snippet: strPtr("five")},
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 1, Snippet: strPtr("one")},
			},
		},
		fileContent: "one\ntwo\nthree\nfour\nfive\nsix\n",
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	out, err := service.SearchAndRead(context.Background(), SearchAndReadInput{
		Query:       "Engine",
		LinesBefore: -50,
		LinesAfter:  -10,
	})
	if err != nil {
		t.Fatalf("SearchAndRead error: %v", err)
	}
	if len(out.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(out.Results))
	}
	for _, result := range out.Results {
		if result.StartLine != result.LineNumber || result.EndLine != result.LineNumber {
			t.Fatalf("window = %d-%d for line %d, want only the matched line", result.StartLine, result.EndLine, result.LineNumber)
		}
	}
}

func TestExtractWindowClampsNegativeSpans(t *testing.T) {
	got := extractWindow("one\ntwo\nthree\n", 1, -5, -5)
	if got.Content != "one" || got.StartLine != 1 || got.EndLine != 1 {
		t.Fatalf("extractWindow = %+v, want only line 1", got)
	}
}
