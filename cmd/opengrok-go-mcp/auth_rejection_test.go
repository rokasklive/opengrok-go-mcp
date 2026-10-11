// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func authTestConfig(serverURL string) config.Config {
	cfg := config.Default()
	cfg.OpenGrokAPIBaseURL = serverURL + "/api/v1"
	cfg.OpenGrokWebBaseURL = serverURL + "/source"
	cfg.OpenGrokAuthHeader = "Basic dXNlcjp3cm9uZw=="
	cfg.ProjectScrapeEnabled = true
	cfg.RetryMaxAttempts = 3
	cfg.RetryBaseDelay = time.Millisecond
	return cfg
}

// runStartup runs the same discovery and probing steps as run().
func runStartup(t *testing.T, cfg *config.Config, client *opengrok.Client) (config.Capabilities, []string, error) {
	t.Helper()
	var logs []string
	logf := func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) }
	ctx := context.Background()
	if err := resolveProjectAllowlist(ctx, cfg, client, logf); err != nil {
		t.Fatalf("resolveProjectAllowlist() error = %v", err)
	}
	caps, err := probeCapabilities(ctx, client, client, *cfg, logf)
	return caps, logs, err
}

// Every rejected request can count as a failed directory login, so a startup
// with rejected credentials must send exactly one request — and stay up, since
// exiting makes MCP clients restart the server and try again.
func TestStartupSendsOneRequestWhenCredentialsRejected(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="EIS opengrok"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := authTestConfig(server.URL)
	client := opengrok.NewClient(cfg.OpenGrokAPIBaseURL, server.Client(), opengrokOptions(cfg)...)

	caps, logs, err := runStartup(t, &cfg, client)
	if err != nil {
		t.Fatalf("probeCapabilities() error = %v, want the server to stay up", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests sent = %d, want 1", got)
	}
	if !caps.SearchCode || !caps.GetFileContext || !caps.ListProjects {
		t.Fatalf("caps = %+v, want the tools registered so calls can report AUTH_REJECTED", caps)
	}
	joined := strings.Join(logs, "\n")
	for _, want := range []string{"web-UI discovery skipped", "rejected the configured credentials", "AUTH_REJECTED", "restarted"} {
		if !strings.Contains(joined, want) {
			t.Errorf("startup logs should mention %q:\n%s", want, joined)
		}
	}
}

// A bare 401 from the projects API can be OpenGrok's API-token check with valid
// credentials: discovery must still fall back to the web UI and probe normally.
func TestStartupKeepsFallbacksForEndpointOnly401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/projects/indexed":
			http.Error(w, "API token required", http.StatusUnauthorized)
		case "/source/":
			_, _ = w.Write([]byte(`<select id="project" name="project" multiple><option value="platform">platform</option></select>`))
		case "/api/v1/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"resultCount":0,"startDocument":0,"endDocument":0,"results":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := authTestConfig(server.URL)
	client := opengrok.NewClient(cfg.OpenGrokAPIBaseURL, server.Client(), opengrokOptions(cfg)...)

	caps, _, err := runStartup(t, &cfg, client)
	if err != nil {
		t.Fatalf("probeCapabilities() error = %v", err)
	}
	if client.AuthRejection() != nil {
		t.Fatalf("AuthRejection() = %v, want nil for an endpoint-only 401", client.AuthRejection())
	}
	if cfg.ProjectSource != config.ProjectSourceScraped || !caps.SearchCode {
		t.Fatalf("project source = %s, caps = %+v; want the web-UI fallback and search enabled", cfg.ProjectSource, caps)
	}
}
