// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

const compactEconomyHint = compactAgentProfileHint

var compactOperationBlurbs = map[string]map[string]string{
	"opengrok_projects": {
		"list":     "operation=list returns indexed projects (paginated; pass next_cursor)",
		"files":    "operation=files lists files under an optional path in a project (paginated; pass next_cursor)",
		"overview": "operation=overview returns language breakdown, file/directory counts, and top-level entries — use for \"what languages does project X use?\"",
	},
	"opengrok_search": {
		"code": "operation=code searches text, file paths (mode=path), or history (mode=history); query required",
		"read": "operation=read runs the same search and returns file content around each match in one call (fewer round trips); query required",
	},
	"opengrok_symbols": {
		"definitions":     "operation=definitions finds symbol definitions; symbol required",
		"references":      "operation=references finds symbol references; symbol required",
		"find":            "operation=find returns a definition with surrounding context plus references in one call; symbol required",
		"implementations": "operation=implementations finds candidate implementations (best-effort — OpenGrok has no semantic implementation map); symbol required",
		"cross_project":   "operation=cross_project finds references across projects, grouped by project; symbol required",
		"list":            "operation=list inventories definitions in a path; optional path_prefix, kind (page-local — heed warning), file_type; set include_snippets=false for broad sweeps",
	},
	"opengrok_read": {
		"file":    "operation=file returns full file content (paginated via next_cursor when truncated; total_lines always returned); file_path required",
		"context": "operation=context returns a line window around line_number (tune with before/after); file_path and line_number required",
	},
}

func compactProjectsDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactProjectsLead(cfg),
		strings.Join(operationBlurbsForTool("opengrok_projects", compactProjectsOperations(cfg)), ". "),
		compactProjectScopeNote(cfg),
	)
}

func compactSearchDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactSearchLead(cfg),
		strings.Join(operationBlurbsForTool("opengrok_search", compactSearchOperations(cfg)), ". "),
		compactProjectScopeNote(cfg),
		compactEconomyHint,
		`QUERY SYNTAX: wrap multi-word queries in quotes for exact phrases ("extends PaymentProcessor"); bare multi-word queries are auto-quoted — set tokenized=true to search words independently`,
		`Inline syntax: -path:legacy, +path:domain, defs:Name; date:[…] works only in mode=history (ignored elsewhere with a warning)`,
		`Narrow with path_prefix (restrict TO a path) or path_exclude (drop paths; space-separate multiple values)`,
		`Wildcards (* ?) cannot be used inside quoted phrases`,
		`For symbol definitions/references use opengrok_symbols, not this tool`,
		`Include citation.url when citing a specific hit`,
	)
}

func compactSymbolsDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactSymbolsLead(cfg),
		`Pass a bare symbol name (PaymentProcessor), not quoted`,
		strings.Join(operationBlurbsForTool("opengrok_symbols", compactSymbolsOperations(cfg)), ". "),
		compactProjectScopeNote(cfg),
		compactEconomyHint,
		`Results are full-text/ctags-backed, not an AST/call graph`,
		`Include citation.url when citing a definition or reference`,
	)
}

func compactReadDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactReadLead(cfg),
		strings.Join(operationBlurbsForTool("opengrok_read", compactReadOperations(cfg)), ". "),
		compactProjectScopeNote(cfg),
		compactEconomyHint,
		"Use project + file_path (and line_number for context) from a prior search/symbol result",
		"Do not WebFetch display_url/raw_url — this tool sends configured auth and falls back to /raw",
		"Include citation.url when you answer about the file",
	)
}

func operationBlurbsForTool(tool string, operations []string) []string {
	blurbs := compactOperationBlurbs[tool]
	out := make([]string, 0, len(operations))
	for _, op := range operations {
		if text, ok := blurbs[op]; ok {
			out = append(out, text)
		}
	}
	return out
}

func joinDescriptionParts(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.HasSuffix(part, ".") {
			part += "."
		}
		out = append(out, part)
	}
	return strings.Join(out, " ")
}
