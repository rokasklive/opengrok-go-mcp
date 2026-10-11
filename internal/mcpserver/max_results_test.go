// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestSearchAndReadMaxResultsKeepsSurplusReachable(t *testing.T) {
	hits := make([]opengrok.Hit, 5)
	for i := range hits {
		hits[i] = opengrok.Hit{Project: "platform", FilePath: "src/Engine.swift", LineNumber: i + 1, Snippet: strPtr("x")}
	}
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{TotalHits: 1, Hits: hits},
		fileContent:  "one\ntwo\nthree\nfour\nfive\n",
	}
	service := NewService(testConfig(), backend)

	var lines []int
	var next *string
	for page := 0; page < 10; page++ {
		out, err := service.SearchAndRead(context.Background(), SearchAndReadInput{
			Query:      "Engine",
			PageSize:   5,
			MaxResults: 2,
			Cursor:     next,
		})
		if err != nil {
			t.Fatalf("page %d: SearchAndRead error: %v", page+1, err)
		}
		if len(out.Results) > 2 {
			t.Fatalf("page %d: %d results, want at most max_results=2", page+1, len(out.Results))
		}
		if page == 0 && (out.Warning == nil || !strings.Contains(*out.Warning, "max_results")) {
			t.Fatalf("page 1 warning = %v, want truncation notice naming max_results", out.Warning)
		}
		for _, result := range out.Results {
			lines = append(lines, result.LineNumber)
		}
		if out.NextCursor == nil {
			break
		}
		next = out.NextCursor
	}
	if !slices.Equal(lines, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("lines across pages = %v, want every line exactly once: [1 2 3 4 5]", lines)
	}
}
