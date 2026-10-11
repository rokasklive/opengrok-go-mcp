// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
)

func (s *Service) GetFileContext(ctx context.Context, input FileContextInput) (FileContextOutput, error) {
	projects, err := s.resolveProjects(input.Project, nil, false)
	if err != nil {
		return FileContextOutput{}, err
	}

	budget, err := s.resolveBudgetTier(input.ContextBudget)
	if err != nil {
		return FileContextOutput{}, err
	}
	if input.Before == 0 {
		input.Before = budget.ContextBefore
	}
	if input.After == 0 {
		input.After = budget.ContextAfter
	}

	content, err := s.backend.FileContent(ctx, projects[0], input.FilePath)
	if err != nil {
		return FileContextOutput{}, mapFileContentError(err)
	}

	if input.LineNumber > 0 {
		return s.windowedFileContext(projects[0], content, input)
	}
	return s.pagedFileContext(projects[0], content, input)
}

func (s *Service) windowedFileContext(project string, content string, input FileContextInput) (FileContextOutput, error) {
	selectedContent, selectedLine, startLine, endLine, capped := fileContextLines(content, input)
	lines := fileLines(content)
	totalLines := len(lines)
	fileLinks := s.links.File(project, input.FilePath, selectedLine)
	var hint *string
	if capped {
		hintText := fmt.Sprintf(
			"Window capped at %d lines, showing %d–%d of %d. Request another window (a different line_number) to see lines outside it.",
			maxContextWindowLines, startLine, endLine, totalLines,
		)
		hint = &hintText
	}
	output := FileContextOutput{
		Project:              project,
		FilePath:             input.FilePath,
		LineNumber:           selectedLine,
		StartLine:            startLine,
		EndLine:              endLine,
		TotalLines:           totalLines,
		Truncated:            capped,
		Hint:                 hint,
		Content:              selectedContent,
		DisplayURL:           fileLinks.DisplayURL,
		RawURL:               fileLinks.RawURL,
		Citation:             citation(displayTitle(input.FilePath, selectedLine), fileLinks.DisplayURL, selectedLine),
		AnnotationsAvailable: input.IncludeAnnotations,
		ResourceURI:          fileLinks.ResourceURI,
	}
	if !s.includeLinks(input.IncludeLinks) {
		output.DisplayURL = ""
		output.RawURL = nil
	}
	return output, nil
}

func (s *Service) pagedFileContext(project string, content string, input FileContextInput) (FileContextOutput, error) {
	startLine := 1

	if input.Cursor != nil && *input.Cursor != "" {
		state, err := cursor.DecodeFile(*input.Cursor)
		if err != nil {
			return FileContextOutput{}, invalidCursorError()
		}
		if state.Project != project || state.FilePath != input.FilePath {
			return FileContextOutput{}, invalidCursorError()
		}
		startLine = state.StartLine
	}

	lines := fileLines(content)
	totalLines := len(lines)

	if totalLines == 0 {
		return FileContextOutput{
			Project:     project,
			FilePath:    input.FilePath,
			TotalLines:  0,
			Content:     "",
			ResourceURI: s.links.File(project, input.FilePath, 0).ResourceURI,
		}, nil
	}

	if startLine > totalLines {
		startLine = totalLines
	}

	endLine := min(startLine+filePageSize-1, totalLines)

	var selectedContent string
	if startLine == 1 && endLine == totalLines {
		selectedContent = content
	} else {
		selectedContent = strings.Join(lines[startLine-1:endLine], "\n")
	}

	truncated := endLine < totalLines

	fileLinks := s.links.File(project, input.FilePath, 0)

	var nextCursor *string
	var hint *string
	if truncated {
		encoded, err := cursor.EncodeFile(cursor.FileState{
			Project:   project,
			FilePath:  input.FilePath,
			StartLine: endLine + 1,
			PageSize:  filePageSize,
		})
		if err != nil {
			return FileContextOutput{}, fmt.Errorf("file cursor: %w", err)
		}
		nextCursor = &encoded
		hintText := fmt.Sprintf("File has %d lines, showing %d–%d. Pass next_cursor to read the next section.", totalLines, startLine, endLine)
		hint = &hintText
	}

	output := FileContextOutput{
		Project:              project,
		FilePath:             input.FilePath,
		LineNumber:           0,
		StartLine:            startLine,
		EndLine:              endLine,
		TotalLines:           totalLines,
		Truncated:            truncated,
		Content:              selectedContent,
		DisplayURL:           fileLinks.DisplayURL,
		RawURL:               fileLinks.RawURL,
		Citation:             citation(displayTitle(input.FilePath, 0), fileLinks.DisplayURL, 0),
		NextCursor:           nextCursor,
		Hint:                 hint,
		AnnotationsAvailable: input.IncludeAnnotations,
		ResourceURI:          fileLinks.ResourceURI,
	}
	if !s.includeLinks(input.IncludeLinks) {
		output.DisplayURL = ""
		output.RawURL = nil
	}
	return output, nil
}

