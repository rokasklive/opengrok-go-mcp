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
		return FileContextOutput{}, fmt.Errorf("file context: %w", err)
	}

	if input.LineNumber > 0 {
		return s.windowedFileContext(projects[0], content, input)
	}
	return s.pagedFileContext(projects[0], content, input)
}

func (s *Service) windowedFileContext(project string, content string, input FileContextInput) (FileContextOutput, error) {
	selectedContent, selectedLine, startLine, endLine := fileContextLines(content, input)
	lines := fileLines(content)
	totalLines := len(lines)
	fileLinks := s.links.File(project, input.FilePath, selectedLine)
	output := FileContextOutput{
		Project:              project,
		FilePath:             input.FilePath,
		LineNumber:           selectedLine,
		StartLine:            startLine,
		EndLine:              endLine,
		TotalLines:           totalLines,
		Truncated:            false,
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

func fileContextLines(content string, input FileContextInput) (string, int, int, int) {
	lines := fileLines(content)
	totalLines := len(lines)
	if input.LineNumber <= 0 {
		if totalLines == 0 {
			return content, 0, 0, 0
		}

		return content, 0, 1, totalLines
	}
	if totalLines == 0 {
		return "", 0, 0, 0
	}

	before := contextWindow(input.Before, defaultBefore)
	after := contextWindow(input.After, defaultAfter)
	selectedLine := min(input.LineNumber, totalLines)
	startLine := max(1, input.LineNumber-before)
	endLine := min(totalLines, input.LineNumber+after)
	if startLine > totalLines {
		startLine = totalLines
	}
	if endLine < startLine {
		endLine = startLine
	}

	return strings.Join(lines[startLine-1:endLine], "\n"), selectedLine, startLine, endLine
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
	lines := fileLines(content)
	totalLines := len(lines)
	if totalLines == 0 || lineNumber <= 0 {
		return ResultContext{}
	}
	selectedLine := min(lineNumber, totalLines)
	startLine := max(1, selectedLine-before)
	endLine := min(totalLines, selectedLine+after)
	return ResultContext{
		Content:   strings.Join(lines[startLine-1:endLine], "\n"),
		StartLine: startLine,
		EndLine:   endLine,
	}
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
