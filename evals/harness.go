// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HarnessOptions configures subprocess startup for eval and token benchmarks.
type HarnessOptions struct {
	ToolSurface string // full, compact, or gateway; default full
}

type Harness struct {
	session     *mcp.ClientSession
	tools       map[string]bool
	listedTools []*mcp.Tool
	stopFuncs   []func()
	buildDir    string
}

// Start builds the server binary, starts a hermetic backend, launches stdio subprocess,
// and records registered tools. Pair every Start with Stop.
func Start(ctx context.Context, moduleRoot, testdataDir string, opts HarnessOptions) (*Harness, error) {
	h := &Harness{tools: map[string]bool{}}

	surface := strings.TrimSpace(opts.ToolSurface)
	if surface == "" {
		surface = surfaceFull
	}

	buildDir, err := os.MkdirTemp("", "opengrok-go-mcp-eval-*")
	if err != nil {
		return nil, fmt.Errorf("create build dir: %w", err)
	}
	h.buildDir = buildDir
	h.stopFuncs = append(h.stopFuncs, func() { _ = os.RemoveAll(buildDir) })

	bin := filepath.Join(buildDir, "opengrok-go-mcp")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/opengrok-go-mcp")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		h.Stop()
		return nil, fmt.Errorf("build server: %w\n%s", err, out)
	}

	backendEnv, stopBackend, err := startBackend(ctx, testdataDir)
	if err != nil {
		h.Stop()
		return nil, fmt.Errorf("start backend: %w", err)
	}
	h.stopFuncs = append(h.stopFuncs, stopBackend)

	env := append([]string{
		"OPENGROK_MCP_TRANSPORT=stdio",
		"OPENGROK_MCP_TOOL_SURFACE=" + surface,
	}, backendEnv...)

	session, err := connectStdio(ctx, bin, nil, env)
	if err != nil {
		h.Stop()
		return nil, err
	}
	h.session = session
	h.stopFuncs = append(h.stopFuncs, func() { _ = session.Close() })

	lt, err := session.ListTools(ctx, nil)
	if err != nil {
		h.Stop()
		return nil, fmt.Errorf("list tools: %w", err)
	}
	h.listedTools = lt.Tools
	for _, t := range lt.Tools {
		h.tools[t.Name] = true
	}
	return h, nil
}

func (h *Harness) Stop() {
	for i := len(h.stopFuncs) - 1; i >= 0; i-- {
		h.stopFuncs[i]()
	}
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