func fileContextLines(content string, input FileContextInput) (string, int, int, int, bool) {
	lines := fileLines(content)
	totalLines := len(lines)
	if input.LineNumber <= 0 {
		if totalLines == 0 {
			return content, 0, 0, 0, false
		}

		return content, 0, 1, totalLines, false
	}
	if totalLines == 0 {
		return "", 0, 0, 0, false
	}

	before := contextWindow(input.Before, defaultBefore)
	after := contextWindow(input.After, defaultAfter)
	selectedLine := min(input.LineNumber, totalLines)
	// The window is anchored on the requested line, so a line_number past EOF
	// yields only the clamped last line rather than a window before it.
	startLine, endLine, capped := windowBounds(totalLines, input.LineNumber, before, after)

	return strings.Join(lines[startLine-1:endLine], "\n"), selectedLine, startLine, endLine, capped
}

// maxContextWindowLines bounds every line_number-centred window — the same
// size as one page of a paged file read — so before/after cannot pull a whole
// large file into a single response.
const maxContextWindowLines = filePageSize

// windowBounds returns the 1-based inclusive line range spanning before/after
// lines around anchor (>= 1, possibly past EOF), clipped to the file and
// capped at maxContextWindowLines lines. When the cap bites, the smaller side
// keeps its full span and the larger side gets the rest. capped reports
// whether the cap withheld lines the requested span would have included.
func windowBounds(totalLines int, anchor int, before int, after int) (int, int, bool) {
	before, after = max(before, 0), max(after, 0)
	start := min(max(1, anchor-before), totalLines)
	end := max(min(totalLines, anchor+after), start)
	if end-start+1 <= maxContextWindowLines {
		return start, end, false
	}
	if anchor > end {
		// Anchor past EOF: keep the lines nearest to it.
		return end - maxContextWindowLines + 1, end, true
	}

	spanBefore, spanAfter := anchor-start, end-anchor
	budget := maxContextWindowLines - 1
	half := budget / 2
	switch {
	case spanBefore <= half:
		spanAfter = budget - spanBefore
	case spanAfter <= budget-half:
		spanBefore = budget - spanAfter
	default:
		spanBefore, spanAfter = half, budget-half
	}

	return anchor - spanBefore, anchor + spanAfter, true
}

func contextWindow(value int, defaultValue int) int {
	if value < 0 {
		return 0
	}
	if value == 0 {
		return defaultValue
	}

	return value
}

func extractWindow(content string, lineNumber int, before int, after int) ResultContext {
	window, _ := extractCappedWindow(content, lineNumber, before, after)
	return window
}

// extractCappedWindow is extractWindow plus whether maxContextWindowLines
// withheld part of the requested span. Negative spans count as zero.
func extractCappedWindow(content string, lineNumber int, before int, after int) (ResultContext, bool) {
	lines := fileLines(content)
	totalLines := len(lines)
	if totalLines == 0 || lineNumber <= 0 {
		return ResultContext{}, false
	}
	// Negative spans would invert the slice bounds and panic the server.
	before = max(before, 0)
	after = max(after, 0)
	selectedLine := min(lineNumber, totalLines)
	startLine, endLine, capped := windowBounds(totalLines, selectedLine, before, after)
	return ResultContext{
		Content:   strings.Join(lines[startLine-1:endLine], "\n"),
		StartLine: startLine,
		EndLine:   endLine,
	}, capped
}

func fileLines(content string) []string {
	if content == "" {
		return []string{}
	}

	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		return lines[:len(lines)-1]
	}

	return lines
}
