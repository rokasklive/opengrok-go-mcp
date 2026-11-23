// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/links"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

// compactResults drops redundant or unused per-result fields when response_mode
// is compact. Context is not cleared here — compact mode skips expansion in
// search_core instead. Citation (title + url) is always preserved.
func compactResults(results []Result) []Result {
	for i := range results {
		results[i].DisplayTitle = ""
		results[i].DisplayURL = ""
		results[i].RawURL = nil
		results[i].Metadata = nil
	}
	return results
}

func applyMaxHitsPerFile(hits []opengrok.Hit, maxHitsPerFile int) []opengrok.Hit {
	fileCounts := make(map[string]int)
	filtered := make([]opengrok.Hit, 0, len(hits))
	for _, hit := range hits {
		key := hit.Project + "\x00" + hit.FilePath
		if fileCounts[key] < maxHitsPerFile {
			filtered = append(filtered, hit)
			fileCounts[key]++
		}
	}
	return filtered
}

func applySort(results []Result, sortOrder string) ([]Result, string, error) {
	switch strings.ToLower(sortOrder) {
	case "", "relevance":
		return results, "", nil
	case "path":
		sorted := make([]Result, len(results))
		copy(sorted, results)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].FilePath != sorted[j].FilePath {
				return sorted[i].FilePath < sorted[j].FilePath
			}
			return sorted[i].LineNumber < sorted[j].LineNumber
		})
		return sorted, "", nil
	case "date":
		return results, "Date sorting requires OpenGrok API support; results are returned in original order.", nil
	default:
		return nil, "", &Error{
			Code:    codeInvalidSort,
			Message: fmt.Sprintf("Invalid sort order %q; valid values: relevance, path, date", sortOrder),
		}
	}
}

func (s *Service) results(
	hits []opengrok.Hit,
	defaultProject string,
	symbol string,
	includeLinks bool,
	warnings *warningSet,
) []Result {
	results := make([]Result, 0, len(hits))
	for _, hit := range hits {
		project := hit.Project
		if project == "" {
			project = defaultProject
		}
		var fileLinks links.FileLinks
		hitCitation := citation(path.Base(hit.FilePath), "", 0)
		if hit.AttributionUncertain {
			warnings.addUncertainAttribution()
		} else {
			fileLinks = s.links.File(project, hit.FilePath, hit.LineNumber)
			hitCitation = citation(displayTitle(hit.FilePath, hit.LineNumber), fileLinks.DisplayURL, hit.LineNumber)
		}

		var resultSymbol *string
		if symbol != "" {
			value := symbol
			resultSymbol = &value
		}
		var attributionWarning *string
		if hit.AttributionWarning != "" {
			value := hit.AttributionWarning
			attributionWarning = &value
		}

		result := Result{
			ResultID:             project + ":" + hit.FilePath + ":" + strconv.Itoa(hit.LineNumber),
			Project:              project,
			FilePath:             hit.FilePath,
			AttributionUncertain: hit.AttributionUncertain,
			AttributionWarning:   attributionWarning,
			AttributionSource:    hit.AttributionSource,
			LineNumber:           hit.LineNumber,
			ColumnNumber:         nil,
			Kind:                 hit.Tag,
			Symbol:               resultSymbol,
			Snippet:              hit.Snippet,
			DisplayTitle:         displayTitle(hit.FilePath, hit.LineNumber),
			Citation:             hitCitation,
			ResourceURI:          fileLinks.ResourceURI,
		}
		if includeLinks {
			result.DisplayURL = fileLinks.DisplayURL
			result.RawURL = fileLinks.RawURL
		}

		results = append(results, result)
	}

	return results
}

type fileKey struct {
	project  string
	filePath string
}

type fileFetchResult struct {
	key     fileKey
	content string
	err     error
}

func (s *Service) expandResultContexts(ctx context.Context, results []Result, budget config.BudgetValues) []Result {
	expanded, _ := s.expandResultContextsWithDiagnostics(ctx, results, budget, newWarningSet())
	return expanded
}

