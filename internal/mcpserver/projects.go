// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

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

	warnings := newWarningSet()
	if truncated {
		warnings.add(warnFileListTruncated, "OpenGrok file listing was truncated at 5,000 entries; total_hits and available pages are incomplete.")
	}

	return ListFilesOutput{
		Project:       project,
		Path:          input.Path,
		Files:         files,
		Pagination:    newPagination(offset, pageSize, total, nextCursor),
		Truncated:     truncated,
		WarningFields: warnings.fields(),
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

	warnings := newWarningSet()
	if truncated {
		warnings.add(warnFileListTruncated, "OpenGrok file listing was truncated at 5,000 entries; project overview counts and language statistics are incomplete.")
	}

	return ProjectOverviewOutput{
		Project:       project,
		TotalFiles:    totalFiles,
		TotalDirs:     totalDirs,
		TopDirs:       topDirs,
		TopFiles:      topFiles,
		Description:   fmt.Sprintf("Project %s overview", project),
		Truncated:     truncated,
		WarningFields: warnings.fields(),
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
