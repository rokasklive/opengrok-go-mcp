// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/links"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

const (
	codeInvalidCursor        = "INVALID_CURSOR"
	codeProjectRequired      = "PROJECT_REQUIRED"
	codeUnknownProject       = "UNKNOWN_PROJECT"
	codeUnknownOperation     = "UNKNOWN_OPERATION"
	codeInvalidResponseMode  = "INVALID_RESPONSE_MODE"
	codeInvalidContextBudget = "INVALID_CONTEXT_BUDGET"

	defaultSearchMode = string(opengrok.ModeFullText)
	defaultBefore     = 30
	defaultAfter      = 60

	filePageSize             = 500
	projectsPageSize         = 50
	searchWarnThreshold      = 500
	listSymbolsWarnThreshold = 100
)

type Backend interface {
	ListProjects(ctx context.Context) ([]string, error)
	ListFiles(ctx context.Context, project string, path string) ([]opengrok.FileEntry, error)
	Search(ctx context.Context, req opengrok.SearchRequest) (opengrok.SearchResult, error)
	FileContent(ctx context.Context, project string, filePath string) (string, error)
	GetProjectOverview(ctx context.Context, project string) (opengrok.ProjectOverview, error)
}

type fileListMetadataBackend interface {
	ListFilesWithMetadata(ctx context.Context, project string, path string) ([]opengrok.FileEntry, bool, error)
}

func listFilesWithMetadata(
	ctx context.Context,
	backend Backend,
	project string,
	path string,
) ([]opengrok.FileEntry, bool, error) {
	if backend, ok := backend.(fileListMetadataBackend); ok {
		return backend.ListFilesWithMetadata(ctx, project, path)
	}

	entries, err := backend.ListFiles(ctx, project, path)
	return entries, false, err
}

type Service struct {
	cfg        config.Config
	backend    Backend
	links      links.Builder
	memoryBank *MemoryBank
}

type gatewayOperation struct {
	Manifest GatewayOperation
	Call     func(context.Context, json.RawMessage) (any, error)
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func IsCode(err error, code string) bool {
	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return serviceErr.Code == code
	}

	return false
}

func NewService(cfg config.Config, backend Backend) *Service {
	return &Service{
		cfg:        cfg,
		backend:    backend,
		links:      links.NewBuilder(cfg.OpenGrokWebBaseURL, cfg.EnableRawLinks),
		memoryBank: NewMemoryBank(),
	}
}

func (s *Service) MemorySet(ctx context.Context, input MemorySetInput) (MemorySetOutput, error) {
	s.memoryBank.Set(input.Key, input.Value)
	return MemorySetOutput{Success: true}, nil
}

func (s *Service) MemoryGet(ctx context.Context, input MemoryGetInput) (MemoryGetOutput, error) {
	value, found := s.memoryBank.Get(input.Key)
	return MemoryGetOutput{Value: value, Found: found}, nil
}

func (s *Service) MemoryList(ctx context.Context, input MemoryListInput) (MemoryListOutput, error) {
	return MemoryListOutput{Entries: s.memoryBank.List()}, nil
}

func (s *Service) MemoryDelete(ctx context.Context, input MemoryDeleteInput) (MemoryDeleteOutput, error) {
	_, found := s.memoryBank.Get(input.Key)
	if found {
		s.memoryBank.Delete(input.Key)
	}
	return MemoryDeleteOutput{Found: found, Deleted: found}, nil
}

func (s *Service) MemoryClear(ctx context.Context, input MemoryClearInput) (MemoryClearOutput, error) {
	s.memoryBank.Clear()
	return MemoryClearOutput{Success: true}, nil
}

func (s *Service) CompactMemory(ctx context.Context, input CompactMemoryInput) (any, error) {
	switch input.Operation {
	case "set":
		var payload MemorySetInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact memory set payload: %w", err)
		}
		return s.MemorySet(ctx, payload)
	case "get":
		var payload MemoryGetInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact memory get payload: %w", err)
		}
		return s.MemoryGet(ctx, payload)
	case "list":
		return s.MemoryList(ctx, MemoryListInput{})
	case "delete":
		var payload MemoryDeleteInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact memory delete payload: %w", err)
		}
		return s.MemoryDelete(ctx, payload)
	case "clear":
		return s.MemoryClear(ctx, MemoryClearInput{})
	default:
		return nil, unknownOperationError(input.Operation, compactMemoryOperations())
	}
}

func compactMemoryOperations() []string {
	return []string{"set", "get", "list", "delete", "clear"}
}

func (s *Service) ListProjects(ctx context.Context, input ListProjectsInput) (ListProjectsOutput, error) {
	offset := 0
	pageSize := projectsPageSize

	if input.Cursor != nil && *input.Cursor != "" {
		state, err := cursor.DecodeProjects(*input.Cursor)
		if err != nil {
			return ListProjectsOutput{}, invalidCursorError()
		}
		offset = state.Offset
		pageSize = state.PageSize
	}

	allProjects := s.cfg.Projects
	if len(allProjects) == 0 && s.cfg.DefaultProject != "" {
		allProjects = []string{s.cfg.DefaultProject}
	}

	total := len(allProjects)
	if offset > total {
		offset = total
	}
	end := min(offset+pageSize, total)
	page := allProjects[offset:end]

	items := make([]ProjectItem, 0, len(page))
	for _, project := range page {
		items = append(items, ProjectItem{
			Project:     project,
			Title:       project,
			Description: "Indexed OpenGrok project",
			ProjectURL:  s.links.Search(project, defaultSearchMode, ""),
			ResourceURI: s.links.Project(project),
		})
	}

	var nextCursor *string
	if offset+pageSize < total {
		encoded, err := cursor.EncodeProjects(cursor.ProjectsState{
			Offset:   offset + pageSize,
			PageSize: pageSize,
		})
		if err != nil {
			return ListProjectsOutput{}, fmt.Errorf("projects cursor: %w", err)
		}
		nextCursor = &encoded
	}

	return ListProjectsOutput{
		Projects:      items,
		TotalProjects: total,
		NextCursor:    nextCursor,
	}, nil
}

