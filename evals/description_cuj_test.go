// SPDX-License-Identifier: Apache-2.0

package evals

import "testing"

func TestDescriptionCUJResolver(t *testing.T) {
	tests := []struct {
		task string
		want string
	}{
		{"find_symbol_definition", "opengrok_symbols"},
		{"find_symbol_references", "opengrok_symbols"},
		{"search_code_text", "opengrok_search"},
		{"read_known_file", "opengrok_read"},
		{"list_projects", "opengrok_projects"},
		{"list_project_files", "opengrok_projects"},
		{"project_overview", "opengrok_projects"},
	}
	for _, tt := range tests {
		if got := ResolveDescriptionCUJ(tt.task); got != tt.want {
			t.Fatalf("ResolveDescriptionCUJ(%q) = %q, want %q", tt.task, got, tt.want)
		}
	}
}
