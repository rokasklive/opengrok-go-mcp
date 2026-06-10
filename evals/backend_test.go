// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestHermeticBackendProjectsIndexed(t *testing.T) {
	testdataDir := filepath.Join("testdata")
	env, stop, err := startBackend(context.Background(), testdataDir)
	if err != nil {
		t.Fatalf("startBackend: %v", err)
	}
	defer stop()

	baseURL := envValue(env, "OPENGROK_MCP_BASE_URL")
	resp, err := http.Get(baseURL + "/projects/indexed")
	if err != nil {
		t.Fatalf("GET projects/indexed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "platform") {
		t.Fatalf("body missing platform: %s", body)
	}
}

func TestHermeticBackendSearchProbe(t *testing.T) {
	testdataDir := filepath.Join("testdata")
	env, stop, err := startBackend(context.Background(), testdataDir)
	if err != nil {
		t.Fatalf("startBackend: %v", err)
	}
	defer stop()

	baseURL := envValue(env, "OPENGROK_MCP_BASE_URL")
	resp, err := http.Get(baseURL + "/search?full=test")
	if err != nil {
		t.Fatalf("GET search probe: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			return e[len(prefix):]
		}
	}
	return ""
}
