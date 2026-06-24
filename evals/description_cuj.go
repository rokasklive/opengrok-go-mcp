// SPDX-License-Identifier: Apache-2.0

package evals

import "strings"

// descriptionCUJMap maps cold-agent task labels to the first compact tool agents should pick.
var descriptionCUJMap = map[string]string{
	"find_symbol_definition": "opengrok_symbols",
	"find_symbol_references": "opengrok_symbols",
	"search_code_text":       "opengrok_search",
	"read_known_file":        "opengrok_read",
	"list_projects":          "opengrok_projects",
	"list_project_files":     "opengrok_projects",
	"project_overview":       "opengrok_projects",
}

// ResolveDescriptionCUJ returns the expected compact tool for a task label.
func ResolveDescriptionCUJ(task string) string {
	task = strings.TrimSpace(strings.ToLower(task))
	return descriptionCUJMap[task]
}
