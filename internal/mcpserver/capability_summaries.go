// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func hasOp(ops []string, want string) bool {
	for _, op := range ops {
		if op == want {
			return true
		}
	}
	return false
}

func compactToolSummary(name string, ops []string) string {
	switch name {
	case "opengrok_projects":
		return summarizeProjectsSummary(ops)
	case "opengrok_search":
		return summarizeSearchSummary(ops)
	case "opengrok_symbols":
		return summarizeSymbolsSummary(ops)
	case "opengrok_read":
		return summarizeReadSummary(ops)
	default:
		return strings.Join(ops, ", ")
	}
}

func summarizeProjectsSummary(ops []string) string {
	parts := make([]string, 0, 3)
	if hasOp(ops, "list") {
		parts = append(parts, "indexed projects")
	}
	if hasOp(ops, "files") {
		parts = append(parts, "file listing")
	}
	if hasOp(ops, "overview") {
		parts = append(parts, "project overview")
	}
	if len(parts) == 0 {
		return "OpenGrok projects (no operations enabled)."
	}
	return strings.Join(parts, ", ") + "."
}

func summarizeSearchSummary(ops []string) string {
	parts := make([]string, 0, 2)
	if hasOp(ops, "code") {
		parts = append(parts, "full-text, path, and history search")
	}
	if hasOp(ops, "read") {
		parts = append(parts, "search-and-read")
	}
	if len(parts) == 0 {
		return "OpenGrok search (no operations enabled)."
	}
	return strings.Join(parts, "; ") + "."
}

func summarizeSymbolsSummary(ops []string) string {
	parts := make([]string, 0, 4)
	if hasOp(ops, "definitions") {
		parts = append(parts, "definitions")
	}
	if hasOp(ops, "references") || hasOp(ops, "find") || hasOp(ops, "cross_project") {
		parts = append(parts, "references")
	}
	if hasOp(ops, "implementations") {
		parts = append(parts, "implementation candidates")
	}
	if hasOp(ops, "list") {
		parts = append(parts, "structural listing")
	}
	if len(parts) == 0 {
		return "ctags symbols (no operations enabled)."
	}
	return "ctags symbols: " + strings.Join(parts, ", ") + "."
}

func summarizeReadSummary(ops []string) string {
	if len(ops) == 0 {
		return "file read (no operations enabled)."
	}
	return "read a known file or line window."
}

func compactProjectsLead(cfg config.Config) string {
	ops := compactProjectsOperations(cfg)
	if len(ops) == 0 {
		return "Work with indexed OpenGrok projects (no operations enabled on this server)."
	}
	if len(ops) == 1 && ops[0] == "list" {
		return "List indexed OpenGrok projects."
	}
	return "Work with indexed OpenGrok projects."
}

func compactSearchLead(cfg config.Config) string {
	ops := compactSearchOperations(cfg)
	if len(ops) == 0 {
		return "Search OpenGrok code (no search operations enabled on this server)."
	}
	return "Search OpenGrok code with Apache Lucene."
}

func compactSymbolsLead(cfg config.Config) string {
	ops := compactSymbolsOperations(cfg)
	if len(ops) == 0 {
		return "Work with ctags symbols (no symbol operations enabled on this server)."
	}
	hasDefs := hasOp(ops, "definitions")
	hasRefs := hasOp(ops, "references") || hasOp(ops, "find") || hasOp(ops, "cross_project")
	switch {
	case hasDefs && hasRefs:
		return `Work with ctags symbols — find where a symbol is defined and who references it.`
	case hasDefs:
		return `Work with ctags symbols — find where a symbol is defined.`
	case hasRefs:
		return `Work with ctags symbols — find who references a symbol.`
	default:
		return `Work with ctags symbols — inventory and structural listing.`
	}
}

func compactReadLead(cfg config.Config) string {
	if len(compactReadOperations(cfg)) == 0 {
		return "Read OpenGrok files (file read not enabled on this server)."
	}
	return "Read a file you already located."
}

func compactProjectScopeNote(cfg config.Config) string {
	if cfg.DefaultProject == "" {
		return "This server has no default project — pass project on every scoped call or call opengrok_projects operation=list first. Do not infer project from the local repo."
	}
	return "Omit project when the server has a default; otherwise call opengrok_projects operation=list first. Do not infer project from the local repo."
}

const compactAgentProfileHint = "Agent profile OPENGROK_MCP_AGENT_PROFILE defaults to economy (lean payloads, no auto expansion). Set rich for expanded search context by default. Per-call expand_context overrides when needed."
