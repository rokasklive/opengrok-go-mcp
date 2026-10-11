// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestSearchAndReadFetchesEachFileOnce(t *testing.T) {
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 2,
			Hits: []opengrok.Hit{
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 1, Snippet: strPtr("a")},
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 2, Snippet: strPtr("b")},
				{Project: "platform", FilePath: "src/Engine.swift", LineNumber: 3, Snippet: strPtr("c")},
				{Project: "platform", FilePath: "src/Other.swift", LineNumber: 1, Snippet: strPtr("d")},
			},
		},
		fileContent: "a\nb\nc\n",
	}
	service := NewService(testConfig(), backend)

	out, err := service.SearchAndRead(context.Background(), SearchAndReadInput{Query: "Engine", PageSize: 10})
	if err != nil {
		t.Fatalf("SearchAndRead error: %v", err)
	}
	if len(out.Results) != 4 {
		t.Fatalf("results = %d, want 4", len(out.Results))
	}
	if backend.fileCallCount != 2 {
		t.Fatalf("file fetches = %d, want 2 (one per distinct file)", backend.fileCallCount)
	}
}
