// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func TestBuildCapabilityReportMatchesCompactTools(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = allCapabilities()
	cfg.AgentProfile = config.AgentProfileEconomy
	cfg.Projects = []string{"platform", "tools"}
	cfg.ProjectSource = config.ProjectSourceAPI
	cfg.DefaultProject = "platform"
	cfg.CapabilityReport = BuildCapabilityReport(cfg)

	if cfg.CapabilityReport.ToolSurface != config.ToolSurfaceCompact {
		t.Fatalf("tool_surface = %q, want compact", cfg.CapabilityReport.ToolSurface)
	}
	if cfg.CapabilityReport.AgentProfile != config.AgentProfileEconomy {
		t.Fatalf("agent_profile = %q, want economy", cfg.CapabilityReport.AgentProfile)
	}
	if len(cfg.CapabilityReport.Tools) != 4 {
		t.Fatalf("tools len = %d, want 4 compact tools", len(cfg.CapabilityReport.Tools))
	}
	names := map[string]bool{}
	for _, tool := range cfg.CapabilityReport.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"opengrok_projects", "opengrok_search", "opengrok_symbols", "opengrok_read"} {
		if !names[want] {
			t.Fatalf("missing tool %q in manifest", want)
		}
	}
	if !cfg.CapabilityReport.ProjectCatalog.IsSnapshot {
		t.Fatal("project_catalog.is_snapshot should be true")
	}
	if cfg.CapabilityReport.ProjectCatalog.ProjectCount != 2 {
		t.Fatalf("project_count = %d, want 2", cfg.CapabilityReport.ProjectCatalog.ProjectCount)
	}
	if cfg.CapabilityReport.ProjectCatalog.DefaultProject == nil || *cfg.CapabilityReport.ProjectCatalog.DefaultProject != "platform" {
		t.Fatalf("default_project = %v, want platform", cfg.CapabilityReport.ProjectCatalog.DefaultProject)
	}
	if cfg.CapabilityReport.ProjectCatalog.ProjectRequired {
		t.Fatal("project_required should be false when default project is set")
	}
	if cfg.CapabilityReport.InterfaceVersion != config.ErgonomicsInterfaceVersion {
		t.Fatalf("interface_version = %q, want %q", cfg.CapabilityReport.InterfaceVersion, config.ErgonomicsInterfaceVersion)
	}
}

func TestGatedSymbolsSummaryOmitsReferences(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = allCapabilities()
	cfg.Capabilities.SearchSymbolReferences = false
	report := BuildCapabilityReport(cfg)

	var symbols config.ToolCapability
	for _, tool := range report.Tools {
		if tool.Name == "opengrok_symbols" {
			symbols = tool
			break
		}
	}
	if symbols.Name == "" {
		t.Fatal("opengrok_symbols missing from manifest")
	}
	for _, op := range symbols.Operations {
		if op == "references" {
			t.Fatalf("operations = %v, references should be absent", symbols.Operations)
		}
	}
	if strings.Contains(strings.ToLower(symbols.Summary), "reference") {
		t.Fatalf("summary = %q, should not mention references", symbols.Summary)
	}
}

func TestProjectCatalogRequiresProjectWhenNoDefault(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultProject = ""
	report := BuildCapabilityReport(cfg)
	if !report.ProjectCatalog.ProjectRequired {
		t.Fatal("project_required = false, want true")
	}
	if report.ProjectCatalog.DefaultProject != nil {
		t.Fatalf("default_project = %v, want null", report.ProjectCatalog.DefaultProject)
	}
}

func TestGatedCapabilitiesRemediationHasNoSecrets(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities.SearchCode = false
	cfg.Capabilities.SearchSymbolReferences = false
	report := BuildCapabilityReport(cfg)

	if len(report.Gated) == 0 {
		t.Fatal("expected gated capabilities when probes fail")
	}
	for _, g := range report.Gated {
		if strings.Contains(g.Remediation, "Bearer ") || strings.Contains(g.Remediation, "sk-") {
			t.Fatalf("remediation for %q looks like it contains a secret: %q", g.Capability, g.Remediation)
		}
		if g.Remediation == "" {
			t.Fatalf("remediation for %q is empty", g.Capability)
		}
	}
}
