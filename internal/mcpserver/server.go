package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/links"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

const (
	codeInvalidCursor   = "INVALID_CURSOR"
	codeProjectRequired = "PROJECT_REQUIRED"

	defaultSearchMode = string(opengrok.ModeFullText)
)

type Backend interface {
	ListProjects(ctx context.Context) ([]string, error)
	Search(ctx context.Context, req opengrok.SearchRequest) (opengrok.SearchResult, error)
	FileContent(ctx context.Context, project string, filePath string) (string, error)
}

type Service struct {
	cfg     config.Config
	backend Backend
	links   links.Builder
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
		cfg:     cfg,
		backend: backend,
		links:   links.NewBuilder(cfg.OpenGrokWebBaseURL, cfg.EnableRawLinks),
	}
}

func (s *Service) ListProjects(ctx context.Context, input ListProjectsInput) (ListProjectsOutput, error) {
	_ = input.Cursor

	projects, err := s.backend.ListProjects(ctx)
	if err != nil {
		return ListProjectsOutput{Projects: []ProjectItem{}}, fmt.Errorf("list projects: %w", err)
	}

	items := make([]ProjectItem, 0, len(projects))
	for _, project := range projects {
		items = append(items, ProjectItem{
			Project:     project,
			Title:       project,
			Description: "Indexed OpenGrok project",
			ProjectURL:  s.links.Search(project, defaultSearchMode, ""),
			ResourceURI: s.links.Project(project),
		})
	}

	return ListProjectsOutput{
		Projects:   items,
		NextCursor: nil,
	}, nil
}

func (s *Service) SearchCode(ctx context.Context, input SearchCodeInput) (SearchOutput, error) {
	mode := input.Mode
	if mode == "" {
		mode = defaultSearchMode
	}

	return s.search(ctx, searchRequest{
		project:      input.Project,
		projects:     input.Projects,
		query:        input.Query,
		mode:         mode,
		pathPrefix:   input.PathPrefix,
		fileType:     input.FileType,
		pageSize:     input.PageSize,
		cursor:       input.Cursor,
		includeLinks: input.IncludeLinks,
	})
}

func (s *Service) SearchSymbolDefinitions(ctx context.Context, input SymbolSearchInput) (SearchOutput, error) {
	return s.search(ctx, searchRequest{
		project:      input.Project,
		projects:     input.Projects,
		query:        input.Symbol,
		mode:         string(opengrok.ModeDefinition),
		pageSize:     input.PageSize,
		cursor:       input.Cursor,
		includeLinks: input.IncludeLinks,
		symbol:       input.Symbol,
	})
}

func (s *Service) SearchSymbolReferences(ctx context.Context, input SymbolSearchInput) (SearchOutput, error) {
	return s.search(ctx, searchRequest{
		project:      input.Project,
		projects:     input.Projects,
		query:        input.Symbol,
		mode:         string(opengrok.ModeReference),
		pageSize:     input.PageSize,
		cursor:       input.Cursor,
		includeLinks: input.IncludeLinks,
		symbol:       input.Symbol,
	})
}

func (s *Service) GetFileContext(ctx context.Context, input FileContextInput) (FileContextOutput, error) {
	projects, err := s.resolveProjects(input.Project, nil)
	if err != nil {
		return FileContextOutput{}, err
	}

	content, err := s.backend.FileContent(ctx, projects[0], input.FilePath)
	if err != nil {
		return FileContextOutput{}, fmt.Errorf("file context: %w", err)
	}

	fileLinks := s.links.File(projects[0], input.FilePath, 0)
	output := FileContextOutput{
		Project:     projects[0],
		FilePath:    input.FilePath,
		Content:     content,
		DisplayURL:  fileLinks.DisplayURL,
		RawURL:      fileLinks.RawURL,
		ResourceURI: fileLinks.ResourceURI,
	}

	return output, nil
}

