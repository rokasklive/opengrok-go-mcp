// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

const compactEconomyHint = compactAgentProfileHint

const (
	natureClaimID         = "opengrok-nature"
	defaultProjectClaimID = "default-project"
)

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
		compactClaimSlot("Nature", natureClaimID),
		strings.Join(operationBlurbsForTool("opengrok_projects", compactProjectsOperations(cfg)), ". "),
		compactClaimSlot("Supported", defaultProjectClaimID),
		compactProjectsExample(cfg),
		compactDefaultProjectSlot(cfg),
		compactCapabilitiesSlot(),
	)
}

func compactSearchDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactSearchLead(cfg),
		compactClaimSlot("Nature", natureClaimID),
		strings.Join(operationBlurbsForTool("opengrok_search", compactSearchOperations(cfg)), ". "),
		compactClaimSlot("Supported syntax", "phrase", "auto-quote", "regex", "field-defs", "field-refs", "field-path"),
		compactClaimSlot("Unsupported and pitfalls", "bare-regex", "wildcard-in-phrase", "inheritance", "call-graph"),
		compactExamplesSlot("phrase", "regex", "field-defs"),
		compactDefaultProjectSlot(cfg),
		compactEconomyHint,
		`Narrow with path_prefix (restrict TO a path) or path_exclude (drop paths; space-separate multiple values)`,
		`For symbol definitions/references use opengrok_symbols, not this tool`,
		`Surface citation.markdown (a ready [path:line](url) link) for hits you cite, so the user gets a clickable source`,
		compactCapabilitiesSlot(),
	)
}

func compactSymbolsDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactSymbolsLead(cfg),
		compactClaimSlot("Nature", natureClaimID),
		strings.Join(operationBlurbsForTool("opengrok_symbols", compactSymbolsOperations(cfg)), ". "),
		compactClaimSlot("Supported syntax", "field-defs", "field-refs"),
		compactClaimSlot("Unsupported and pitfalls", "inheritance", "call-graph"),
		compactExamplesSlot("field-defs"),
		compactDefaultProjectSlot(cfg),
		compactEconomyHint,
		`Pass a bare symbol name (PaymentProcessor), not quoted`,
		`Surface citation.markdown (a ready [path:line](url) link) when citing a definition or reference`,
		compactCapabilitiesSlot(),
	)
}

func compactReadDescription(cfg config.Config) string {
	return joinDescriptionParts(
		compactReadLead(cfg),
		compactClaimSlot("Nature", natureClaimID),
		strings.Join(operationBlurbsForTool("opengrok_read", compactReadOperations(cfg)), ". "),
		compactClaimSlot("Supported", defaultProjectClaimID),
		compactReadExample(),
		compactDefaultProjectSlot(cfg),
		compactEconomyHint,
		"Use project + file_path (and line_number for context) from a prior search/symbol result",
		"No offset/limit params: page a truncated file (truncated=true) with cursor from next_cursor; for a slice use operation=context with line_number + before/after",
		"Do not WebFetch display_url/raw_url — this tool sends configured auth and falls back to /raw",
		"Surface citation.markdown (a ready [path:line](url) link) as a clickable citation when you answer about the file",
		compactCapabilitiesSlot(),
	)
}

func compactClaimSlot(title string, claimIDs ...string) string {
	parts := make([]string, 0, len(claimIDs))
	for _, claimID := range claimIDs {
		claim := mustClaim(claimID)
		parts = append(parts, claim.AgentClaimText+" (claim_id="+claim.ID+")")
	}
	if len(parts) == 0 {
		return ""
	}
	return title + ": " + strings.Join(parts, " ")
}

func compactExamplesSlot(claimIDs ...string) string {
	parts := make([]string, 0, len(claimIDs))
	for _, claimID := range claimIDs {
		claim := mustClaim(claimID)
		if claim.Example == "" {
			continue
		}
		parts = append(parts, claim.Example+" (claim_id="+claim.ID+")")
	}
	if len(parts) == 0 {
		return ""
	}
	return "Example: " + strings.Join(parts, "; ")
}

// compactProjectsExample renders a copy-pasteable example call that is valid for
// the opengrok_projects schema. It must reference an operation the tool actually
// enables — the default-project claim's example is search-shaped and would name a
// nonexistent operation/field here.
func compactProjectsExample(cfg config.Config) string {
	ops := compactProjectsOperations(cfg)
	switch {
	case operationEnabled(ops, "list"):
		return `Example: {"operation":"list"}`
	case operationEnabled(ops, "overview"):
		project := cfg.DefaultProject
		if project == "" {
			project = "<project>"
		}
		return `Example: {"operation":"overview","project":"` + project + `"}`
	case len(ops) > 0:
		return `Example: {"operation":"` + ops[0] + `"}`
	default:
		return ""
	}
}

// compactReadExample renders a copy-pasteable example call valid for the
// opengrok_read schema; omitting project demonstrates default-project resolution.
func compactReadExample() string {
	return `Example: {"operation":"file","file_path":"src/Engine.swift"}`
}

func operationEnabled(operations []string, operation string) bool {
	for _, op := range operations {
		if op == operation {
			return true
		}
	}
	return false
}

func compactDefaultProjectSlot(cfg config.Config) string {
	if cfg.DefaultProject == "" {
		return "Default project: none configured; pass project explicitly when required"
	}
	return `Default project: omitting project uses "` + cfg.DefaultProject + `" (claim_id=` + defaultProjectClaimID + `)`
}

func compactCapabilitiesSlot() string {
	return "More: opengrok://capabilities carries the full syntax catalog and edge conditions"
}

func mustClaim(id string) Claim {
	claim, ok := ClaimByID(id)
	if !ok {
		panic("unknown claim ID in compact description: " + id)
	}
	return claim
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
