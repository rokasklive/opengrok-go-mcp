// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestListProjectsReturnsResourceURIs(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	if len(output.Projects) != 2 {
		t.Fatalf("projects length = %d, want 2", len(output.Projects))
	}
	if output.Projects[0].ResourceURI != "opengrok://project/platform" {
		t.Fatalf("resource URI = %q, want project URI", output.Projects[0].ResourceURI)
	}
	if output.Projects[0].ProjectURL == "" {
		t.Fatal("project URL is empty, want stable search URL")
	}
}

func TestListProjectsUsesConfiguredProjects(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	cfg.ProjectSource = config.ProjectSourceConfigured
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	got := []string{}
	for _, project := range output.Projects {
		got = append(got, project.Project)
	}
	if !slices.Equal(got, []string{"platform", "tools"}) {
		t.Fatalf("projects = %#v, want configured projects", got)
	}
}

func TestListProjectsSnapshotMatchesSearchValidation(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	cfg.ProjectSource = config.ProjectSourceAPI
	cfg.DefaultProject = "platform"
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	listed := make(map[string]bool)
	for _, item := range output.Projects {
		listed[item.Project] = true
		if err := service.validateConfiguredProjects([]string{item.Project}); err != nil {
			t.Fatalf("listed project %q rejected by validation: %v", item.Project, err)
		}
	}

	if err := service.validateConfiguredProjects([]string{"unknown"}); err == nil {
		t.Fatal("validateConfiguredProjects() error = nil, want UNKNOWN_PROJECT")
	} else if !IsCode(err, codeUnknownProject) {
		t.Fatalf("validateConfiguredProjects() error = %v, want UNKNOWN_PROJECT", err)
	}
	if listed["unknown"] {
		t.Fatal("unknown project appears in list_projects output")
	}
}

func TestListProjectsFirstPageReturnsPageAndTotal(t *testing.T) {
	projects := make([]string, 120)
	for i := range projects {
		projects[i] = fmt.Sprintf("project-%03d", i)
	}
	cfg := testConfig()
	cfg.Projects = projects
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	if output.TotalProjects != 120 {
		t.Fatalf("TotalProjects = %d, want 120", output.TotalProjects)
	}
	if len(output.Projects) != 50 {
		t.Fatalf("projects count = %d, want 50 (first page)", len(output.Projects))
	}
	if output.NextCursor == nil {
		t.Fatal("NextCursor is nil, want non-nil (more pages remain)")
	}
	if output.Projects[0].Project != "project-000" {
		t.Fatalf("first project = %q, want project-000", output.Projects[0].Project)
	}
}

func TestListProjectsSecondPageViaCursorReturnsCorrectOffset(t *testing.T) {
	projects := make([]string, 120)
	for i := range projects {
		projects[i] = fmt.Sprintf("project-%03d", i)
	}
	cfg := testConfig()
	cfg.Projects = projects
	service := NewService(cfg, &fakeBackend{})

	firstPage, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects first page error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first page NextCursor is nil")
	}

	secondPage, err := service.ListProjects(context.Background(), ListProjectsInput{
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("ListProjects second page error: %v", err)
	}

	if len(secondPage.Projects) != 50 {
		t.Fatalf("second page count = %d, want 50", len(secondPage.Projects))
	}
	if secondPage.Projects[0].Project != "project-050" {
		t.Fatalf("second page first project = %q, want project-050", secondPage.Projects[0].Project)
	}
	if secondPage.NextCursor == nil {
		t.Fatal("second page NextCursor is nil, want non-nil (page 3 remains)")
	}
}

func TestListProjectsLastPageHasNoNextCursor(t *testing.T) {
	projects := make([]string, 60)
	for i := range projects {
		projects[i] = fmt.Sprintf("project-%03d", i)
	}
	cfg := testConfig()
	cfg.Projects = projects
	service := NewService(cfg, &fakeBackend{})

	firstPage, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("first ListProjects error: %v", err)
	}
	if firstPage.NextCursor == nil {
		t.Fatal("first page NextCursor is nil, expected more pages")
	}

	lastPage, err := service.ListProjects(context.Background(), ListProjectsInput{
		Cursor: firstPage.NextCursor,
	})
	if err != nil {
		t.Fatalf("last ListProjects error: %v", err)
	}

	if len(lastPage.Projects) != 10 {
		t.Fatalf("last page count = %d, want 10", len(lastPage.Projects))
	}
	if lastPage.NextCursor != nil {
		t.Fatal("last page NextCursor is non-nil, want nil (no more pages)")
	}
}

