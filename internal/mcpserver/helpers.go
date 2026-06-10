// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
)

func cursorValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
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

func invalidCursorError() error {
	return &Error{
		Code:    codeInvalidCursor,
		Message: "Invalid cursor.",
	}
}
