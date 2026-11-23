// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var buildEvalServer func() (string, error)

func evalServer(t *testing.T) string {
	t.Helper()
	bin, err := buildEvalServer()
	if err != nil {
		t.Fatal(err)
	}
	return bin
}

func buildServer(dir string) (string, error) {
	bin := filepath.Join(dir, "opengrok-go-mcp")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	args := []string{"build", "-o", bin}
	if raceEnabled {
		args = append(args, "-race")
	}
	args = append(args, "./cmd/opengrok-go-mcp")
	build := exec.Command("go", args...)
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build eval server: %w\n%s", err, out)
	}
	return bin, nil
}

func stopHarness(t *testing.T, h *Harness) {
	t.Helper()
	if err := h.Stop(); err != nil {
		t.Errorf("stop harness: %v", err)
	}
}

func TestReleaseArchiveSmoke(t *testing.T) {
	path := os.Getenv("OPENGROK_MCP_EVAL_BINARY")
	if path == "" {
		t.Skip("set OPENGROK_MCP_EVAL_BINARY to the unpacked release executable")
	}
	bin, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	h, err := Start(context.Background(), bin, "testdata", HarnessOptions{ToolSurface: surfaceCompact})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopHarness(t, h) })

	if !h.hasTool("opengrok_search") || !h.hasTool("opengrok_read") {
		t.Fatal("compact search and read tools missing from release binary")
	}
	if want := os.Getenv("OPENGROK_MCP_EVAL_VERSION"); want != "" {
		got := h.Session().InitializeResult().ServerInfo.Version
		if got != want {
			t.Errorf("initialize server version = %q, want %q", got, want)
		}
	}
}