func TestListProjectsInvalidCursorReturnsError(t *testing.T) {
	service := NewService(testConfig(), &fakeBackend{})
	badCursor := "not-a-valid-cursor!!!"

	_, err := service.ListProjects(context.Background(), ListProjectsInput{
		Cursor: &badCursor,
	})
	if err == nil {
		t.Fatal("ListProjects error is nil, want INVALID_CURSOR")
	}
	if !IsCode(err, "INVALID_CURSOR") {
		t.Fatalf("ListProjects error = %v, want INVALID_CURSOR", err)
	}
}

func TestListFilesReportsBackendTruncation(t *testing.T) {
	backend := &fakeBackend{
		fileEntries:       []opengrok.FileEntry{{Path: "src/main.go"}},
		fileListTruncated: true,
	}
	service := NewService(testConfig(), backend)

	output, err := service.ListFiles(context.Background(), ListFilesInput{})
	if err != nil {
		t.Fatalf("ListFiles error = %v", err)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal ListFiles output: %v", err)
	}
	if !strings.Contains(string(data), `"truncated":true`) {
		t.Fatalf("ListFiles output = %s, want truncated=true", data)
	}
	if output.Warning == nil {
		t.Fatal("ListFiles warning is nil, want truncation warning")
	}
}

func TestListFilesPopulatesPagination(t *testing.T) {
	backend := &fakeBackend{
		fileEntries: []opengrok.FileEntry{
			{Path: "src/a.go"},
			{Path: "src/b.go"},
			{Path: "src/c.go"},
		},
	}
	cfg := testConfig()
	cfg.DefaultProject = "platform"
	service := NewService(cfg, backend)

	output, err := service.ListFiles(context.Background(), ListFilesInput{PageSize: 2})
	if err != nil {
		t.Fatalf("ListFiles returned error: %v", err)
	}
	if output.TotalHits != 3 {
		t.Fatalf("TotalHits = %d, want 3", output.TotalHits)
	}
	if output.TotalPages != 2 {
		t.Fatalf("TotalPages = %d, want 2 (3 files / page size 2)", output.TotalPages)
	}
	if !output.HasMore {
		t.Fatal("HasMore = false, want true")
	}
}

func TestProjectOverviewReportsBackendTruncation(t *testing.T) {
	backend := &fakeBackend{
		fileEntries:       []opengrok.FileEntry{{Path: "src/main.go"}},
		fileListTruncated: true,
	}
	service := NewService(testConfig(), backend)

	output, err := service.GetProjectOverview(context.Background(), ProjectOverviewInput{})
	if err != nil {
		t.Fatalf("GetProjectOverview error = %v", err)
	}
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal project overview output: %v", err)
	}
	if !strings.Contains(string(data), `"truncated":true`) {
		t.Fatalf("ProjectOverview output = %s, want truncated=true", data)
	}
	if output.Warning == nil {
		t.Fatal("ProjectOverview warning is nil, want truncation warning")
	}
}

func TestListProjectsReturnsCatalogMetadata(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform", "tools"}
	cfg.ProjectSource = config.ProjectSourceAPI
	service := NewService(cfg, &fakeBackend{})

	output, err := service.ListProjects(context.Background(), ListProjectsInput{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}
	if output.CatalogSource != config.ProjectSourceAPI {
		t.Fatalf("CatalogSource = %q, want api", output.CatalogSource)
	}
	if !output.CatalogIsSnapshot {
		t.Fatal("CatalogIsSnapshot = false, want true")
	}
}

func TestUnknownProjectMessageMentionsSnapshotRestart(t *testing.T) {
	cfg := testConfig()
	cfg.Projects = []string{"platform"}
	cfg.ProjectSource = config.ProjectSourceConfigured
	cfg.DefaultProject = "platform"
	service := NewService(cfg, &fakeBackend{})

	err := service.validateConfiguredProjects([]string{"unknown"})
	if err == nil {
		t.Fatal("validateConfiguredProjects() error = nil, want UNKNOWN_PROJECT")
	}
	if !IsCode(err, codeUnknownProject) {
		t.Fatalf("error = %v, want UNKNOWN_PROJECT", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "snapshot") || !strings.Contains(msg, "restart") {
		t.Fatalf("error message = %q, want snapshot and restart guidance", msg)
	}
}