func (s *Service) ListFiles(ctx context.Context, input ListFilesInput) (ListFilesOutput, error) {
	projects, err := s.resolveProjects(input.Project, nil, false)
	if err != nil {
		return ListFilesOutput{}, err
	}
	project := projects[0]

	offset := 0
	pageSize := filePageSize
	if input.PageSize > 0 {
		pageSize = input.PageSize
	}
	if s.cfg.PageSizeMax > 0 && pageSize > s.cfg.PageSizeMax {
		pageSize = s.cfg.PageSizeMax
	}

	if input.Cursor != nil && *input.Cursor != "" {
		state, err := cursor.DecodeFileList(*input.Cursor)
		if err != nil {
			return ListFilesOutput{}, invalidCursorError()
		}
		if state.Project != project || state.Path != input.Path {
			return ListFilesOutput{}, invalidCursorError()
		}
		offset = state.Offset
		pageSize = state.PageSize
		if s.cfg.PageSizeMax > 0 && pageSize > s.cfg.PageSizeMax {
			pageSize = s.cfg.PageSizeMax
		}
	}

	entries, truncated, err := listFilesWithMetadata(ctx, s.backend, project, input.Path)
	if err != nil {
		return ListFilesOutput{}, fmt.Errorf("list files: %w", err)
	}

	filtered := make([]opengrok.FileEntry, 0, len(entries))
	for _, entry := range entries {
		if input.Path == "" || strings.HasPrefix(entry.Path, input.Path+"/") || entry.Path == input.Path {
			filtered = append(filtered, entry)
		}
	}

	if input.Kind != nil {
		kind := *input.Kind
		switch kind {
		case "file":
			fileFiltered := make([]opengrok.FileEntry, 0, len(filtered))
			for _, entry := range filtered {
				if !entry.IsDirectory {
					fileFiltered = append(fileFiltered, entry)
				}
			}
			filtered = fileFiltered
		case "directory":
			dirFiltered := make([]opengrok.FileEntry, 0, len(filtered))
			for _, entry := range filtered {
				if entry.IsDirectory {
					dirFiltered = append(dirFiltered, entry)
				}
			}
			filtered = dirFiltered
		case "both":
			// keep all
		default:
			return ListFilesOutput{}, &Error{
				Code:    "INVALID_KIND",
				Message: fmt.Sprintf("Invalid kind %q; valid values: file, directory, both", kind),
			}
		}
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := min(offset+pageSize, total)
	page := filtered[offset:end]

	includeLinks := s.includeLinks(input.IncludeLinks)

	files := make([]FileItem, 0, len(page))
	for _, entry := range page {
		fileLinks := s.links.File(project, entry.Path, 0)
		item := FileItem{
			Project:     project,
			Path:        entry.Path,
			Name:        path.Base(entry.Path),
			IsDirectory: entry.IsDirectory,
			NumLines:    entry.NumLines,
			Loc:         entry.Loc,
			Size:        entry.Size,
			Description: entry.Description,
			ResourceURI: fileLinks.ResourceURI,
		}
		if includeLinks {
			item.DisplayURL = fileLinks.DisplayURL
		}
		files = append(files, item)
	}

	var nextCursor *string
	if offset+pageSize < total {
		encoded, err := cursor.EncodeFileList(cursor.FileListState{
			Project:  project,
			Path:     input.Path,
			Offset:   offset + pageSize,
			PageSize: pageSize,
		})
		if err != nil {
			return ListFilesOutput{}, fmt.Errorf("file list cursor: %w", err)
		}
		nextCursor = &encoded
	}

	var warning *string
	if truncated {
		value := "OpenGrok file listing was truncated at 5,000 entries; total_hits and available pages are incomplete."
		warning = &value
	}

	return ListFilesOutput{
		Project:    project,
		Path:       input.Path,
		Files:      files,
		Pagination: newPagination(offset, pageSize, total, nextCursor),
		Truncated:  truncated,
		Warning:    warning,
	}, nil
}

func (s *Service) GetProjectOverview(ctx context.Context, input ProjectOverviewInput) (ProjectOverviewOutput, error) {
	projects, err := s.resolveProjects(input.Project, nil, false)
	if err != nil {
		return ProjectOverviewOutput{}, err
	}
	project := projects[0]

	entries, truncated, err := listFilesWithMetadata(ctx, s.backend, project, "")
	if err != nil {
		return ProjectOverviewOutput{}, fmt.Errorf("project overview: %w", err)
	}

	var totalFiles, totalDirs, totalLines int
	type langAgg struct {
		files int
		lines int
	}
	langStats := make(map[string]langAgg)
	projectPrefix := project + "/"

	var topFilesEntries []opengrok.FileEntry
	var topDirsEntries []opengrok.FileEntry

	for _, entry := range entries {
		rel := strings.TrimPrefix(entry.Path, projectPrefix)
		if entry.IsDirectory {
			totalDirs++
			if !strings.Contains(rel, "/") {
				topDirsEntries = append(topDirsEntries, entry)
			}
		} else {
			totalFiles++
			totalLines += entry.NumLines
			if !strings.Contains(rel, "/") {
				topFilesEntries = append(topFilesEntries, entry)
			}

			lang := detectLanguage(entry.Path)
			agg := langStats[lang]
			agg.files++
			agg.lines += entry.NumLines
			langStats[lang] = agg
		}
	}

	languages := make([]LanguageStat, 0, len(langStats))
	for lang, agg := range langStats {
		var percent float64
		if totalLines > 0 {
			percent = float64(agg.lines) / float64(totalLines) * 100
		}
		languages = append(languages, LanguageStat{
			Language: lang,
			Files:    agg.files,
			Lines:    agg.lines,
			Percent:  percent,
		})
	}

	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Percent > languages[j].Percent
	})

	topFiles := make([]FileItem, 0, len(topFilesEntries))
	for _, f := range topFilesEntries {
		topFiles = append(topFiles, FileItem{
			Project:     project,
			Path:        f.Path,
			Name:        path.Base(f.Path),
			IsDirectory: f.IsDirectory,
			NumLines:    f.NumLines,
			Loc:         f.Loc,
			Size:        f.Size,
			Description: f.Description,
			ResourceURI: s.links.File(project, f.Path, 0).ResourceURI,
		})
	}

	topDirs := make([]FileItem, 0, len(topDirsEntries))
	for _, d := range topDirsEntries {
		topDirs = append(topDirs, FileItem{
			Project:     project,
			Path:        d.Path,
			Name:        path.Base(d.Path),
			IsDirectory: true,
			NumLines:    d.NumLines,
			Loc:         d.Loc,
			Size:        d.Size,
			Description: d.Description,
			ResourceURI: s.links.File(project, d.Path, 0).ResourceURI,
		})
	}

	var warning *string
	if truncated {
		value := "OpenGrok file listing was truncated at 5,000 entries; project overview counts and language statistics are incomplete."
		warning = &value
	}

	return ProjectOverviewOutput{
		Project:     project,
		TotalFiles:  totalFiles,
		TotalDirs:   totalDirs,
		TopDirs:     topDirs,
		TopFiles:    topFiles,
		Description: fmt.Sprintf("Project %s overview", project),
		Truncated:   truncated,
		Warning:     warning,
		Languages:   languages,
	}, nil
}

