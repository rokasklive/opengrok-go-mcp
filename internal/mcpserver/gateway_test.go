// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func TestGatewayRegistryDoesNotExposeContentOrMemoryWithoutCapabilities(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{
		SearchCode:              true,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  true,
	}
	registry := buildGatewayRegistry(NewService(cfg, &fakeBackend{}), cfg)

	for _, operation := range []string{
		"compound.search_and_read",
		"compound.find_symbol_and_references",
		"memory.set",
		"memory.get",
		"memory.list",
		"memory.delete",
		"memory.clear",
	} {
		if _, ok := registry[operation]; ok {
			t.Fatalf("operation %q registered without required capability", operation)
		}
	}
}

func TestGatewayRegistryRegistersProjectOverviewOnlyWithListFiles(t *testing.T) {
	tests := []struct {
		name         string
		capabilities config.Capabilities
		wantPresent  bool
	}{
		{
			name:         "list projects only",
			capabilities: config.Capabilities{ListProjects: true},
			wantPresent:  false,
		},
		{
			name:         "list files only",
			capabilities: config.Capabilities{ListFiles: true},
			wantPresent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.ToolSurface = config.ToolSurfaceGateway
			cfg.Capabilities = tt.capabilities
			registry := buildGatewayRegistry(NewService(cfg, &fakeBackend{}), cfg)

			_, gotPresent := registry["project.overview"]
			if gotPresent != tt.wantPresent {
				t.Fatalf("project.overview present = %t, want %t", gotPresent, tt.wantPresent)
			}
		})
	}
}

func TestHTTPTransportDoesNotExposeProcessScopedMemory(t *testing.T) {
	cfg := testConfig()
	cfg.Transport = config.TransportHTTP
	cfg.Capabilities = config.Capabilities{Memory: true}
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	tools, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	for _, tool := range tools.Tools {
		if strings.HasPrefix(tool.Name, "memory_") {
			t.Fatalf("process-scoped memory tool %q exposed over HTTP", tool.Name)
		}
	}

	cfg.ToolSurface = config.ToolSurfaceGateway
	registry := buildGatewayRegistry(NewService(cfg, &fakeBackend{}), cfg)
	if _, ok := registry["memory.get"]; ok {
		t.Fatal("process-scoped memory gateway operation exposed over HTTP")
	}
}
