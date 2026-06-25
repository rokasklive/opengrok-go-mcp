// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

type searchRequest struct {
	project          string
	projects         []string
	query            string
	userQuery        string // trimmed, pre-normalization user query; used for warnings. Empty on the symbol path.
	autoQuoted       bool   // true when normalizeCodeQuery wrapped the query in quotes
	mode             string
	pathPrefix       string
	fileType         string
	pageSize         int
	cursor           string
	includeLinks     *bool
	includeSnippets  *bool
	maxHitsPerFile   int
	sort             string
	symbol           string
	expandContext    bool
	allowAllProjects bool
	responseMode     string
	contextBudget    string
}

func (s *Service) search(ctx context.Context, req searchRequest) (SearchOutput, error) {
	if err := validateResponseMode(req.responseMode); err != nil {
		return emptySearchOutput(req.mode, req.query), err
	}

	budget, err := s.resolveBudgetTier(req.contextBudget)
	if err != nil {
		return emptySearchOutput(req.mode, req.query), err
	}

	projects, err := s.resolveSearchProjects(req.project, req.projects, req.allowAllProjects)
	if err != nil {
		return emptySearchOutput(req.mode, req.query), err
	}
	project := firstProject(projects)

	pageSize := s.pageSize(req.pageSize)
	offset := 0
	if req.cursor != "" {
		state, err := cursor.Decode(req.cursor)
		if err != nil {
			return emptySearchOutput(req.mode, req.query), invalidCursorError()
		}

		expected := cursor.State{
			Project:    project,
			Projects:   projects,
			Query:      req.query,
			Mode:       req.mode,
			PathPrefix: req.pathPrefix,
			FileType:   req.fileType,
		}
		if err := state.Validate(expected); err != nil {
			return emptySearchOutput(req.mode, req.query), invalidCursorError()
		}

		offset = state.Offset
		pageSize = s.pageSize(state.PageSize)
	}

	result, err := s.backend.Search(ctx, opengrok.SearchRequest{
		Projects:   projects,
		Query:      req.query,
		Mode:       opengrok.Mode(req.mode),
		PathPrefix: req.pathPrefix,
		FileType:   req.fileType,
		Limit:      pageSize,
		Offset:     offset,
	})
	if err != nil {
		if isQueryParserStatus(err) {
			return emptySearchOutput(req.mode, req.query), newQueryParserError(req.query, err)
		}
		return emptySearchOutput(req.mode, req.query), fmt.Errorf("search: %w", err)
	}

	hits := result.Hits
	warnings := newWarningSet()
	if pageSize > 0 && len(hits) > pageSize {
		warnings.add(warnPageSizeTruncated, fmt.Sprintf(
			"OpenGrok returned %d hits for page_size %d; results truncated to %d.",
			len(hits), pageSize, pageSize,
		))
		hits = hits[:pageSize]
	}

	nextCursor, err := s.nextCursor(cursor.State{
		Project:    project,
		Projects:   projects,
		Query:      req.query,
		Mode:       req.mode,
		Offset:     offset + pageSize,
		PageSize:   pageSize,
		PathPrefix: req.pathPrefix,
		FileType:   req.fileType,
	}, result.TotalHits)
	if err != nil {
		return emptySearchOutput(req.mode, req.query), fmt.Errorf("search cursor: %w", err)
	}

	if req.autoQuoted {
		warnings.add(warnAutoQuotedQuery, "Matched multi-word query as an exact phrase — the default, and usually the most precise result for code. Review these hits before changing; set tokenized:true only if you specifically need broader independent-term (bag-of-words) matching.")
	}
	if req.userQuery != "" && req.mode != string(opengrok.ModeHistory) && queryHasDateField(req.userQuery) {
		warnings.add(warnDateIgnored, "date: is only valid in history mode and was ignored in this search.")
	}
	if result.TotalHits > searchWarnThreshold {
		msg := fmt.Sprintf("Query returned %d hits. Consider narrowing with path_prefix, file_type, or a more specific query.", result.TotalHits)
		if req.userQuery != "" && !req.autoQuoted && isMultiWord(req.userQuery) {
			msg += fmt.Sprintf(" For an exact phrase, wrap it in quotes: %q.", req.userQuery)
		}
		warnings.add(warnHighHitCount, msg)
	}

	results := s.results(hits, project, req.mode, req.symbol, s.includeLinks(req.includeLinks))

	totalHits := result.TotalHits
	if req.maxHitsPerFile > 0 {
		results = applyMaxHitsPerFile(results, req.maxHitsPerFile)
	}

	sortedResults, sortWarning, sortErr := applySort(results, req.sort)
	if sortErr != nil {
		return emptySearchOutput(req.mode, req.query), sortErr
	}
	if sortWarning != "" {
		warnings.add(warnSortUnsupported, sortWarning)
	}
	results = sortedResults

	var expansion *ExpansionDiagnostics
	if req.responseMode != "compact" {
		results, expansion = s.maybeExpandResults(ctx, results, req.expandContext, budget)
		if expansion != nil {
			maybeWarnExpansionBudget(warnings, expansion, results)
		}
	}

	if req.includeSnippets != nil && !*req.includeSnippets {
		for i := range results {
			results[i].Snippet = nil
		}
	}

	if req.responseMode == "compact" {
		results = compactResults(results)
		expansion = nil
	}

	return SearchOutput{
		Project:       project,
		Mode:          req.mode,
		Query:         req.query,
		Pagination:    newPagination(offset, pageSize, totalHits, nextCursor),
		Results:       results,
		WarningFields: warnings.fields(),
		Diagnostics:   s.searchDiagnostics(offset, result.Start, pageSize),
		Expansion:     expansion,
	}, nil
}

func (s *Service) searchDiagnostics(offset int, start int, maxResults int) *Diagnostics {
	if !s.cfg.Diagnostics {
		return nil
	}
	return &Diagnostics{
		OffsetUsed:         offset,
		OpenGrokStart:      start,
		OpenGrokMaxResults: maxResults,
	}
}