func detectLanguage(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	base := strings.ToLower(path.Base(filePath))

	switch ext {
	case ".go":
		return "Go"
	case ".java":
		return "Java"
	case ".py":
		return "Python"
	case ".js", ".ts", ".tsx":
		return "JavaScript/TypeScript"
	case ".c", ".h":
		return "C"
	case ".cpp", ".cc", ".hpp":
		return "C++"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".swift":
		return "Swift"
	case ".kt":
		return "Kotlin"
	case ".scala":
		return "Scala"
	case ".md", ".markdown":
		return "Markdown"
	case ".yaml", ".yml":
		return "YAML"
	case ".json":
		return "JSON"
	case ".xml":
		return "XML"
	case ".sh", ".bash":
		return "Shell"
	case ".html", ".htm":
		return "HTML"
	case ".css", ".scss", ".less":
		return "CSS"
	case ".sql":
		return "SQL"
	case ".proto":
		return "Protobuf"
	case ".dockerfile":
		return "Dockerfile"
	case ".tf":
		return "Terraform"
	case ".makefile", ".mk":
		return "Makefile"
	default:
		if base == "dockerfile" {
			return "Dockerfile"
		}
		if base == "makefile" {
			return "Makefile"
		}
		return "Other"
	}
}

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

func (s *Service) CompactCompound(ctx context.Context, input CompactCompoundInput) (any, error) {
	switch input.Operation {
	case "search_and_read":
		if !s.cfg.Capabilities.SearchCode || !s.cfg.Capabilities.GetFileContext {
			return nil, unknownOperationError(input.Operation, compactCompoundOperations(s.cfg))
		}
		var payload SearchAndReadInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact compound search_and_read payload: %w", err)
		}
		return s.SearchAndRead(ctx, payload)
	case "find_symbol_and_references":
		if !s.cfg.Capabilities.SearchSymbolDefinitions || !s.cfg.Capabilities.SearchSymbolReferences || !s.cfg.Capabilities.GetFileContext {
			return nil, unknownOperationError(input.Operation, compactCompoundOperations(s.cfg))
		}
		var payload FindSymbolAndReferencesInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact compound find_symbol_and_references payload: %w", err)
		}
		return s.FindSymbolAndReferences(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactCompoundOperations(s.cfg))
	}
}

func compactCompoundOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.SearchCode {
		operations = append(operations, "search_and_read")
	}
	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "find_symbol_and_references")
	}
	return operations
}

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

func (s *Service) CompactSearch(ctx context.Context, input CompactSearchInput) (SearchOutput, error) {
	switch input.Operation {
	case "code":
		if !s.cfg.Capabilities.SearchCode {
			return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		var payload SearchCodeInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return SearchOutput{}, fmt.Errorf("decode compact search code payload: %w", err)
		}
		return s.SearchCode(ctx, payload)
	case "definitions":
		if !s.cfg.Capabilities.SearchSymbolDefinitions {
			return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		var payload SymbolSearchInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return SearchOutput{}, fmt.Errorf("decode compact search definitions payload: %w", err)
		}
		return s.SearchSymbolDefinitions(ctx, payload)
	case "references":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		var payload SymbolSearchInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return SearchOutput{}, fmt.Errorf("decode compact search references payload: %w", err)
		}
		return s.SearchSymbolReferences(ctx, payload)
	default:
		return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
	}
}

func (s *Service) CompactSymbols(ctx context.Context, input CompactSymbolsInput) (any, error) {
	switch input.Operation {
	case "list":
		if !s.cfg.Capabilities.ListSymbols {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		var payload ListSymbolsInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact symbols list payload: %w", err)
		}
		return s.ListSymbols(ctx, payload)
	case "implementations":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		var payload ImplementationSearchInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact symbols implementations payload: %w", err)
		}
		return s.SearchImplementations(ctx, payload)
	case "cross_project_references":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		var payload CrossProjectReferencesInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact symbols cross_project_references payload: %w", err)
		}
		return s.SearchCrossProjectReferences(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
	}
}

func (s *Service) CompactRead(ctx context.Context, input CompactReadInput) (FileContextOutput, error) {
	if input.Operation != "file" && input.Operation != "context" {
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}
	if !s.cfg.Capabilities.GetFileContext {
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}

	var payload FileContextInput
	if err := json.Unmarshal(input.Payload, &payload); err != nil {
		return FileContextOutput{}, fmt.Errorf("decode compact read %s payload: %w", input.Operation, err)
	}
	return s.GetFileContext(ctx, payload)
}

func compactSearchOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.SearchCode {
		operations = append(operations, "code")
	}
	if cfg.Capabilities.SearchSymbolDefinitions {
		operations = append(operations, "definitions")
	}
	if cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "references")
	}
	return operations
}

func compactSymbolsOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.ListSymbols {
		operations = append(operations, "list")
	}
	if cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "implementations")
		operations = append(operations, "cross_project_references")
	}
	return operations
}

func compactReadOperations(cfg config.Config) []string {
	if !cfg.Capabilities.GetFileContext {
		return []string{}
	}
	return []string{"file", "context"}
}

