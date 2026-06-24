// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "fmt"

const expansionBudgetWarnFraction = 0.5

func estimateResultsPayloadBytes(results []Result) int {
	total := 0
	for _, r := range results {
		if r.Snippet != nil {
			total += len(*r.Snippet)
		}
		if r.Context != nil {
			total += len(r.Context.Content)
		}
	}
	return total
}

func maybeWarnExpansionBudget(warnings *warningSet, expansion *ExpansionDiagnostics, results []Result) {
	if expansion == nil || expansion.ExpandedContextBytes <= 0 {
		return
	}
	total := estimateResultsPayloadBytes(results)
	if total == 0 {
		return
	}
	share := float64(expansion.ExpandedContextBytes) / float64(total)
	if share < expansionBudgetWarnFraction {
		return
	}
	warnings.add(warnExpansionBudgetHigh, formatExpansionBudgetWarning(share, expansion.ExpandedContextBytes, total))
}

func formatExpansionBudgetWarning(share float64, expandedBytes, totalBytes int) string {
	return fmt.Sprintf(
		"Auto-expanded context is ~%.0f%% of this page's payload (%d of %d bytes). Set expand_context=false or OPENGROK_MCP_AGENT_PROFILE=economy for leaner sweeps.",
		share*100,
		expandedBytes,
		totalBytes,
	)
}
