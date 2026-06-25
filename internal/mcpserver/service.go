// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"errors"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
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

type Error struct {
	Code       string
	Message    string
	Suggestion string
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
