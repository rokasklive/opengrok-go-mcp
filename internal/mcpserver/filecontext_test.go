// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

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
