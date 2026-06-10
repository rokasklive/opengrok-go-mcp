// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) SearchAndRead(ctx context.Context, input SearchAndReadInput) (SearchAndReadOutput, error) {
	mode := input.Mode
	if mode == "" {
		mode = defaultSearchMode
	}

	tokenized := input.Tokenized != nil && *input.Tokenized
	normalized, autoQuoted := normalizeCodeQuery(input.Query, tokenized)
	finalQuery := appendPathExcludes(normalized, input.PathExclude)

	searchOutput, err := s.search(ctx, searchRequest{
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
		maxHitsPerFile:   0,
		sort:             "",
		expandContext:    false,
		allowAllProjects: input.AllowAllProjects != nil && *input.AllowAllProjects,
		responseMode:     input.ResponseMode,
		contextBudget:    input.ContextBudget,
	})
	if err != nil {
		return SearchAndReadOutput{}, err
	}

	budget, err := s.resolveBudgetTier(input.ContextBudget)
	if err != nil {
		return SearchAndReadOutput{}, err
	}

	before := input.LinesBefore
	if before == 0 {
		before = budget.ContextBefore
	}
	after := input.LinesAfter
	if after == 0 {
		after = budget.ContextAfter
	}

	results := searchOutput.Results
	if input.MaxResults > 0 && len(results) > input.MaxResults {
		results = results[:input.MaxResults]
	}

	readResults := make([]SearchAndReadResult, 0, len(results))
	failedReads := 0
	for _, result := range results {
		content, err := s.backend.FileContent(ctx, result.Project, result.FilePath)
		if err != nil {
			failedReads++
			continue
		}
		window := extractWindow(content, result.LineNumber, before, after)
		readResults = append(readResults, SearchAndReadResult{
			ResultID:    result.ResultID,
			Project:     result.Project,
			FilePath:    result.FilePath,
			LineNumber:  result.LineNumber,
			Kind:        result.Kind,
			Symbol:      result.Symbol,
			Snippet:     result.Snippet,
			Content:     window.Content,
			StartLine:   window.StartLine,
			EndLine:     window.EndLine,
			Citation:    result.Citation,
			ResourceURI: result.ResourceURI,
		})
	}

	var warning *string
	if failedReads > 0 {
		warning = appendWarning(warning, fmt.Sprintf("Failed to read %d result files; results may be incomplete.", failedReads))
	}
	if searchOutput.Warning != nil {
		warning = appendWarning(warning, *searchOutput.Warning)
	}

	return SearchAndReadOutput{
		Project:     searchOutput.Project,
		Mode:        searchOutput.Mode,
		Query:       searchOutput.Query,
		TotalHits:   searchOutput.TotalHits,
		Results:     readResults,
		PageSize:    searchOutput.PageSize,
		NextCursor:  searchOutput.NextCursor,
		Warning:     warning,
		Diagnostics: searchOutput.Diagnostics,
	}, nil
}

func (s *Service) FindSymbolAndReferences(ctx context.Context, input FindSymbolAndReferencesInput) (FindSymbolAndReferencesOutput, error) {
	budget, err := s.resolveBudgetTier(input.ContextBudget)
	if err != nil {
		return FindSymbolAndReferencesOutput{}, err
	}

	referenceInput := SymbolSearchInput{
		Project:          input.Project,
		Projects:         input.Projects,
		Symbol:           input.Symbol,
		PageSize:         input.PageSize,
		Cursor:           input.Cursor,
		IncludeLinks:     input.IncludeLinks,
		IncludeSnippets:  input.IncludeSnippets,
		AllowAllProjects: input.AllowAllProjects,
		ResponseMode:     input.ResponseMode,
		ContextBudget:    input.ContextBudget,
	}
	if err := s.validateSymbolReferenceCursor(referenceInput); err != nil {
		return FindSymbolAndReferencesOutput{}, err
	}

	defOutput, err := s.SearchSymbolDefinitions(ctx, SymbolSearchInput{
		Project:          input.Project,
		Projects:         input.Projects,
		Symbol:           input.Symbol,
		IncludeLinks:     input.IncludeLinks,
		IncludeSnippets:  input.IncludeSnippets,
		AllowAllProjects: input.AllowAllProjects,
		ResponseMode:     input.ResponseMode,
		ContextBudget:    input.ContextBudget,
	})
	if err != nil {
		return FindSymbolAndReferencesOutput{}, err
	}

	var definition *SearchAndReadResult
	if len(defOutput.Results) > 0 {
		defResult := defOutput.Results[0]
		content, err := s.backend.FileContent(ctx, defResult.Project, defResult.FilePath)
		if err == nil {
			window := extractWindow(content, defResult.LineNumber, budget.ContextBefore, budget.ContextAfter)
			definition = &SearchAndReadResult{
				ResultID:    defResult.ResultID,
				Project:     defResult.Project,
				FilePath:    defResult.FilePath,
				LineNumber:  defResult.LineNumber,
				Kind:        defResult.Kind,
				Symbol:      defResult.Symbol,
				Snippet:     defResult.Snippet,
				Content:     window.Content,
				StartLine:   window.StartLine,
				EndLine:     window.EndLine,
				Citation:    defResult.Citation,
				ResourceURI: defResult.ResourceURI,
			}
		}
	}

	refOutput, err := s.SearchSymbolReferences(ctx, referenceInput)
	if err != nil {
		return FindSymbolAndReferencesOutput{}, err
	}

	var warning *string
	if definition == nil {
		w := fmt.Sprintf("No definition found for symbol %q.", input.Symbol)
		warning = &w
	}
	if refOutput.Warning != nil {
		if warning != nil {
			combined := *warning + " " + *refOutput.Warning
			warning = &combined
		} else {
			warning = refOutput.Warning
		}
	}

	return FindSymbolAndReferencesOutput{
		Symbol:      input.Symbol,
		Definition:  definition,
		References:  refOutput.Results,
		TotalRefs:   refOutput.TotalHits,
		PageSize:    refOutput.PageSize,
		NextCursor:  refOutput.NextCursor,
		Warning:     warning,
		Diagnostics: refOutput.Diagnostics,
	}, nil
}
