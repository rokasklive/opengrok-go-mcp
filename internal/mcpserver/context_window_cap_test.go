// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestWindowBoundsCapsWindows(t *testing.T) {
	tests := []struct {
		name                         string
		total, anchor, before, after int
		wantStart, wantEnd           int
		wantCapped                   bool
	}{
		{"both sides huge", 2000, 1000, 1_000_000, 1_000_000, 751, 1250, true},
		{"small before keeps its span", 2000, 1000, 10, 1_000_000, 990, 1489, true},
		{"small after keeps its span", 2000, 1000, 1_000_000, 10, 511, 1010, true},
		{"short file is not capped", 100, 50, 1_000_000, 1_000_000, 1, 100, false},
		{"anchor past EOF keeps nearest lines", 2000, 5000, 1_000_000, 0, 1501, 2000, true},
		{"anchor past EOF small span", 2, 99, 1, 1, 2, 2, false},
		{"negative spans mean zero", 3, 1, -5, -5, 1, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, capped := windowBounds(tt.total, tt.anchor, tt.before, tt.after)
			if start != tt.wantStart || end != tt.wantEnd || capped != tt.wantCapped {
				t.Fatalf("windowBounds = (%d, %d, %v), want (%d, %d, %v)", start, end, capped, tt.wantStart, tt.wantEnd, tt.wantCapped)
			}
			if end-start+1 > maxContextWindowLines {
				t.Fatalf("window = %d lines, want at most %d", end-start+1, maxContextWindowLines)
			}
		})
	}
}

func TestGetFileContextCapsLargeWindow(t *testing.T) {
	lines := make([]string, 2000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	backend := &fakeBackend{fileContent: strings.Join(lines, "\n") + "\n"}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:    "platform",
		FilePath:   "src/Big.java",
		LineNumber: 1000,
		Before:     30,
		After:      1_000_000,
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}
	if got := output.EndLine - output.StartLine + 1; got != maxContextWindowLines {
		t.Fatalf("window = %d lines (%d-%d), want %d", got, output.StartLine, output.EndLine, maxContextWindowLines)
	}
	if output.StartLine != 970 {
		t.Fatalf("StartLine = %d, want 970 (the smaller before-span is kept in full)", output.StartLine)
	}
	if !output.Truncated || output.Hint == nil {
		t.Fatalf("Truncated = %v, Hint = %v, want a truncated window with a hint", output.Truncated, output.Hint)
	}
}

func TestGetFileContextWholeShortFileIsNotTruncated(t *testing.T) {
	backend := &fakeBackend{fileContent: "one\ntwo\nthree\n"}
	service := NewService(testConfig(), backend)

	output, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:    "platform",
		FilePath:   "src/Small.java",
		LineNumber: 2,
		After:      1_000_000,
	})
	if err != nil {
		t.Fatalf("GetFileContext returned error: %v", err)
	}
	if output.StartLine != 1 || output.EndLine != 3 || output.Truncated || output.Hint != nil {
		t.Fatalf("window = %d-%d truncated=%v hint=%v, want 1-3 untruncated", output.StartLine, output.EndLine, output.Truncated, output.Hint)
	}
}

func TestSearchAndReadWarnsWhenContextWindowCapped(t *testing.T) {
	lines := make([]string, 2000)
	for i := range lines {
		lines[i] = "line"
	}
	backend := &fakeBackend{
		searchResult: opengrok.SearchResult{
			TotalHits: 1,
			Hits:      []opengrok.Hit{{Project: "platform", FilePath: "src/Big.java", LineNumber: 1000, Snippet: strPtr("line")}},
		},
		fileContent: strings.Join(lines, "\n") + "\n",
	}
	service := NewService(testConfig(), backend)

	out, err := service.SearchAndRead(context.Background(), SearchAndReadInput{Query: "line", LinesAfter: 1_000_000})
	if err != nil {
		t.Fatalf("SearchAndRead error: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(out.Results))
	}
	if got := out.Results[0].EndLine - out.Results[0].StartLine + 1; got != maxContextWindowLines {
		t.Fatalf("window = %d lines, want capped at %d", got, maxContextWindowLines)
	}
	if !hasWarning(out.WarningFields, warnContextWindowCapped) {
		t.Fatalf("warnings = %+v, want %s", out.Warnings, warnContextWindowCapped)
	}
}
