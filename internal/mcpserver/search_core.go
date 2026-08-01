// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

type searchRequest struct {
	project         string
	projects        []string
	query           string
	userQuery       string // trimmed, pre-normalization user query; used for warnings. Empty on the symbol path.
	autoQuoted      bool   // true when normalizeCodeQuery wrapped the query in quotes
	mode            string
	pathPrefix      string
	fileType        string
	pageSize        int
	cursor          string
	includeLinks    *bool
	includeSnippets *bool
	maxHitsPerFile  int
	sort            string
	symbol          string
	expandContext   bool
	// expandContextSet records that the caller passed expand_context
	// explicitly, as opposed to it being derived from the agent profile.
	expandContextSet bool
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
	lineOffset := 0
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
		lineOffset = state.LineOffset
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
			// OpenGrok answers 400 both for a malformed query and for a search
			// mode the instance does not serve at all. When startup probing
			// already found the mode unavailable, the query is not the problem
			// and query-syntax advice actively misleads the caller.
			if modeErr := s.unsupportedModeError(req.mode); modeErr != nil {
				return emptySearchOutput(req.mode, req.query), modeErr
			}
			return emptySearchOutput(req.mode, req.query), newQueryParserError(req.query, err)
		}
		return emptySearchOutput(req.mode, req.query), fmt.Errorf("search: %w", err)
	}

	// The file window starting at offset holds every line match in those files.
	// A page returns at most pageSize of them, resuming at lineOffset, so the
	// remainder of a line-dense window is reachable on the next page instead of
	// being skipped when the cursor advances past the window.
	windowHits := result.Hits
	warnings := newWarningSet()
	if lineOffset > len(windowHits) {
		// The window shrank (reindex, or a changed corpus) — the saved position
		// no longer exists, and silently returning page 1 would look like a loop.
		return emptySearchOutput(req.mode, req.query), invalidCursorError()
	}
	hits := windowHits[lineOffset:]
	windowExhausted := true
	if pageSize > 0 && len(hits) > pageSize {
		remaining := len(hits) - pageSize
		hits = hits[:pageSize]
		windowExhausted = false
		warnings.add(warnPageSizeTruncated, fmt.Sprintf(
			"The files on this page hold %d more matching lines than page_size (%d) allows. "+
				"They are NOT lost — pass next_cursor to continue through them before the search moves on to further files. "+
				"Raise page_size to see more per call.",
			remaining, pageSize,
		))
	}

	// Advance within the window while lines remain; otherwise move to the next
	// file window and restart at its first line.
	nextState := cursor.State{
		Project:    project,
		Projects:   projects,
		Query:      req.query,
		Mode:       req.mode,
		PageSize:   pageSize,
		PathPrefix: req.pathPrefix,
		FileType:   req.fileType,
	}
	if windowExhausted {
		nextState.Offset = offset + pageSize
		nextState.LineOffset = 0
	} else {
		nextState.Offset = offset
		nextState.LineOffset = lineOffset + len(hits)
	}

	nextCursor, err := s.nextCursor(nextState, result.TotalHits, windowExhausted)
	if err != nil {
		return emptySearchOutput(req.mode, req.query), fmt.Errorf("search cursor: %w", err)
	}

	if req.autoQuoted {
		warnings.add(warnAutoQuotedQuery, "Matched multi-word query as an exact phrase — the default, and usually the most precise result for code. Review these hits before changing; set tokenized:true only if you specifically need broader independent-term (bag-of-words) matching.")
	}
	if req.userQuery != "" && req.mode != string(opengrok.ModeHistory) && queryHasDateField(req.userQuery) {
		warnings.add(warnDateIgnored, "date: is only valid in history mode and was ignored in this search.")
	}
	// An unrecognized analyzer name is not an error to OpenGrok — it just
	// matches nothing. Without this the caller sees an empty result set and
	// concludes the code does not exist.
	if req.fileType != "" && result.TotalHits == 0 {
		warnings.add(warnFileTypeNoMatch, fmt.Sprintf(
			"No hits with file_type %q. file_type is an OpenGrok analyzer name, not a file extension — "+
				"common mistakes are \"go\" (use \"golang\"), \"cs\" (use \"csharp\"), \"py\" (use \"python\"). "+
				"Retry without file_type to confirm the query itself matches.",
			req.fileType,
		))
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
	// response_mode=compact skips expansion by default, but an explicit
	// expand_context=true is a direct request and outranks that default.
	// Without this, expand_context had no effect at all under the economy
	// profile, where compact is the default response mode.
	if req.responseMode != "compact" || req.expandContextSet {
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
		if !req.expandContextSet {
			expansion = nil
		}
	}

	return SearchOutput{
		Project:       project,
		Mode:          req.mode,
		Query:         req.query,
		Pagination:    newPagination(offset, pageSize, totalHits, len(results), nextCursor),
		Results:       results,
		WarningFields: warnings.fields(),
		Diagnostics:   s.searchDiagnostics(offset, result.Start, pageSize),
		Expansion:     expansion,
	}, nil
}

// unsupportedModeError returns a capability error when the requested search
// mode was found unavailable during startup probing, and nil otherwise.
func (s *Service) unsupportedModeError(mode string) *Error {
	var available bool
	switch opengrok.Mode(mode) {
	case opengrok.ModeReference:
		available = s.cfg.Capabilities.SearchSymbolReferences
	case opengrok.ModeDefinition:
		available = s.cfg.Capabilities.SearchSymbolDefinitions
	default:
		return nil
	}
	if available {
		return nil
	}

	return &Error{
		Code: codeSearchModeUnsupported,
		Message: fmt.Sprintf(
			"This OpenGrok instance does not serve %q searches; startup capability probing found the mode unavailable.",
			mode,
		),
		Suggestion: "The query is not the problem — no query will succeed in this mode here. " +
			"Use operation=code (full-text) and filter with path_prefix, or check opengrok://capabilities for what this instance supports.",
	}
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
