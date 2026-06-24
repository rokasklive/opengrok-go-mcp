// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func registerResources(server *mcp.Server, service *Service, cfg config.Config) {
	server.AddResource(&mcp.Resource{
		URI:         "opengrok://capabilities",
		Name:        "capabilities",
		Title:       "OpenGrok MCP capabilities",
		Description: "Runtime tool surface, enabled operations, and gated capability remediation. Read before planning multi-step workflows.",
		MIMEType:    "application/json",
	}, service.capabilitiesResource)

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

func (s *Service) capabilitiesResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	_ = ctx
	report := s.cfg.CapabilityReport
	if report.ToolSurface == "" {
		report = BuildCapabilityReport(s.cfg)
	}
	return jsonResource(req.Params.URI, report)
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
