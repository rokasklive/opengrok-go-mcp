// SPDX-License-Identifier: Apache-2.0

package config

// ErgonomicsInterfaceVersion is the pinned agent-facing description/manifest contract.
const ErgonomicsInterfaceVersion = "ergonomics-1"

// CapabilityReport is a process-lifetime snapshot of registered tools and gated
// capabilities, exposed to agents via opengrok://capabilities.
type CapabilityReport struct {
	InterfaceVersion string             `json:"interface_version"`
	ToolSurface      string             `json:"tool_surface"`
	AgentProfile     string             `json:"agent_profile"`
	Tools            []ToolCapability   `json:"tools"`
	Gated            []GatedCapability  `json:"gated"`
	ProjectCatalog   ProjectCatalogMeta `json:"project_catalog"`
}

// ToolCapability describes one registered MCP tool and its enabled operations.
type ToolCapability struct {
	Name       string   `json:"name"`
	Operations []string `json:"operations"`
	Summary    string   `json:"summary"`
}

// GatedCapability describes a probed-but-disabled capability family.
type GatedCapability struct {
	Capability  string `json:"capability"`
	ReasonCode  string `json:"reason_code"`
	Remediation string `json:"remediation"`
}

// ProjectCatalogMeta describes the startup-resolved project list.
type ProjectCatalogMeta struct {
	Source          string  `json:"source"`
	IsSnapshot      bool    `json:"is_snapshot"`
	ProjectCount    int     `json:"project_count"`
	DefaultProject  *string `json:"default_project"`
	ProjectRequired bool    `json:"project_required"`
}
