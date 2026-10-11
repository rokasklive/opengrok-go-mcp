// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestSearchCodeMaxHitsPerFileAppliesAcrossTheWindow(t *testing.T) {
	var hits []opengrok.Hit
	for line := 1; line <= 6; line++ {
		hits = append(hits, opengrok.Hit{Project: "platform", FilePath: "src/A.java", LineNumber: line})
	}
	hits = append(hits,
		opengrok.Hit{Project: "platform", FilePath: "src/B.java", LineNumber: 1},
		opengrok.Hit{Project: "platform", FilePath: "src/C.java", LineNumber: 1},
	)
	backend := &fakeBackend{searchResult: opengrok.SearchResult{TotalHits: 3, Hits: hits}}
	service := NewService(testConfig(), backend)

	first, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "foo", PageSize: 1, MaxHitsPerFile: 1})
	if err != nil {
		t.Fatalf("page 1 SearchCode error: %v", err)
	}
	// Only B and C remain after the cap; the five extra A lines must not be counted.
	if first.Warning == nil || !strings.Contains(*first.Warning, "hold 2 more matching lines") {
		t.Fatalf("page 1 warning = %v, want it to count the 2 capped-window lines left", first.Warning)
	}
	files := []string{}
	for _, result := range first.Results {
		files = append(files, result.FilePath)
	}

	// Later pages omit max_hits_per_file: the cursor carries the cap.
	next := first.NextCursor
	for page := 2; page <= 3; page++ {
		if next == nil {
			t.Fatalf("page %d: NextCursor is nil, want more capped results", page)
		}
		out, err := service.SearchCode(context.Background(), SearchCodeInput{Query: "foo", Cursor: next})
		if err != nil {
			t.Fatalf("page %d SearchCode error: %v", page, err)
		}
		for _, result := range out.Results {
			files = append(files, result.FilePath)
		}
		next = out.NextCursor
	}
	if !slices.Equal(files, []string{"src/A.java", "src/B.java", "src/C.java"}) {
		t.Fatalf("files across pages = %v, want each file once: [src/A.java src/B.java src/C.java]", files)
	}
}
