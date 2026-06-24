// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func TestReadFileResourceMatchesSlashContainingPath(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{
		fileContent: "final class Engine {}",
	}
	server := NewMCPServer(testConfig(), backend, "test")
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect returned error: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect returned error: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "opengrok://project/platform/files/src/services/Engine.swift",
	})
	if err != nil {
		t.Fatalf("ReadResource returned error: %v", err)
	}
	if backend.fileProject != "platform" {
		t.Fatalf("FileContent project = %q, want platform", backend.fileProject)
	}
	if backend.filePath != "src/services/Engine.swift" {
		t.Fatalf("FileContent path = %q, want src/services/Engine.swift", backend.filePath)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(result.Contents))
	}

	var output FileContextOutput
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &output); err != nil {
		t.Fatalf("resource JSON unmarshal returned error: %v", err)
	}
	if output.Content != "final class Engine {}" {
		t.Fatalf("content = %q, want file body", output.Content)
	}
}

func TestReadFileResourceLineFragmentSelectsContext(t *testing.T) {
	ctx := context.Background()
	backend := &fakeBackend{
		fileContent: "one\ntwo\nthree\n",
	}
	server := NewMCPServer(testConfig(), backend, "test")
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect returned error: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect returned error: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "opengrok://project/platform/files/src/services/Engine.swift#L2",
	})
	if err != nil {
		t.Fatalf("ReadResource returned error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(result.Contents))
	}

	var output FileContextOutput
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &output); err != nil {
		t.Fatalf("resource JSON unmarshal returned error: %v", err)
	}
	if output.LineNumber != 2 {
		t.Fatalf("LineNumber = %d, want 2", output.LineNumber)
	}
	if output.Content != "one\ntwo\nthree" {
		t.Fatalf("Content = %q, want selected context around line 2", output.Content)
	}
}

func TestCapabilitiesResourceAlwaysRegistered(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{} // all probes disabled
	server := NewMCPServer(cfg, &fakeBackend{}, "test")
	clientSession, cleanup := connectMCPServer(t, server)
	defer cleanup()

	result, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "opengrok://capabilities",
	})
	if err != nil {
		t.Fatalf("ReadResource capabilities returned error: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents length = %d, want 1", len(result.Contents))
	}

	var report config.CapabilityReport
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &report); err != nil {
		t.Fatalf("unmarshal capability report: %v", err)
	}
	if report.ToolSurface == "" {
		t.Fatal("tool_surface is empty in capability report")
	}
}
