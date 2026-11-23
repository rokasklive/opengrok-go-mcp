// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HarnessOptions configures subprocess startup for eval and token benchmarks.
type HarnessOptions struct {
	ToolSurface        string // full, compact, or gateway
	AgentProfile       string // economy or rich; unset leaves server default (economy)
	GateReferenceProbe bool   // disable reference search capability probe (hermetic)
}

type Harness struct {
	session     *mcp.ClientSession
	tools       map[string]bool
	listedTools []*mcp.Tool
	stopBackend func()
}

// Start launches a server against a hermetic backend and lists its tools.
// Pair every Start with Stop.
func Start(ctx context.Context, bin, testdataDir string, opts HarnessOptions) (*Harness, error) {
	surface := strings.TrimSpace(opts.ToolSurface)
	if surface == "" {
		surface = surfaceFull
	}

	backendEnv, stopBackend, err := startBackendWithOptions(ctx, testdataDir, BackendOptions{
		GateReferenceProbe: opts.GateReferenceProbe,
	})
	if err != nil {
		return nil, fmt.Errorf("start backend: %w", err)
	}

	env := append([]string{
		"OPENGROK_MCP_TRANSPORT=stdio",
		"OPENGROK_MCP_TOOL_SURFACE=" + surface,
	}, backendEnv...)
	if profile := strings.TrimSpace(opts.AgentProfile); profile != "" {
		env = append(env, "OPENGROK_MCP_AGENT_PROFILE="+profile)
	}

	h, err := startHarness(ctx, bin, env)
	if err != nil {
		stopBackend()
		return nil, err
	}
	h.stopBackend = stopBackend
	return h, nil
}

// StartLive launches the server against the OpenGrok instance named by the
// caller's OPENGROK_MCP_* environment.
func StartLive(ctx context.Context, bin string, opts HarnessOptions) (*Harness, error) {
	surface := strings.TrimSpace(opts.ToolSurface)
	if surface == "" {
		surface = surfaceCompact
	}

	env := []string{
		"OPENGROK_MCP_TRANSPORT=stdio",
		"OPENGROK_MCP_TOOL_SURFACE=" + surface,
		"OPENGROK_MCP_PROJECT_REQUIRED=false",
		"OPENGROK_MCP_AUTO_EXPAND_CONTEXT=false",
	}
	if profile := strings.TrimSpace(opts.AgentProfile); profile != "" {
		env = append(env, "OPENGROK_MCP_AGENT_PROFILE="+profile)
	}

	return startHarness(ctx, bin, env)
}

func startHarness(ctx context.Context, bin string, env []string) (*Harness, error) {
	session, err := connectStdio(ctx, bin, env)
	if err != nil {
		return nil, err
	}

	lt, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("list tools: %w", err)
	}
	h := &Harness{session: session, tools: map[string]bool{}, listedTools: lt.Tools}
	for _, tool := range lt.Tools {
		h.tools[tool.Name] = true
	}
	return h, nil
}

func (h *Harness) Stop() error {
	if h.stopBackend != nil {
		defer h.stopBackend()
	}
	return h.session.Close()
}

func (h *Harness) hasTool(name string) bool {
	return h.tools[name]
}

func (h *Harness) Session() *mcp.ClientSession {
	return h.session
}

func (h *Harness) ListedTools() []*mcp.Tool {
	return h.listedTools
}
