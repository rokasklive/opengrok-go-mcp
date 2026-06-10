// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func (s *Service) ListSymbols(ctx context.Context, input ListSymbolsInput) (ListSymbolsOutput, error) {
	projects, err := s.resolveSearchProjects(input.Project, input.Projects, false)
	if err != nil {
		return ListSymbolsOutput{Symbols: []SymbolItem{}}, err
	}
	project := firstProject(projects)

	pageSize := s.pageSize(input.PageSize)
	offset := 0

	query := input.Symbol
	if query == "" {
		query = "*"
	}

	if input.Cursor != nil && *input.Cursor != "" {
		state, err := cursor.Decode(*input.Cursor)
		if err != nil {
			return ListSymbolsOutput{Symbols: []SymbolItem{}}, invalidCursorError()
		}
		expected := cursor.State{
			Project:    project,
			Projects:   projects,
			Query:      query,
			Mode:       string(opengrok.ModeDefinition),
			PathPrefix: input.PathPrefix,
			FileType:   input.FileType,
		}
		if err := state.Validate(expected); err != nil {
			return ListSymbolsOutput{Symbols: []SymbolItem{}}, invalidCursorError()
		}
		offset = state.Offset
		pageSize = s.pageSize(state.PageSize)
	}

	result, err := s.backend.Search(ctx, opengrok.SearchRequest{
		Projects:   projects,
		Query:      query,
		Mode:       opengrok.ModeDefinition,
		PathPrefix: input.PathPrefix,
		FileType:   input.FileType,
		Limit:      pageSize,
		Offset:     offset,
	})
	if err != nil {
		return ListSymbolsOutput{Symbols: []SymbolItem{}}, fmt.Errorf("list symbols: %w", err)
	}

	hits := result.Hits
	if input.Kind != "" {
		filtered := make([]opengrok.Hit, 0, len(hits))
		for _, h := range hits {
			if h.Tag == input.Kind {
				filtered = append(filtered, h)
			}
		}
		hits = filtered
	}

	includeSnippets := input.IncludeSnippets == nil || *input.IncludeSnippets
	includeLinks := s.includeLinks(input.IncludeLinks)

	symbols := make([]SymbolItem, 0, len(hits))
	for _, h := range hits {
		hitProject := h.Project
		if hitProject == "" {
			hitProject = project
		}
		fileLinks := s.links.File(hitProject, h.FilePath, h.LineNumber)

		item := SymbolItem{
			Project:     hitProject,
			FilePath:    h.FilePath,
			Kind:        h.Tag,
			LineNumber:  h.LineNumber,
			ResourceURI: fileLinks.ResourceURI,
		}
		if includeSnippets {
			item.Snippet = h.Snippet
		}
		if includeLinks {
			item.DisplayURL = fileLinks.DisplayURL
		}
		symbols = append(symbols, item)
	}

	nextCursor, err := s.nextCursor(cursor.State{
		Project:    project,
		Projects:   projects,
		Query:      query,
		Mode:       string(opengrok.ModeDefinition),
		Offset:     offset + pageSize,
		PageSize:   pageSize,
		PathPrefix: input.PathPrefix,
		FileType:   input.FileType,
	}, result.TotalHits)
	if err != nil {
		return ListSymbolsOutput{Symbols: []SymbolItem{}}, fmt.Errorf("list symbols cursor: %w", err)
	}

	var warning *string
	if result.TotalHits > listSymbolsWarnThreshold {
		morePages := (result.TotalHits - 1) / pageSize
		w := fmt.Sprintf(
			"Query matched %d definitions. At page_size %d, full enumeration would require ~%d more calls. Provide path_prefix or kind to narrow.",
			result.TotalHits, pageSize, morePages,
		)
		warning = &w
	}
	if input.Kind != "" && nextCursor != nil {
		kw := fmt.Sprintf(
			"total_hits (%d) counts all definitions before the %q kind filter; OpenGrok cannot filter by ctags kind server-side, so the global count of %q definitions across pages is unknown. This page contains %d matching %q definitions. Narrow with path_prefix to enumerate fully.",
			result.TotalHits, input.Kind, input.Kind, len(hits), input.Kind,
		)
		if warning != nil {
			combined := *warning + " " + kw
			warning = &combined
		} else {
			warning = &kw
		}
	}

	return ListSymbolsOutput{
		Symbols:    symbols,
		Pagination: newPagination(offset, pageSize, result.TotalHits, nextCursor),
		Warning:    warning,
	}, nil
}
