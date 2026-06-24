// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func TestResolveResponseModeDefaultsByProfile(t *testing.T) {
	tests := []struct {
		profile  string
		request  string
		wantMode string
	}{
		{profile: config.AgentProfileRich, request: "", wantMode: "full"},
		{profile: config.AgentProfileEconomy, request: "", wantMode: "compact"},
		{profile: config.AgentProfileEconomy, request: "full", wantMode: "full"},
		{profile: config.AgentProfileRich, request: "compact", wantMode: "compact"},
	}
	for _, tt := range tests {
		t.Run(tt.profile+"_"+tt.request, func(t *testing.T) {
			cfg := testConfig()
			cfg.AgentProfile = tt.profile
			svc := NewService(cfg, &fakeBackend{})
			got, err := svc.resolveResponseMode(tt.request)
			if err != nil {
				t.Fatalf("resolveResponseMode error: %v", err)
			}
			if got != tt.wantMode {
				t.Fatalf("mode = %q, want %q", got, tt.wantMode)
			}
		})
	}
}

func TestShouldExpandContextDefaultsByProfile(t *testing.T) {
	cfg := testConfig()
	cfg.AutoExpandContext = true
	cfg.AgentProfile = config.AgentProfileEconomy
	svc := NewService(cfg, &fakeBackend{})
	if svc.shouldExpandContext(nil) {
		t.Fatal("economy profile should default expand_context to false")
	}
	cfg.AgentProfile = config.AgentProfileRich
	svc = NewService(cfg, &fakeBackend{})
	if !svc.shouldExpandContext(nil) {
		t.Fatal("rich profile should default expand_context to true when AutoExpandContext=true")
	}
	explicit := true
	if !svc.shouldExpandContext(&explicit) {
		t.Fatal("explicit expand_context=true should win over profile")
	}
}

func TestIncludeLinksDefaultsByProfile(t *testing.T) {
	cfg := testConfig()
	cfg.IncludeLinksDefault = true
	cfg.AgentProfile = config.AgentProfileEconomy
	svc := NewService(cfg, &fakeBackend{})
	if svc.includeLinks(nil) {
		t.Fatal("economy profile should default include_links to false")
	}
	cfg.AgentProfile = config.AgentProfileRich
	svc = NewService(cfg, &fakeBackend{})
	if !svc.includeLinks(nil) {
		t.Fatal("rich profile should default include_links to true")
	}
	explicit := true
	if !svc.includeLinks(&explicit) {
		t.Fatal("explicit include_links=true should win over profile")
	}
}
