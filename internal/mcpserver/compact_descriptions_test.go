// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

var claimIDPattern = regexp.MustCompile(`claim_id=([a-z0-9-]+)`)

func TestCompactToolDescriptionsAreRegistryGrounded(t *testing.T) {
	cfg := testConfig()
	cfg.ToolSurface = config.ToolSurfaceCompact
	cfg.Capabilities = allCapabilities()
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	byName := map[string]string{}
	for _, tool := range tools.Tools {
		byName[tool.Name] = tool.Description
	}

	for _, toolName := range []string{"opengrok_projects", "opengrok_search", "opengrok_symbols", "opengrok_read"} {
		desc := byName[toolName]
		if desc == "" {
			t.Fatalf("%s description is empty or tool is missing", toolName)
		}
		assertDescriptionHasCommonGrounding(t, toolName, desc, cfg.DefaultProject)
	}

	assertDescriptionReferencesClaims(t, "opengrok_projects", byName["opengrok_projects"], defaultProjectClaimID)
	assertDescriptionReferencesClaims(t, "opengrok_read", byName["opengrok_read"], defaultProjectClaimID)
	assertDescriptionReferencesClaims(t, "opengrok_search", byName["opengrok_search"],
		"phrase", "auto-quote", "regex", "field-defs", "field-refs", "field-path",
		"bare-regex", "wildcard-in-phrase", "inheritance", "call-graph",
	)
	assertDescriptionReferencesClaims(t, "opengrok_symbols", byName["opengrok_symbols"],
		"field-defs", "field-refs", "inheritance", "call-graph",
	)
}

func assertDescriptionHasCommonGrounding(t *testing.T, toolName string, desc string, defaultProject string) {
	t.Helper()
	for _, want := range []string{"full-text", "ctags", "not an AST", "Example:", "opengrok://capabilities"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("%s description missing %q: %s", toolName, want, desc)
		}
	}
	if !strings.Contains(desc, `Default project: omitting project uses "`+defaultProject+`"`) {
		t.Fatalf("%s description missing named default project %q: %s", toolName, defaultProject, desc)
	}
	assertDescriptionReferencesClaims(t, toolName, desc, natureClaimID, defaultProjectClaimID)
	assertOnlyRegisteredClaimIDs(t, toolName, desc)
}

func assertDescriptionReferencesClaims(t *testing.T, toolName string, desc string, claimIDs ...string) {
	t.Helper()
	for _, claimID := range claimIDs {
		if !strings.Contains(desc, "claim_id="+claimID) {
			t.Fatalf("%s description missing claim_id=%s: %s", toolName, claimID, desc)
		}
	}
}

func TestCompactReadDescriptionPaginationGuidance(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = allCapabilities()
	desc := compactReadDescription(cfg)
	// The offset/limit report came from agents guessing a pagination model the
	// tool does not have; the description must steer them to the real ones.
	for _, want := range []string{"offset/limit", "cursor", "next_cursor", "operation=context"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("opengrok_read description missing pagination guidance %q: %s", want, desc)
		}
	}
	if !strings.Contains(desc, "citation.markdown") {
		t.Fatalf("opengrok_read description should nudge surfacing citation.markdown: %s", desc)
	}
}

func assertOnlyRegisteredClaimIDs(t *testing.T, toolName string, desc string) {
	t.Helper()
	matches := claimIDPattern.FindAllStringSubmatch(desc, -1)
	if len(matches) == 0 {
		t.Fatalf("%s description has no claim_id references: %s", toolName, desc)
	}
	for _, match := range matches {
		if _, ok := ClaimByID(match[1]); !ok {
			t.Fatalf("%s description references unknown claim_id=%s: %s", toolName, match[1], desc)
		}
	}
}
