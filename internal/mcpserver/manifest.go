// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

// BuildCapabilityReport constructs the agent-facing capability manifest from the
// running process configuration and probed capabilities.
func BuildCapabilityReport(cfg config.Config) config.CapabilityReport {
	var defaultProject *string
	if cfg.DefaultProject != "" {
		dp := cfg.DefaultProject
		defaultProject = &dp
	}
	report := config.CapabilityReport{
		InterfaceVersion: config.ErgonomicsInterfaceVersion,
		ToolSurface:      cfg.ToolSurface,
		AgentProfile:     cfg.AgentProfile,
		ProjectCatalog: config.ProjectCatalogMeta{
			Source:          catalogSourceLabel(cfg.ProjectSource),
			IsSnapshot:      true,
			ProjectCount:    len(cfg.Projects),
			DefaultProject:  defaultProject,
			ProjectRequired: cfg.DefaultProject == "",
		},
	}

	switch cfg.ToolSurface {
	case config.ToolSurfaceCompact:
		report.Tools = compactToolCapabilities(cfg)
	case config.ToolSurfaceGateway:
		report.Tools = []config.ToolCapability{
			{Name: "opengrok_discover", Operations: []string{"discover"}, Summary: "List gateway operations (experimental)."},
			{Name: "opengrok_call", Operations: []string{"call"}, Summary: "Dispatch a gateway operation by name (experimental)."},
		}
	default:
		report.Tools = fullToolCapabilities(cfg)
	}

	report.Gated = gatedCapabilities(cfg)
	return report
}

func catalogSourceLabel(source string) string {
	if source == "" {
		return config.ProjectSourceNone
	}
	return source
}

func compactToolCapabilities(cfg config.Config) []config.ToolCapability {
	out := []config.ToolCapability{}
	if ops := compactProjectsOperations(cfg); len(ops) > 0 {
		out = append(out, config.ToolCapability{
			Name:       "opengrok_projects",
			Operations: ops,
			Summary:    compactToolSummary("opengrok_projects", ops),
		})
	}
	if ops := compactSearchOperations(cfg); len(ops) > 0 {
		out = append(out, config.ToolCapability{
			Name:       "opengrok_search",
			Operations: ops,
			Summary:    compactToolSummary("opengrok_search", ops),
		})
	}
	if ops := compactSymbolsOperations(cfg); len(ops) > 0 {
		out = append(out, config.ToolCapability{
			Name:       "opengrok_symbols",
			Operations: ops,
			Summary:    compactToolSummary("opengrok_symbols", ops),
		})
	}
	if ops := compactReadOperations(cfg); len(ops) > 0 {
		out = append(out, config.ToolCapability{
			Name:       "opengrok_read",
			Operations: ops,
			Summary:    compactToolSummary("opengrok_read", ops),
		})
	}
	return out
}

func fullToolCapabilities(cfg config.Config) []config.ToolCapability {
	type toolSpec struct {
		name    string
		enabled bool
		summary string
	}
	specs := []toolSpec{
		{"list_projects", cfg.Capabilities.ListProjects, "List indexed OpenGrok projects."},
		{"search_code", cfg.Capabilities.SearchCode, "Full-text and path search."},
		{"search_and_read", cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext, "Search with inlined file context."},
		{"search_symbol_definitions", cfg.Capabilities.SearchSymbolDefinitions, "Find symbol definitions."},
		{"search_symbol_references", cfg.Capabilities.SearchSymbolReferences, "Find symbol references."},
		{"find_symbol_and_references", cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext, "Definition plus references in one call."},
		{"search_implementations", cfg.Capabilities.SearchSymbolReferences, "Best-effort implementation candidates."},
		{"search_cross_project_references", cfg.Capabilities.SearchSymbolReferences, "Cross-project symbol references."},
		{"list_symbols", cfg.Capabilities.ListSymbols, "List symbol definitions in scope."},
		{"list_files", cfg.Capabilities.ListFiles, "List files in a project path."},
		{"get_project_overview", cfg.Capabilities.ListFiles, "Project scale and language breakdown."},
		{"read_file", cfg.Capabilities.GetFileContext, "Read file content with pagination."},
		{"get_file_context", cfg.Capabilities.GetFileContext, "Read a line window in a file."},
	}
	out := make([]config.ToolCapability, 0, len(specs))
	for _, spec := range specs {
		if !spec.enabled {
			continue
		}
		out = append(out, config.ToolCapability{
			Name:       spec.name,
			Operations: []string{spec.name},
			Summary:    spec.summary,
		})
	}
	if cfg.Capabilities.Memory && memoryToolsEnabled(cfg) {
		out = append(out, config.ToolCapability{
			Name:       "memory_set",
			Operations: []string{"memory_set", "memory_get", "memory_list", "memory_delete", "memory_clear"},
			Summary:    "Process-scoped ephemeral memory (stdio full surface only).",
		})
	}
	return out
}

func gatedCapabilities(cfg config.Config) []config.GatedCapability {
	type gate struct {
		name        string
		enabled     bool
		reason      string
		remediation string
	}
	gates := []gate{
		{
			name:        "SearchCode",
			enabled:     cfg.Capabilities.SearchCode,
			reason:      "PROBE_FAILED",
			remediation: "Verify OpenGrok search access. If probes return 401/403, set OPENGROK_MCP_API_TOKEN and restart.",
		},
		{
			name:        "SearchSymbolDefinitions",
			enabled:     cfg.Capabilities.SearchSymbolDefinitions,
			reason:      "PROBE_FAILED",
			remediation: "Verify definition search is enabled on OpenGrok. Set OPENGROK_MCP_API_TOKEN if auth is required and restart.",
		},
		{
			name:        "SearchSymbolReferences",
			enabled:     cfg.Capabilities.SearchSymbolReferences,
			reason:      "PROBE_FAILED",
			remediation: "Verify reference search is enabled on OpenGrok. Set OPENGROK_MCP_API_TOKEN if auth is required and restart.",
		},
		{
			name:        "GetFileContext",
			enabled:     cfg.Capabilities.GetFileContext,
			reason:      "PROBE_FAILED",
			remediation: "Set OPENGROK_MCP_PROBE_FILE to a readable project/path and restart, or fix raw-file access.",
		},
		{
			name:        "ListFiles",
			enabled:     cfg.Capabilities.ListFiles,
			reason:      "PROBE_FAILED",
			remediation: "Verify OpenGrok /list access for the configured project. Set OPENGROK_MCP_API_TOKEN if auth is required and restart.",
		},
	}
	out := make([]config.GatedCapability, 0)
	for _, g := range gates {
		if g.enabled {
			continue
		}
		out = append(out, config.GatedCapability{
			Capability:  g.name,
			ReasonCode:  g.reason,
			Remediation: g.remediation,
		})
	}
	return out
}