func unknownOperationError(operation string, enabled []string) error {
	return &Error{
		Code:    codeUnknownOperation,
		Message: fmt.Sprintf("Unknown operation %q; enabled operations: %s.", operation, strings.Join(enabled, ", ")),
	}
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

func NewMCPServer(cfg config.Config, backend Backend, version string) *mcp.Server {
	service := NewService(cfg, backend)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "opengrok-go-mcp",
		Version: version,
	}, nil)

	coercer := &scalarCoercer{}

	switch cfg.ToolSurface {
	case config.ToolSurfaceCompact:
		registerCompactTools(server, coercer, service, cfg)
		registerResources(server, service, cfg)
	case config.ToolSurfaceGateway:
		registerGatewayTools(server, coercer, service, cfg)
		registerResources(server, service, cfg)
	default:
		registerFullTools(server, coercer, service, cfg)
		registerResources(server, service, cfg)
	}

	// Coerce string-encoded booleans (e.g. include_links:"true") before the SDK
	// validates tool arguments, tolerating clients that serialize scalars as
	// strings.
	server.AddReceivingMiddleware(coercer.middleware())

	return server
}

func registerFullTools(server *mcp.Server, coercer *scalarCoercer, service *Service, cfg config.Config) {
	readOnlyAnnotations := &mcp.ToolAnnotations{ReadOnlyHint: true}

	if cfg.Capabilities.ListProjects {
		addTool(server, coercer, &mcp.Tool{
			Name:        "list_projects",
			Description: "List indexed OpenGrok projects. Results are paginated (50 per page); pass next_cursor to retrieve subsequent pages. total_projects is always returned so agents know the full count.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
			output, err := service.ListProjects(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchCode {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_code",
			Description: "Search reference/base code in OpenGrok (Apache Lucene backend). Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. Use mode full_text, path, history, definition, or reference; for file-name searches use mode=path.\n\nQUERY SYNTAX — wrap multi-word queries in quotes for exact-phrase matching. Unquoted `extends PaymentProcessor` tokenises into independent terms and returns 1000+ noisy hits; quoted \"extends PaymentProcessor\" returns ~7 exact hits. This server AUTO-QUOTES bare multi-word queries by default and notes it in the response warning; pass tokenized:true to search the words as independent terms instead. Use path_exclude to drop matches under a path (e.g. path_exclude=test) and path_prefix to restrict to a path.\n\nInline Lucene syntax also works in the query string: -path:legacy (exclude), +path:domain (require), defs:ClassName (symbol definition), refs:ClassName (symbol reference), hist:bugfix (commit messages, history mode), date:[20230101 TO 20261231] (history mode only). GOTCHAS: date: only works in history mode — used elsewhere it is ignored, but the response warning flags it. Wildcards (* ?) cannot be used inside quoted phrases (this silently matches nothing).\n\nUse returned file_path/project with read_file instead of fetching display_url/raw_url yourself. When answering about a specific file or class, include the selected result's citation.url.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchCodeInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchCode(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_and_read",
			Description: "Search OpenGrok and read the file content around each match in a single call, reducing round trips. Uses the same query interface as search_code.\n\nQUERY SYNTAX — wrap multi-word queries in quotes (\"extends PaymentProcessor\", not `extends PaymentProcessor`); bare multi-word queries are auto-quoted by default, pass tokenized:true to opt out. Inline Lucene syntax works: -path:legacy, +path:domain, defs:ClassName. Use path_exclude to drop matches under a path. date: only works in history mode (ignored elsewhere, but flagged in the response warning); wildcards cannot be used inside quoted phrases.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchAndReadInput) (*mcp.CallToolResult, SearchAndReadOutput, error) {
			output, err := service.SearchAndRead(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchSymbolDefinitions {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_symbol_definitions",
			Description: "Search symbol definitions in reference/base OpenGrok code. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. Use returned file_path/project with read_file to read the matched file; do not use WebFetch for display_url/raw_url because browser URLs may require auth. When answering about a class/interface, include citation.url for the definition.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SymbolSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchSymbolDefinitions(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_symbol_references",
			Description: "Search symbol references in reference/base OpenGrok code. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. Use returned file_path/project with read_file to read the matched file; avoid calling this for broad symbols unless you need many references. If discussing a specific reference, include citation.url.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SymbolSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchSymbolReferences(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext {
		addTool(server, coercer, &mcp.Tool{
			Name:        "find_symbol_and_references",
			Description: "Find a symbol's definition and all its references in a single call. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Returns the definition with surrounding context plus a paginated reference list.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input FindSymbolAndReferencesInput) (*mcp.CallToolResult, FindSymbolAndReferencesOutput, error) {
			output, err := service.FindSymbolAndReferences(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.GetFileContext {
		readFile := func(ctx context.Context, req *mcp.CallToolRequest, input FileContextInput) (*mcp.CallToolResult, FileContextOutput, error) {
			output, err := service.GetFileContext(ctx, input)
			return nil, output, err
		}
		addTool(server, coercer, &mcp.Tool{
			Name:        "get_file_context",
			Description: "Read a line window around a specific line in an OpenGrok file. Pass line_number (e.g. from a search result) to center the window and before/after to size it; if you omit line_number the whole file is returned, but prefer read_file for that. Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. When answering the user about this file, include citation.url.",
			Annotations: readOnlyAnnotations,
		}, readFile)
		addTool(server, coercer, &mcp.Tool{
			Name:        "read_file",
			Description: "Read full file content from OpenGrok. Returns up to 500 lines per call; if truncated is true, pass next_cursor to read the next section. total_lines is always returned. Use project and file_path from search results; omit project otherwise unless the user explicitly names one. Do not use WebFetch on display_url/raw_url; this tool sends configured auth and falls back to /raw. For a targeted line window use get_file_context with line_number. When summarizing a class or file, include citation.url in the final answer.",
			Annotations: readOnlyAnnotations,
		}, readFile)
	}

	if cfg.Capabilities.ListSymbols {
		addTool(server, coercer, &mcp.Tool{
			Name:        "list_symbols",
			Description: "List symbol definitions in OpenGrok, optionally filtered by ctags kind (class, interface, function, method, etc.) and scoped to a path. Use this for structural, architect-oriented queries: \"what classes exist in this package?\", \"find all interfaces under src/api/\". Combine path_prefix and kind for precise structural inventory. For broad sweeps across a large codebase, set include_snippets=false to reduce token cost — the warning field will tell you if the result set is large and how many additional calls full enumeration would require. Results are lean — use read_file or get_file_context to drill into a specific symbol. Omit project unless the user explicitly names one.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListSymbolsInput) (*mcp.CallToolResult, ListSymbolsOutput, error) {
			output, err := service.ListSymbols(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.ListFiles {
		addTool(server, coercer, &mcp.Tool{
			Name:        "list_files",
			Description: "List files in an OpenGrok project directory. Results are paginated; use page_size to control page size and next_cursor for subsequent pages.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListFilesInput) (*mcp.CallToolResult, ListFilesOutput, error) {
			output, err := service.ListFiles(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.ListFiles {
		addTool(server, coercer, &mcp.Tool{
			Name:        "get_project_overview",
			Description: "Get a high-level overview of an OpenGrok project: total file/directory counts, a per-language breakdown (files, lines, and percent per language), and top-level directory and file entries. Use this to answer questions like \"what languages does this project use?\".",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ProjectOverviewInput) (*mcp.CallToolResult, ProjectOverviewOutput, error) {
			output, err := service.GetProjectOverview(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_implementations",
			Description: "Search candidate implementations and usages of a symbol. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Delegates to symbol-reference search; results are best-effort since OpenGrok does not provide language-semantic implementation mapping.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ImplementationSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchImplementations(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "search_cross_project_references",
			Description: "Search for references to a symbol across multiple projects, grouped by project for cross-project analysis. Pass a bare symbol name (e.g. PaymentProcessor), not quoted.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CrossProjectReferencesInput) (*mcp.CallToolResult, CrossProjectReferencesOutput, error) {
			output, err := service.SearchCrossProjectReferences(ctx, input)
			return nil, output, err
		})
	}

	if memoryToolsEnabled(cfg) {
		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_set",
			Description: "Store a key-value pair in the server's memory bank. Values persist for the lifetime of the server process.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemorySetInput) (*mcp.CallToolResult, MemorySetOutput, error) {
			output, err := service.MemorySet(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_get",
			Description: "Retrieve a value from the memory bank by key.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryGetInput) (*mcp.CallToolResult, MemoryGetOutput, error) {
			output, err := service.MemoryGet(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_list",
			Description: "List all entries in the memory bank.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryListInput) (*mcp.CallToolResult, MemoryListOutput, error) {
			output, err := service.MemoryList(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_delete",
			Description: "Delete a key from the memory bank.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryDeleteInput) (*mcp.CallToolResult, MemoryDeleteOutput, error) {
			output, err := service.MemoryDelete(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_clear",
			Description: "Clear all entries from the memory bank.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryClearInput) (*mcp.CallToolResult, MemoryClearOutput, error) {
			output, err := service.MemoryClear(ctx, input)
			return nil, output, err
		})
	}
}

// compactInputSchema returns an explicit input schema for compact wrapper
// tools. Their nested payload field is typed as json.RawMessage; reflection-based
// schema inference would otherwise publish it as a byte array ([]byte), so an
// object-shaped payload fails validation before the handler runs. operation is
// the only required field — some operations (e.g. memory list/clear) take no
// payload.
func compactInputSchema(operationDescription string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": operationDescription,
			},
			"payload": map[string]any{
				"type":        "object",
				"description": "The selected operation's input object.",
			},
		},
		"required": []any{"operation"},
	}
}

func registerCompactTools(server *mcp.Server, coercer *scalarCoercer, service *Service, cfg config.Config) {
	if cfg.Capabilities.ListProjects {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_projects",
			Description: "List indexed OpenGrok projects. Results are paginated; pass next_cursor to retrieve subsequent pages.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
			output, err := service.ListProjects(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.SearchCode || cfg.Capabilities.SearchSymbolDefinitions || cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_search",
			Description: "Search OpenGrok code and symbols. operation=code searches text/path/history/definition/reference; operation=definitions finds symbol definitions; operation=references finds symbol references. Payload is the selected operation's input object.\n\nFor operation=code, the payload query field follows Lucene syntax. Wrap multi-word queries in quotes: \"extends PaymentProcessor\" returns ~7 exact results; unquoted `extends PaymentProcessor` returns 1000+ tokenised hits. Bare multi-word queries are auto-quoted by default; set payload.tokenized=true to search words as independent terms. Payload fields: query, mode, path_prefix, path_exclude, file_type, tokenized. Inline field syntax also works: -path:legacy, +path:domain, defs:ClassName, date:[20230101 TO 20261231] (history mode only; ignored elsewhere and flagged in the response warning).",
			InputSchema: compactInputSchema("one of: code, definitions, references"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.CompactSearch(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.ListSymbols || cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_symbols",
			Description: "Work with OpenGrok symbols. operation=list lists symbols (requires list_symbols capability); operation=implementations finds candidate implementations of a symbol; operation=cross_project_references finds references across projects. Each operation payload matches the corresponding full tool input.",
			InputSchema: compactInputSchema("one of: list, implementations, cross_project_references"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactSymbolsInput) (*mcp.CallToolResult, any, error) {
			output, err := service.CompactSymbols(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.GetFileContext {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_read",
			Description: "Read OpenGrok files or line windows. operation=file and operation=context both use a file-context payload.",
			InputSchema: compactInputSchema("one of: file, context"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactReadInput) (*mcp.CallToolResult, FileContextOutput, error) {
			output, err := service.CompactRead(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.GetFileContext &&
		(cfg.Capabilities.SearchCode || (cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences)) {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_compound",
			Description: "Compound OpenGrok operations. operation=search_and_read searches and reads file content around matches; operation=find_symbol_and_references finds a symbol's definition and references. Each operation payload matches the corresponding full tool input.\n\nFor operation=search_and_read, the query field follows the same Lucene syntax as opengrok_search: wrap multi-word queries in quotes (\"extends PaymentProcessor\"); bare multi-word queries are auto-quoted by default, set payload.tokenized=true to opt out. Use -path:legacy / +path:domain / defs:ClassName inline, or the path_exclude payload field. date: only works in history mode (ignored elsewhere, but flagged in the response warning).",
			InputSchema: compactInputSchema("one of: search_and_read, find_symbol_and_references"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactCompoundInput) (*mcp.CallToolResult, any, error) {
			output, err := service.CompactCompound(ctx, input)
			return nil, output, err
		})
	}

	if memoryToolsEnabled(cfg) {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_memory",
			Description: "Interact with the server's process-scoped memory bank. Available only for stdio servers with memory enabled.",
			InputSchema: compactInputSchema("one of: set, get, list, delete, clear"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactMemoryInput) (*mcp.CallToolResult, any, error) {
			output, err := service.CompactMemory(ctx, input)
			return nil, output, err
		})
	}
}

func buildGatewayRegistry(service *Service, cfg config.Config) map[string]gatewayOperation {
	registry := make(map[string]gatewayOperation)

	if cfg.Capabilities.ListProjects {
		registry["projects.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "projects.list",
				Description: "List indexed OpenGrok projects.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ListProjectsInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.ListProjects(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchCode {
		registry["search.code"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.code",
				Description: "Search code in OpenGrok (full-text, path, history, definition, or reference).",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SearchCodeInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchCode(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolDefinitions {
		registry["search.definitions"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.definitions",
				Description: "Search symbol definitions in OpenGrok.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SymbolSearchInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchSymbolDefinitions(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolReferences {
		registry["search.references"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.references",
				Description: "Search symbol references in OpenGrok.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SymbolSearchInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchSymbolReferences(ctx, input)
			},
		}
	}

	if cfg.Capabilities.ListSymbols {
		registry["symbols.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "symbols.list",
				Description: "List symbol definitions in OpenGrok, optionally filtered by kind and path.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ListSymbolsInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.ListSymbols(ctx, input)
			},
		}
	}

	if cfg.Capabilities.ListFiles {
		registry["files.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "files.list",
				Description: "List files in an OpenGrok project directory.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ListFilesInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.ListFiles(ctx, input)
			},
		}

		registry["project.overview"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "project.overview",
				Description: "Get project overview with file and directory counts.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ProjectOverviewInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.GetProjectOverview(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolReferences {
		registry["search.implementations"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.implementations",
				Description: "Search candidate implementations and usages of a symbol.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ImplementationSearchInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchImplementations(ctx, input)
			},
		}

		registry["search.cross_project_references"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.cross_project_references",
				Description: "Search symbol references across multiple projects.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input CrossProjectReferencesInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchCrossProjectReferences(ctx, input)
			},
		}
	}

	if cfg.Capabilities.GetFileContext {
		registry["file.read"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "file.read",
				Description: "Read full file content from OpenGrok.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input FileContextInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.GetFileContext(ctx, input)
			},
		}

		registry["file.context"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "file.context",
				Description: "Read a line window around a specific line number in a file.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input FileContextInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.GetFileContext(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext {
		registry["compound.search_and_read"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "compound.search_and_read",
				Description: "Search OpenGrok and read the file content around each match in a single call.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SearchAndReadInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchAndRead(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext {
		registry["compound.find_symbol_and_references"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "compound.find_symbol_and_references",
				Description: "Find a symbol's definition and all its references in a single call.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input FindSymbolAndReferencesInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.FindSymbolAndReferences(ctx, input)
			},
		}
	}

	if memoryToolsEnabled(cfg) {
		registry["memory.set"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.set",
				Description: "Store a key-value pair in the server's memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input MemorySetInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.MemorySet(ctx, input)
			},
		}

		registry["memory.get"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.get",
				Description: "Retrieve a value from the memory bank by key.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input MemoryGetInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.MemoryGet(ctx, input)
			},
		}

		registry["memory.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.list",
				Description: "List all entries in the memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				return service.MemoryList(ctx, MemoryListInput{})
			},
		}

		registry["memory.delete"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.delete",
				Description: "Delete a key from the memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input MemoryDeleteInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.MemoryDelete(ctx, input)
			},
		}

		registry["memory.clear"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.clear",
				Description: "Clear all entries from the memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				return service.MemoryClear(ctx, MemoryClearInput{})
			},
		}
	}

	return registry
}

func memoryToolsEnabled(cfg config.Config) bool {
	return cfg.Capabilities.Memory && cfg.Transport != config.TransportHTTP
}

func registerGatewayTools(server *mcp.Server, coercer *scalarCoercer, service *Service, cfg config.Config) {
	registry := buildGatewayRegistry(service, cfg)

	addTool(server, coercer, &mcp.Tool{
		Name:        "opengrok_discover",
		Description: "List available gateway operations for OpenGrok. Returns the full operation manifest with names and descriptions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GatewayDiscoverInput) (*mcp.CallToolResult, GatewayDiscoverOutput, error) {
		operations := make([]GatewayOperation, 0, len(registry))
		names := make([]string, 0, len(registry))
		for name := range registry {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			operations = append(operations, registry[name].Manifest)
		}
		return nil, GatewayDiscoverOutput{Operations: operations}, nil
	})

	addTool(server, coercer, &mcp.Tool{
		Name:        "opengrok_call",
		Description: "Call an OpenGrok gateway operation. Use opengrok_discover to list available operations and their payload schemas.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{"type": "string"},
				"payload":   map[string]any{}, // any valid JSON
			},
			"required": []any{"operation"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GatewayCallInput) (*mcp.CallToolResult, GatewayCallOutput, error) {
		op, ok := registry[input.Operation]
		if !ok {
			enabledOps := make([]string, 0, len(registry))
			for name := range registry {
				enabledOps = append(enabledOps, name)
			}
			sort.Strings(enabledOps)
			return nil, GatewayCallOutput{}, &Error{
				Code:    codeUnknownOperation,
				Message: fmt.Sprintf("unknown operation %q; enabled operations: %v", input.Operation, enabledOps),
			}
		}
		result, err := op.Call(ctx, input.Payload)
		if err != nil {
			return nil, GatewayCallOutput{}, err
		}
		return nil, GatewayCallOutput{
			Operation: input.Operation,
			Result:    result,
		}, nil
	})
}

func registerResources(server *mcp.Server, service *Service, cfg config.Config) {
	if cfg.Capabilities.ListProjects {
		server.AddResource(&mcp.Resource{
			URI:         "opengrok://projects",
			Name:        "projects",
			Title:       "OpenGrok projects",
			Description: "Indexed OpenGrok projects.",
			MIMEType:    "application/json",
		}, service.projectsResource)
		server.AddResourceTemplate(&mcp.ResourceTemplate{
			URITemplate: "opengrok://project/{project}",
			Name:        "project",
			Title:       "OpenGrok project",
			Description: "OpenGrok project metadata.",
			MIMEType:    "application/json",
		}, service.projectResource)
	}
	if cfg.Capabilities.GetFileContext {
		server.AddResourceTemplate(&mcp.ResourceTemplate{
			URITemplate: "opengrok://project/{project}/files/{+path}",
			Name:        "file",
			Title:       "OpenGrok file",
			Description: "OpenGrok file contents.",
			MIMEType:    "application/json",
		}, service.fileResource)
	}
}

func cursorValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

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

func validateResponseMode(responseMode string) error {
	if responseMode == "" || responseMode == "full" || responseMode == "compact" {
		return nil
	}

	return &Error{
		Code:    codeInvalidResponseMode,
		Message: fmt.Sprintf("Invalid response_mode %q; valid values: full, compact", responseMode),
	}
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
		return emptySearchOutput(req.mode, req.query), fmt.Errorf("search: %w", err)
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

	var warning *string
	if req.autoQuoted {
		warning = appendWarning(warning, "Auto-quoted multi-word query for exact-phrase match. Pass tokenized:true to search the words as independent terms.")
	}
	if req.userQuery != "" && req.mode != string(opengrok.ModeHistory) && queryHasDateField(req.userQuery) {
		warning = appendWarning(warning, "date: is only valid in history mode and was ignored in this search.")
	}
	if result.TotalHits > searchWarnThreshold {
		msg := fmt.Sprintf("Query returned %d hits. Consider narrowing with path_prefix, file_type, or a more specific query.", result.TotalHits)
		if req.userQuery != "" && !req.autoQuoted && isMultiWord(req.userQuery) {
			msg += fmt.Sprintf(" For an exact phrase, wrap it in quotes: %q.", req.userQuery)
		}
		warning = appendWarning(warning, msg)
	}

	results := s.results(result.Hits, project, req.mode, req.symbol, s.includeLinks(req.includeLinks))

	totalHits := result.TotalHits
	if req.maxHitsPerFile > 0 {
		results = applyMaxHitsPerFile(results, req.maxHitsPerFile)
	}

	sortedResults, sortWarning, sortErr := applySort(results, req.sort)
	if sortErr != nil {
		return emptySearchOutput(req.mode, req.query), sortErr
	}
	warning = appendWarning(warning, sortWarning)
	results = sortedResults

	var expansion *ExpansionDiagnostics
	if req.responseMode != "compact" {
		results, expansion = s.maybeExpandResults(ctx, results, req.expandContext, budget)
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
		Project:    project,
		Mode:       req.mode,
		Query:      req.query,
		Pagination: newPagination(offset, pageSize, totalHits, nextCursor),
		Results:    results,
		Warning:    warning,
		Diagnostics: Diagnostics{
			OffsetUsed:         offset,
			OpenGrokStart:      result.Start,
			OpenGrokMaxResults: pageSize,
		},
		Expansion: expansion,
	}, nil
}

func compactResults(results []Result) []Result {
	for i := range results {
		results[i].ColumnNumber = nil
		results[i].DisplayTitle = ""
		results[i].DisplayURL = ""
		results[i].RawURL = nil
		results[i].Score = nil
		results[i].Context = nil
		results[i].Metadata = nil
	}
	return results
}

func applyMaxHitsPerFile(results []Result, maxHitsPerFile int) []Result {
	fileCounts := make(map[string]int)
	filtered := make([]Result, 0, len(results))
	for _, result := range results {
		key := result.Project + "\x00" + result.FilePath
		if fileCounts[key] < maxHitsPerFile {
			filtered = append(filtered, result)
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
			Code:    "INVALID_SORT",
			Message: fmt.Sprintf("Invalid sort order %q; valid values: relevance, path, date", sortOrder),
		}
	}
}

func (s *Service) maybeExpandResults(ctx context.Context, results []Result, expand bool, budget config.BudgetValues) ([]Result, *ExpansionDiagnostics) {
	if !expand {
		return results, nil
	}
	return s.expandResultContextsWithDiagnostics(ctx, results, budget)
}

func (s *Service) resolveSearchProjects(project string, projects []string, allowAllProjects bool) ([]string, error) {
	if allowAllProjects && project == "" && len(projects) == 0 {
		return []string{}, nil
	}

	resolved, err := s.resolveProjects(project, projects, allowAllProjects)
	if err == nil {
		return resolved, nil
	}
	if s.cfg.ProjectRequired {
		return nil, err
	}

	return []string{}, nil
}

func (s *Service) resolveProjects(project string, projects []string, allowAllProjects bool) ([]string, error) {
	switch {
	case project != "":
		if err := s.validateConfiguredProjects([]string{project}); err != nil {
			return nil, err
		}
		return []string{project}, nil
	case len(projects) > 0:
		resolved := make([]string, len(projects))
		copy(resolved, projects)
		if err := s.validateConfiguredProjects(resolved); err != nil {
			return nil, err
		}
		return resolved, nil
	case s.cfg.DefaultProject != "":
		return []string{s.cfg.DefaultProject}, nil
	default:
		return nil, &Error{
			Code:    codeProjectRequired,
			Message: "No project selected. Pass project or call list_projects first.",
		}
	}
}

func (s *Service) validateConfiguredProjects(projects []string) error {
	if len(s.cfg.Projects) == 0 {
		return nil
	}

	allowed := map[string]bool{}
	for _, project := range s.cfg.Projects {
		allowed[project] = true
	}
	for _, project := range projects {
		if allowed[project] {
			continue
		}
		return &Error{
			Code: codeUnknownProject,
			Message: fmt.Sprintf(
				"Unknown OpenGrok project %q. Resolved OpenGrok projects (source=%s): %s. Omit project to use the default project %q.",
				project,
				projectSourceLabel(s.cfg.ProjectSource),
				strings.Join(s.cfg.Projects, ", "),
				s.cfg.DefaultProject,
			),
		}
	}

	return nil
}

func projectSourceLabel(source string) string {
	if source == "" {
		return config.ProjectSourceNone
	}
	return source
}

func firstProject(projects []string) string {
	if len(projects) == 0 {
		return ""
	}

	return projects[0]
}

func (s *Service) pageSize(requested int) int {
	size := requested
	if size == 0 {
		size = s.cfg.PageSizeDefault
	}
	if s.cfg.PageSizeMax > 0 && size > s.cfg.PageSizeMax {
		size = s.cfg.PageSizeMax
	}
	if size <= 0 {
		return 20
	}

	return size
}

func (s *Service) includeLinks(value *bool) bool {
	if value != nil {
		return *value
	}

	return s.cfg.IncludeLinksDefault
}

func (s *Service) shouldExpandContext(param *bool) bool {
	if param != nil {
		return *param
	}
	return s.cfg.AutoExpandContext
}

func (s *Service) resolveBudgetTier(budget string) (config.BudgetValues, error) {
	switch strings.ToLower(budget) {
	case "", config.ContextBudgetDefault:
		return config.BudgetValues{
			ContextBefore:      s.cfg.ContextBefore,
			ContextAfter:       s.cfg.ContextAfter,
			MaxExpandedResults: s.cfg.MaxExpandedResults,
			MaxExpandedFiles:   s.cfg.MaxExpandedFiles,
		}, nil
	case config.ContextBudgetMinimal:
		return s.cfg.BudgetTiers.Minimal, nil
	case config.ContextBudgetMaximal:
		return s.cfg.BudgetTiers.Maximal, nil
	default:
		return config.BudgetValues{}, &Error{
			Code: codeInvalidContextBudget,
			Message: fmt.Sprintf(
				"Invalid context_budget %q; valid values: %s, %s, %s",
				budget,
				config.ContextBudgetMinimal,
				config.ContextBudgetDefault,
				config.ContextBudgetMaximal,
			),
		}
	}
}

func (s *Service) nextCursor(state cursor.State, totalHits int) (*string, error) {
	if state.Offset >= totalHits {
		return nil, nil
	}

	value, err := cursor.Encode(state)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

func (s *Service) results(
	hits []opengrok.Hit,
	defaultProject string,
	mode string,
	symbol string,
	includeLinks bool,
) []Result {
	results := make([]Result, 0, len(hits))
	for _, hit := range hits {
		project := hit.Project
		if project == "" {
			project = defaultProject
		}
		fileLinks := s.links.File(project, hit.FilePath, hit.LineNumber)

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
			Citation:             citation(displayTitle(hit.FilePath, hit.LineNumber), fileLinks.DisplayURL, hit.LineNumber),
			ResourceURI:          fileLinks.ResourceURI,
			Score:                nil,
			Metadata:             map[string]any{},
		}
		if includeLinks {
			result.DisplayURL = fileLinks.DisplayURL
			result.RawURL = fileLinks.RawURL
		}

		results = append(results, result)
	}

	return results
}

func displayTitle(filePath string, lineNumber int) string {
	return path.Base(filePath) + ":" + strconv.Itoa(lineNumber)
}

func citation(title string, url string, line int) Citation {
	return Citation{
		Title: title,
		URL:   url,
		Line:  line,
	}
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
	expanded, _ := s.expandResultContextsWithDiagnostics(ctx, results, budget)
	return expanded
}

func (s *Service) expandResultContextsWithDiagnostics(ctx context.Context, results []Result, budget config.BudgetValues) ([]Result, *ExpansionDiagnostics) {
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

	for i, result := range results[:eligibleResults] {
		key := fileKey{project: result.Project, filePath: result.FilePath}
		content, ok := fileContents[key]
		if !ok {
			continue
		}
		window := extractWindow(content, result.LineNumber, budget.ContextBefore, budget.ContextAfter)
		if window.StartLine > 0 {
			results[i].Context = &window
			diagnostics.ExpandedResults++
		}
	}
	diagnostics.FetchedFiles = len(fileContents)
	diagnostics.SkippedResults = len(results) - diagnostics.ExpandedResults

	return results, diagnostics
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

func emptySearchOutput(mode string, query string) SearchOutput {
	return SearchOutput{
		Mode:    mode,
		Query:   query,
		Results: []Result{},
	}
}

func invalidCursorError() error {
	return &Error{
		Code:    codeInvalidCursor,
		Message: "Invalid cursor.",
	}
}

func (s *Service) projectsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	output, err := s.ListProjects(ctx, ListProjectsInput{})
	if err != nil {
		return nil, err
	}

	return jsonResource(req.Params.URI, output)
}

func (s *Service) projectResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	project, _, _, ok := parseProjectResourceURI(req.Params.URI)
	if !ok {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	output := ProjectItem{
		Project:     project,
		Title:       project,
		Description: "Indexed OpenGrok project",
		ProjectURL:  s.links.Search(project, defaultSearchMode, ""),
		ResourceURI: s.links.Project(project),
	}

	return jsonResource(req.Params.URI, output)
}

func (s *Service) fileResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	project, filePath, lineNumber, ok := parseProjectResourceURI(req.Params.URI)
	if !ok || filePath == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	output, err := s.GetFileContext(ctx, FileContextInput{
		Project:    project,
		FilePath:   filePath,
		LineNumber: lineNumber,
	})
	if err != nil {
		return nil, err
	}

	return jsonResource(req.Params.URI, output)
}

func jsonResource(uri string, value any) (*mcp.ReadResourceResult, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal resource: %w", err)
	}

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func parseProjectResourceURI(rawURI string) (string, string, int, bool) {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "opengrok" || parsed.Host != "project" {
		return "", "", 0, false
	}

	lineNumber, ok := parseLineFragment(parsed.Fragment)
	if !ok {
		return "", "", 0, false
	}

	rest := strings.TrimPrefix(parsed.EscapedPath(), "/")
	projectPart, filePart, hasFile := strings.Cut(rest, "/files/")
	project, err := url.PathUnescape(projectPart)
	if err != nil || project == "" {
		return "", "", 0, false
	}
	if !hasFile {
		return project, "", lineNumber, true
	}

	filePath, err := url.PathUnescape(filePart)
	if err != nil || filePath == "" {
		return "", "", 0, false
	}

	return project, filePath, lineNumber, true
}

func parseLineFragment(fragment string) (int, bool) {
	if fragment == "" {
		return 0, true
	}

	var value string
	switch {
	case strings.HasPrefix(fragment, "L"):
		value = strings.TrimPrefix(fragment, "L")
	case strings.HasPrefix(fragment, "line="):
		value = strings.TrimPrefix(fragment, "line=")
	default:
		return 0, true
	}

	lineNumber, err := strconv.Atoi(value)
	if err != nil || lineNumber <= 0 {
		return 0, false
	}

	return lineNumber, true
}