func NewMCPServer(cfg config.Config, backend Backend, version string) *mcp.Server {
	service := NewService(cfg, backend)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "opengrok-go-mcp",
		Version: version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_projects",
		Description: "List indexed OpenGrok projects.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
		output, err := service.ListProjects(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_code",
		Description: "Search OpenGrok code by text, path, history, definitions, or references.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchCodeInput) (*mcp.CallToolResult, SearchOutput, error) {
		output, err := service.SearchCode(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_symbol_definitions",
		Description: "Search OpenGrok symbol definitions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SymbolSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		output, err := service.SearchSymbolDefinitions(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_symbol_references",
		Description: "Search OpenGrok symbol references.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input SymbolSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
		output, err := service.SearchSymbolReferences(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_file_context",
		Description: "Read an OpenGrok file.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input FileContextInput) (*mcp.CallToolResult, FileContextOutput, error) {
		output, err := service.GetFileContext(ctx, input)
		return nil, output, err
	})

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
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "opengrok://project/{project}/files/{+path}",
		Name:        "file",
		Title:       "OpenGrok file",
		Description: "OpenGrok file contents.",
		MIMEType:    "application/json",
	}, service.fileResource)

	return server
}

type searchRequest struct {
	project      string
	projects     []string
	query        string
	mode         string
	pathPrefix   string
	fileType     string
	pageSize     int
	cursor       string
	includeLinks *bool
	symbol       string
}

func (s *Service) search(ctx context.Context, req searchRequest) (SearchOutput, error) {
	projects, err := s.resolveProjects(req.project, req.projects)
	if err != nil {
		return emptySearchOutput(req.mode, req.query), err
	}

	pageSize := s.pageSize(req.pageSize)
	offset := 0
	if req.cursor != "" {
		state, err := cursor.Decode(req.cursor)
		if err != nil {
			return emptySearchOutput(req.mode, req.query), invalidCursorError()
		}

		expected := cursor.State{
			Project:    projects[0],
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
		Project:    projects[0],
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

	return SearchOutput{
		Project:    projects[0],
		Mode:       req.mode,
		Query:      req.query,
		TotalHits:  result.TotalHits,
		Results:    s.results(result.Hits, projects[0], req.mode, req.symbol, s.includeLinks(req.includeLinks)),
		PageSize:   pageSize,
		NextCursor: nextCursor,
		Diagnostics: Diagnostics{
			OffsetUsed:         offset,
			OpenGrokStart:      result.Start,
			OpenGrokMaxResults: pageSize,
		},
	}, nil
}

func (s *Service) resolveProjects(project string, projects []string) ([]string, error) {
	switch {
	case project != "":
		return []string{project}, nil
	case len(projects) > 0:
		resolved := make([]string, len(projects))
		copy(resolved, projects)
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

		result := Result{
			ResultID:     project + ":" + hit.FilePath + ":" + strconv.Itoa(hit.LineNumber),
			Project:      project,
			FilePath:     hit.FilePath,
			LineNumber:   hit.LineNumber,
			ColumnNumber: nil,
			Kind:         mode,
			Symbol:       resultSymbol,
			Snippet:      hit.Snippet,
			DisplayTitle: displayTitle(hit.FilePath, hit.LineNumber),
			ResourceURI:  fileLinks.ResourceURI,
			Score:        nil,
			Metadata:     map[string]any{},
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
	project, _, ok := parseProjectResourceURI(req.Params.URI)
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
	project, filePath, ok := parseProjectResourceURI(req.Params.URI)
	if !ok || filePath == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	output, err := s.GetFileContext(ctx, FileContextInput{
		Project:  project,
		FilePath: filePath,
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

func parseProjectResourceURI(rawURI string) (string, string, bool) {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "opengrok" || parsed.Host != "project" {
		return "", "", false
	}

	rest := strings.TrimPrefix(parsed.EscapedPath(), "/")
	projectPart, filePart, hasFile := strings.Cut(rest, "/files/")
	project, err := url.PathUnescape(projectPart)
	if err != nil || project == "" {
		return "", "", false
	}
	if !hasFile {
		return project, "", true
	}

	filePath, err := url.PathUnescape(filePart)
	if err != nil || filePath == "" {
		return "", "", false
	}

	return project, filePath, true
}
