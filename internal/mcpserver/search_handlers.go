// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func (s *Service) SearchCode(ctx context.Context, input SearchCodeInput) (SearchOutput, error) {
	mode := input.Mode
	if mode == "" {
		mode = defaultSearchMode
	}

	tokenized := input.Tokenized != nil && *input.Tokenized
	normalized, autoQuoted := normalizeCodeQuery(input.Query, tokenized)
	finalQuery := appendPathExcludes(normalized, input.PathExclude)

	return s.search(ctx, searchRequest{
		project:          input.Project,
		projects:         input.Projects,
		query:            finalQuery,
		userQuery:        strings.TrimSpace(input.Query),
		autoQuoted:       autoQuoted,
		mode:             mode,
		pathPrefix:       input.PathPrefix,
		fileType:         input.FileType,
		pageSize:         input.PageSize,
		cursor:           cursorValue(input.Cursor),
		includeLinks:     input.IncludeLinks,
		includeSnippets:  input.IncludeSnippets,
		maxHitsPerFile:   input.MaxHitsPerFile,
		sort:             input.Sort,
		expandContext:    s.shouldExpandContext(input.ExpandContext),
		allowAllProjects: input.AllowAllProjects != nil && *input.AllowAllProjects,
		responseMode:     input.ResponseMode,
		contextBudget:    input.ContextBudget,
	})
}

func (s *Service) SearchSymbolDefinitions(ctx context.Context, input SymbolSearchInput) (SearchOutput, error) {
	return s.search(ctx, searchRequest{
		project:          input.Project,
		projects:         input.Projects,
		query:            input.Symbol,
		mode:             string(opengrok.ModeDefinition),
		pageSize:         input.PageSize,
		cursor:           cursorValue(input.Cursor),
		includeLinks:     input.IncludeLinks,
		includeSnippets:  input.IncludeSnippets,
		maxHitsPerFile:   input.MaxHitsPerFile,
		sort:             input.Sort,
		symbol:           input.Symbol,
		expandContext:    s.shouldExpandContext(input.ExpandContext),
		allowAllProjects: input.AllowAllProjects != nil && *input.AllowAllProjects,
		responseMode:     input.ResponseMode,
		contextBudget:    input.ContextBudget,
	})
}

func (s *Service) SearchSymbolReferences(ctx context.Context, input SymbolSearchInput) (SearchOutput, error) {
	return s.search(ctx, searchRequest{
		project:          input.Project,
		projects:         input.Projects,
		query:            input.Symbol,
		mode:             string(opengrok.ModeReference),
		pageSize:         input.PageSize,
		cursor:           cursorValue(input.Cursor),
		includeLinks:     input.IncludeLinks,
		includeSnippets:  input.IncludeSnippets,
		maxHitsPerFile:   input.MaxHitsPerFile,
		sort:             input.Sort,
		symbol:           input.Symbol,
		expandContext:    s.shouldExpandContext(input.ExpandContext),
		allowAllProjects: input.AllowAllProjects != nil && *input.AllowAllProjects,
		responseMode:     input.ResponseMode,
		contextBudget:    input.ContextBudget,
	})
}

func (s *Service) validateSymbolReferenceCursor(input SymbolSearchInput) error {
	if err := validateResponseMode(input.ResponseMode); err != nil {
		return err
	}

	value := cursorValue(input.Cursor)
	if value == "" {
		return nil
	}

	projects, err := s.resolveSearchProjects(
		input.Project,
		input.Projects,
		input.AllowAllProjects != nil && *input.AllowAllProjects,
	)
	if err != nil {
		return err
	}

	state, err := cursor.Decode(value)
	if err != nil {
		return invalidCursorError()
	}

	expected := cursor.State{
		Project:  firstProject(projects),
		Projects: projects,
		Query:    input.Symbol,
		Mode:     string(opengrok.ModeReference),
	}
	if err := state.Validate(expected); err != nil {
		return invalidCursorError()
	}

	return nil
}

func (s *Service) SearchCrossProjectReferences(ctx context.Context, input CrossProjectReferencesInput) (CrossProjectReferencesOutput, error) {
	symbolSearchInput := SymbolSearchInput{
		Projects:         input.Projects,
		Symbol:           input.Symbol,
		PageSize:         input.PageSize,
		Cursor:           input.Cursor,
		IncludeLinks:     input.IncludeLinks,
		ExpandContext:    input.ExpandContext,
		MaxHitsPerFile:   input.MaxHitsPerFile,
		Sort:             input.Sort,
		AllowAllProjects: input.AllowAllProjects,
		ResponseMode:     input.ResponseMode,
		ContextBudget:    input.ContextBudget,
	}
	searchOutput, err := s.SearchSymbolReferences(ctx, symbolSearchInput)
	if err != nil {
		return CrossProjectReferencesOutput{}, err
	}

	groups := make([]ProjectReferenceGroup, 0)
	groupMap := make(map[string]int)
	for _, result := range searchOutput.Results {
		idx, ok := groupMap[result.Project]
		if !ok {
			idx = len(groups)
			groupMap[result.Project] = idx
			groups = append(groups, ProjectReferenceGroup{
				Project: result.Project,
				Results: []Result{},
			})
		}
		groups[idx].Results = append(groups[idx].Results, result)
		groups[idx].Total++
	}

	return CrossProjectReferencesOutput{
		Symbol:      input.Symbol,
		Projects:    groups,
		TotalHits:   searchOutput.TotalHits,
		PageSize:    searchOutput.PageSize,
		NextCursor:  searchOutput.NextCursor,
		Warning:     searchOutput.Warning,
		Diagnostics: searchOutput.Diagnostics,
	}, nil
}

func (s *Service) SearchImplementations(ctx context.Context, input ImplementationSearchInput) (SearchOutput, error) {
	symbolInput := SymbolSearchInput{
		Project:          input.Project,
		Projects:         input.Projects,
		Symbol:           input.Symbol,
		PageSize:         input.PageSize,
		Cursor:           input.Cursor,
		IncludeLinks:     input.IncludeLinks,
		ExpandContext:    input.ExpandContext,
		MaxHitsPerFile:   input.MaxHitsPerFile,
		Sort:             input.Sort,
		AllowAllProjects: input.AllowAllProjects,
		ResponseMode:     input.ResponseMode,
		ContextBudget:    input.ContextBudget,
	}
	output, err := s.SearchSymbolReferences(ctx, symbolInput)
	if err != nil {
		return output, err
	}
	warning := "OpenGrok does not provide language-semantic implementation mapping; results are candidate references from symbol usage."
	output.Warning = &warning
	// Results are best-effort candidate references, not exhaustive.
	output.BestEffort = &[]bool{true}[0]
	return output, nil
}
