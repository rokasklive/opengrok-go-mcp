// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strings"
)

const (
	AgentProfileRich    = "rich"
	AgentProfileEconomy = "economy"
)

// NormalizeAgentProfile returns the canonical profile name. Empty means economy.
func NormalizeAgentProfile(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return AgentProfileEconomy, nil
	}
	switch value {
	case AgentProfileRich, AgentProfileEconomy:
		return value, nil
	default:
		return "", fmt.Errorf(
			"invalid OPENGROK_MCP_AGENT_PROFILE %q; valid values: %s, %s",
			value,
			AgentProfileEconomy,
			AgentProfileRich,
		)
	}
}

// IsEconomyProfile reports whether the active profile uses economy defaults.
func IsEconomyProfile(profile string) bool {
	return strings.ToLower(strings.TrimSpace(profile)) == AgentProfileEconomy
}
