// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

type fakeBackend struct {
	projects    []string
	projectsErr error

	searchRequests []opengrok.SearchRequest
	searchResult   opengrok.SearchResult
	searchErr      error

	fileContent   string
	fileContents  map[string]string // key: "project:filePath"
	fileErr       error
	fileErrors    map[string]error // key: "project:filePath"
	panicFileRead bool
	fileProject   string
	filePath      string
	fileCallCount int
	mu            sync.Mutex

	fileEntries       []opengrok.FileEntry
	fileListProject   string
	fileListPath      string
	fileListErr       error
	fileListTruncated bool

	projectOverview    opengrok.ProjectOverview
	projectOverviewErr error
}

func (b *fakeBackend) ListProjects(context.Context) ([]string, error) {
	if b.projectsErr != nil {
		return nil, b.projectsErr
	}
	return b.projects, nil
}

func (b *fakeBackend) Search(_ context.Context, req opengrok.SearchRequest) (opengrok.SearchResult, error) {
	b.searchRequests = append(b.searchRequests, req)
	if b.searchErr != nil {
		return opengrok.SearchResult{Hits: []opengrok.Hit{}}, b.searchErr
	}
	return b.searchResult, nil
}

func (b *fakeBackend) FileContent(_ context.Context, project string, filePath string) (string, error) {
	if b.panicFileRead {
		panic("file decoder panic")
	}
	b.mu.Lock()
	b.fileProject = project
	b.filePath = filePath
	b.fileCallCount++
	b.mu.Unlock()
	key := project + ":" + filePath
	if b.fileErrors != nil {
		if err, ok := b.fileErrors[key]; ok {
			return "", err
		}
	}
	if b.fileErr != nil {
		return "", b.fileErr
	}
	if b.fileContents != nil {
		if content, ok := b.fileContents[key]; ok {
			return content, nil
		}
	}
	return b.fileContent, nil
}

func (b *fakeBackend) ListFiles(_ context.Context, project string, path string) ([]opengrok.FileEntry, error) {
	b.fileListProject = project
	b.fileListPath = path
	if b.fileListErr != nil {
		return nil, b.fileListErr
	}
	return b.fileEntries, nil
}

func (b *fakeBackend) ListFilesWithMetadata(ctx context.Context, project string, path string) ([]opengrok.FileEntry, bool, error) {
	entries, err := b.ListFiles(ctx, project, path)
	return entries, b.fileListTruncated, err
}

func (b *fakeBackend) GetProjectOverview(_ context.Context, project string) (opengrok.ProjectOverview, error) {
	if b.projectOverviewErr != nil {
		return opengrok.ProjectOverview{}, b.projectOverviewErr
	}
	return b.projectOverview, nil
}

func connectMCPServer(t *testing.T, server *mcp.Server) (*mcp.ClientSession, func()) {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect returned error: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		serverSession.Close()
		t.Fatalf("client.Connect returned error: %v", err)
	}

	return clientSession, func() {
		clientSession.Close()
		serverSession.Close()
	}
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	cfg.DefaultProject = "platform"
	return cfg
}

func strPtr(s string) *string { return &s }