func (s *Service) expandResultContextsWithDiagnostics(ctx context.Context, results []Result, budget config.BudgetValues, warnings *warningSet) ([]Result, *ExpansionDiagnostics) {
	diagnostics := &ExpansionDiagnostics{
		Requested:        len(results),
		FetchConcurrency: s.contextFetchConcurrency(),
	}
	if len(results) == 0 {
		return results, diagnostics
	}

	eligibleResults := len(results)
	if budget.MaxExpandedResults >= 0 {
		eligibleResults = min(eligibleResults, budget.MaxExpandedResults)
	}
	diagnostics.SkippedResults = len(results) - eligibleResults

	fileGroups := make(map[fileKey][]int)
	orderedKeys := make([]fileKey, 0, eligibleResults)
	for i, r := range results[:eligibleResults] {
		if r.AttributionUncertain || r.LineNumber <= 0 {
			continue
		}
		key := fileKey{project: r.Project, filePath: r.FilePath}
		if _, ok := fileGroups[key]; !ok {
			orderedKeys = append(orderedKeys, key)
		}
		fileGroups[key] = append(fileGroups[key], i)
	}
	if budget.MaxExpandedFiles >= 0 && len(orderedKeys) > budget.MaxExpandedFiles {
		diagnostics.SkippedFiles = len(orderedKeys) - budget.MaxExpandedFiles
		orderedKeys = orderedKeys[:budget.MaxExpandedFiles]
	}
	if len(orderedKeys) == 0 {
		diagnostics.SkippedResults = len(results)
		return results, diagnostics
	}

	jobs := make(chan fileKey)
	ch := make(chan fileFetchResult, len(orderedKeys))
	workerCount := min(diagnostics.FetchConcurrency, len(orderedKeys))
	for range workerCount {
		go func() {
			for key := range jobs {
				ch <- s.fetchExpandedFile(ctx, key)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, key := range orderedKeys {
			jobs <- key
		}
	}()

	fileContents := make(map[fileKey]string, len(orderedKeys))
	for range orderedKeys {
		r := <-ch
		if r.err == nil {
			fileContents[r.key] = r.content
		}
	}

	for i := range results[:eligibleResults] {
		result := &results[i]
		if result.AttributionUncertain || result.LineNumber <= 0 {
			continue
		}
		key := fileKey{project: result.Project, filePath: result.FilePath}
		content, ok := fileContents[key]
		if !ok {
			continue
		}
		window, _, stale := s.searchHitWindow(result, content, budget.ContextBefore, budget.ContextAfter)
		if stale {
			warnings.addStaleSearchHit()
		}
		if window.StartLine > 0 {
			result.Context = &window
			diagnostics.ExpandedResults++
			diagnostics.ExpandedContextBytes += len(window.Content)
		}
	}
	diagnostics.FetchedFiles = len(fileContents)
	diagnostics.SkippedResults = len(results) - diagnostics.ExpandedResults

	return results, diagnostics
}

// searchHitWindow keeps indexed coordinates for identification and drops line
// anchors from the evidence links when the current file lacks the indexed line.
func (s *Service) searchHitWindow(result *Result, content string, before, after int) (window ResultContext, capped, stale bool) {
	window, capped = extractCappedWindow(content, result.LineNumber, before, after)
	stale = result.LineNumber > 0 && window.StartLine == 0
	if stale {
		fileLinks := s.links.File(result.Project, result.FilePath, 0)
		result.Citation = citation(path.Base(result.FilePath), fileLinks.DisplayURL, 0)
		result.ResourceURI = fileLinks.ResourceURI
		if result.DisplayURL != "" {
			result.DisplayTitle = result.Citation.Title
			result.DisplayURL = fileLinks.DisplayURL
		}
	}
	return window, capped, stale
}

func (s *Service) fetchExpandedFile(ctx context.Context, key fileKey) (result fileFetchResult) {
	result.key = key
	defer func() {
		if recovered := recover(); recovered != nil {
			result.err = fmt.Errorf("file content panic for %s/%s: %v", key.project, key.filePath, recovered)
		}
	}()

	if err := ctx.Err(); err != nil {
		result.err = err
		return result
	}
	result.content, result.err = s.backend.FileContent(ctx, key.project, key.filePath)
	return result
}

func (s *Service) contextFetchConcurrency() int {
	if s.cfg.ContextFetchConcurrency <= 0 {
		return 1
	}

	return s.cfg.ContextFetchConcurrency
}

func emptySearchOutput(mode string, query string) SearchOutput {
	return SearchOutput{
		Mode:    mode,
		Query:   query,
		Results: []Result{},
	}
}
