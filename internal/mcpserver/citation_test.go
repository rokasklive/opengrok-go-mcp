// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "testing"

func TestCitationMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		title string
		url   string
		line  int
		want  string
	}{
		{
			name:  "title and url produce a clickable link",
			title: "Engine.swift:42",
			url:   "https://grok.example.com/source/xref/platform/src/Engine.swift#42",
			line:  42,
			want:  "[Engine.swift:42](https://grok.example.com/source/xref/platform/src/Engine.swift#42)",
		},
		{
			name:  "no url yields no markdown (avoid empty link)",
			title: "Engine.swift:42",
			url:   "",
			line:  42,
			want:  "",
		},
		{
			name:  "empty title falls back to url as anchor text",
			title: "",
			url:   "https://grok.example.com/x",
			line:  0,
			want:  "[https://grok.example.com/x](https://grok.example.com/x)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := citation(tt.title, tt.url, tt.line).Markdown
			if got != tt.want {
				t.Fatalf("citation(%q,%q,%d).Markdown = %q, want %q", tt.title, tt.url, tt.line, got, tt.want)
			}
		})
	}
}
