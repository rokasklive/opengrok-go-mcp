// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestNewHTTPClientSkipVerifyPreservesProxyFromEnvironment(t *testing.T) {
	cfg := config.Default()
	cfg.ReadTimeout = 5 * time.Second
	cfg.InsecureSkipTLSVerify = true

	client := newHTTPClient(cfg)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("Transport.Proxy is nil, want http.ProxyFromEnvironment")
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false, want true when skip-verify is enabled")
	}

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(proxyServer.Close)

	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatalf("Parse proxy URL: %v", err)
	}
	t.Setenv("HTTPS_PROXY", proxyURL.String())
	t.Setenv("HTTP_PROXY", "")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	gotProxy, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy() error = %v", err)
	}
	if gotProxy == nil || gotProxy.Host != proxyURL.Host {
		t.Fatalf("Proxy() = %v, want host %q", gotProxy, proxyURL.Host)
	}
}

func TestNewHTTPClientDefaultTransportWhenSkipVerifyDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.ReadTimeout = 5 * time.Second

	client := newHTTPClient(cfg)
	if client.Transport != nil {
		t.Fatalf("Transport = %v, want nil (default transport)", client.Transport)
	}
	if client.Timeout != cfg.ReadTimeout {
		t.Fatalf("Timeout = %v, want %v", client.Timeout, cfg.ReadTimeout)
	}
}

func TestHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	handler := &noopHandler{}
	readTimeout := 2 * time.Second
	writeTimeout := 3 * time.Second

	server := newHTTPServer("127.0.0.1:0", handler, readTimeout, writeTimeout)

	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q, want %q", server.Addr, "127.0.0.1:0")
	}
	if server.Handler != handler {
		t.Fatal("Handler does not match configured handler")
	}
	if server.ReadTimeout != readTimeout {
		t.Fatalf("ReadTimeout = %v, want %v", server.ReadTimeout, readTimeout)
	}
	if server.WriteTimeout != writeTimeout {
		t.Fatalf("WriteTimeout = %v, want %v", server.WriteTimeout, writeTimeout)
	}
}

type noopHandler struct{}

func (h *noopHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

func TestRunHelpReturnsNil(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() {
		os.Args = originalArgs
	})

	os.Args = []string{"opengrok-go-mcp", "--help"}
	if err := run(); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
}

func TestDetectCapabilitiesFallsBackToDefaultProjectWhenAPIForbidden(t *testing.T) {
	backend := &capabilityBackend{
		listProjectsErr: errors.New("unauthorized"),
		searchResults: map[opengrok.Mode]error{
			opengrok.ModeFullText:   nil,
			opengrok.ModeDefinition: nil,
			opengrok.ModeReference:  nil,
		},
		fileErr: errors.New("unauthorized"),
	}

	cfg := config.Default()
	cfg.DefaultProject = "platform"
	cfg.ProbeFile = "platform/src/Engine.swift"
	caps, err := detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
	if err != nil {
		t.Fatalf("detectCapabilities returned error: %v", err)
	}
	if !caps.ListProjects {
		t.Fatal("ListProjects capability = false, want true (falls back to default project)")
	}
	if !caps.SearchCode || !caps.SearchSymbolDefinitions || !caps.SearchSymbolReferences {
		t.Fatalf("search capabilities = %#v, want all search enabled", caps)
	}
	if caps.GetFileContext {
		t.Fatal("GetFileContext capability = true, want false")
	}
}

func TestDetectCapabilitiesFailsWhenProjectsAndSearchFail(t *testing.T) {
	backend := &capabilityBackend{
		listProjectsErr: errors.New("unauthorized"),
		searchResults: map[opengrok.Mode]error{
			opengrok.ModeFullText:   errors.New("unauthorized"),
			opengrok.ModeDefinition: errors.New("unauthorized"),
			opengrok.ModeReference:  errors.New("unauthorized"),
		},
	}

	_, err := detectCapabilities(context.Background(), backend, config.Default(), func(string, ...any) {})
	if err == nil {
		t.Fatal("detectCapabilities error = nil, want error")
	}
}

func TestDetectCapabilitiesUsesConfiguredProjects(t *testing.T) {
	backend := &capabilityBackend{
		listProjectsErr: errors.New("unauthorized"),
		searchResults: map[opengrok.Mode]error{
			opengrok.ModeFullText: nil,
		},
	}

	cfg := config.Default()
	cfg.Projects = []string{"platform"}
	caps, err := detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
	if err != nil {
		t.Fatalf("detectCapabilities returned error: %v", err)
	}
	if !caps.ListProjects {
		t.Fatal("ListProjects capability = false, want true for configured projects")
	}
	if got := backend.searchRequests[0].Projects; len(got) != 1 || got[0] != "platform" {
		t.Fatalf("search probe projects = %#v, want configured project", got)
	}
}

func TestDetectCapabilitiesEnablesFileContextWithRawFallback(t *testing.T) {
	backend := &capabilityBackend{
		searchResults: map[opengrok.Mode]error{
			opengrok.ModeFullText:   nil,
			opengrok.ModeDefinition: nil,
			opengrok.ModeReference:  nil,
		},
	}

	cfg := config.Default()
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	caps, err := detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
	if err != nil {
		t.Fatalf("detectCapabilities returned error: %v", err)
	}
	if !caps.GetFileContext {
		t.Fatal("GetFileContext capability = false, want true with raw fallback")
	}
}

func TestDetectCapabilitiesProbesListFiles(t *testing.T) {
	t.Run("ListFiles success enables capability", func(t *testing.T) {
		backend := &capabilityBackend{
			searchResults: map[opengrok.Mode]error{
				opengrok.ModeFullText:   nil,
				opengrok.ModeDefinition: nil,
				opengrok.ModeReference:  nil,
			},
		}

		cfg := config.Default()
		cfg.DefaultProject = "platform"
		caps, err := detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
		if err != nil {
			t.Fatalf("detectCapabilities returned error: %v", err)
		}
		if !caps.ListFiles {
			t.Fatal("ListFiles capability = false, want true")
		}
	})

	t.Run("ListFiles failure disables capability", func(t *testing.T) {
		backend := &capabilityBackend{
			listFilesErr: errors.New("unauthorized"),
			searchResults: map[opengrok.Mode]error{
				opengrok.ModeFullText:   nil,
				opengrok.ModeDefinition: nil,
				opengrok.ModeReference:  nil,
			},
		}

		cfg := config.Default()
		cfg.DefaultProject = "platform"
		caps, err := detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
		if err != nil {
			t.Fatalf("detectCapabilities returned error: %v", err)
		}
		if caps.ListFiles {
			t.Fatal("ListFiles capability = true, want false")
		}
	})
}

func TestDetectCapabilitiesPreservesConfiguredMemoryCapability(t *testing.T) {
	backend := &capabilityBackend{
		searchResults: map[opengrok.Mode]error{
			opengrok.ModeFullText:   nil,
			opengrok.ModeDefinition: nil,
			opengrok.ModeReference:  nil,
		},
	}

	cfg := config.Default()
	cfg.DefaultProject = "platform"
	caps, err := detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
	if err != nil {
		t.Fatalf("detectCapabilities returned error: %v", err)
	}
	if !caps.Memory {
		t.Fatal("Memory capability = false, want configured local capability preserved")
	}

	cfg.Capabilities.Memory = false
	caps, err = detectCapabilities(context.Background(), backend, cfg, func(string, ...any) {})
	if err != nil {
		t.Fatalf("detectCapabilities disabled-memory error: %v", err)
	}
	if caps.Memory {
		t.Fatal("Memory capability = true, want configured disable preserved")
	}
}

type capabilityBackend struct {
	listProjectsErr error
	listFilesErr    error
	searchResults   map[opengrok.Mode]error
	searchRequests  []opengrok.SearchRequest
	fileErr         error
}

func (b *capabilityBackend) ListProjects(context.Context) ([]string, error) {
	if b.listProjectsErr != nil {
		return nil, b.listProjectsErr
	}
	return []string{"platform"}, nil
}

func (b *capabilityBackend) Search(_ context.Context, req opengrok.SearchRequest) (opengrok.SearchResult, error) {
	b.searchRequests = append(b.searchRequests, req)
	if err := b.searchResults[req.Mode]; err != nil {
		return opengrok.SearchResult{Hits: []opengrok.Hit{}}, err
	}
	return opengrok.SearchResult{Hits: []opengrok.Hit{}}, nil
}

func (b *capabilityBackend) ListFiles(context.Context, string, string) ([]opengrok.FileEntry, error) {
	if b.listFilesErr != nil {
		return nil, b.listFilesErr
	}
	return []opengrok.FileEntry{}, nil
}

func (b *capabilityBackend) FileContent(context.Context, string, string) (string, error) {
	if b.fileErr != nil {
		return "", b.fileErr
	}
	return "content", nil
}

func TestOpenGrokOptionsBasicAuthWinsWhenBothTokensAreConfigured(t *testing.T) {
	server := authHeaderServer(t, "Basic basic-token-value")
	defer server.Close()

	cfg := config.Config{
		OpenGrokAPIToken:       "api-token-value",
		OpenGrokBasicAuthToken: "basic-token-value",
	}
	client := opengrok.NewClient(
		server.URL+"/api/v1",
		server.Client(),
		opengrokOptions(cfg)...,
	)

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func TestOpenGrokOptionsAPITokenOnlyUsesBearerAuth(t *testing.T) {
	server := authHeaderServer(t, "Bearer api-token-value")
	defer server.Close()

	cfg := config.Config{
		OpenGrokAPIToken: "api-token-value",
	}
	client := opengrok.NewClient(
		server.URL+"/api/v1",
		server.Client(),
		opengrokOptions(cfg)...,
	)

	if _, err := client.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects() error = %v, want nil", err)
	}
}

func TestOpenGrokOptionsConfiguresRawWebFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/file/content":
			http.Error(w, "forbidden", http.StatusUnauthorized)
		case "/source/raw/platform/src/Engine.swift":
			if _, err := w.Write([]byte("content")); err != nil {
				t.Fatalf("Write() error = %v", err)
			}
		default:
			t.Fatalf("path = %q, want API or raw web fallback", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := config.Config{OpenGrokWebBaseURL: server.URL + "/source"}
	client := opengrok.NewClient(
		server.URL+"/api/v1",
		server.Client(),
		opengrokOptions(cfg)...,
	)

	got, err := client.FileContent(context.Background(), "platform", "src/Engine.swift")
	if err != nil {
		t.Fatalf("FileContent() error = %v, want nil", err)
	}
	if got != "content" {
		t.Fatalf("FileContent() = %q, want fallback content", got)
	}
}

func TestStartupLogsDerivedConfig(t *testing.T) {
	t.Setenv("OPENGROK_MCP_WEB_BASE_URL", "")
	t.Setenv("OPENGROK_MCP_DEFAULT_PROJECT", "")
	t.Setenv("OPENGROK_MCP_PROJECTS", "")

	cfg := config.Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	cfg.Projects = []string{"platform"}
	cfg.DefaultProject = "platform"
	cfg.Capabilities = config.Capabilities{
		ListProjects: true,
		SearchCode:   true,
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	logStartupDiagnostics(cfg, "")

	output := buf.String()
	if !strings.Contains(output, "web URL") && !strings.Contains(output, "derived") {
		t.Fatalf("expected log output to contain 'web URL' or 'derived', got: %s", output)
	}
}

func TestStartupLogsEnabledCapabilities(t *testing.T) {
	cfg := config.Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.OpenGrokWebBaseURL = "https://grok.example.com/source"
	cfg.DefaultProject = "platform"
	cfg.Capabilities = config.Capabilities{
		ListProjects:            true,
		SearchCode:              false,
		SearchSymbolDefinitions: true,
		SearchSymbolReferences:  false,
		GetFileContext:          true,
		ListSymbols:             false,
		ListFiles:               true,
		Memory:                  true,
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	logStartupDiagnostics(cfg, "")

	output := buf.String()
	for _, name := range []string{
		"list_projects", "search_code", "search_symbol_definitions",
		"search_symbol_references", "get_file_context", "list_symbols", "list_files", "memory",
	} {
		if !strings.Contains(output, name) {
			t.Fatalf("expected capability name %q in log output, got: %s", name, output)
		}
	}
}

func authHeaderServer(t *testing.T, wantAuth string) *httptest.Server {
	t.Helper()

	errCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/indexed" {
			errCh <- fmt.Errorf("path = %q, want %q", r.URL.Path, "/api/v1/projects/indexed")
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}

		gotAuth := r.Header.Get("Authorization")
		if gotAuth != wantAuth {
			errCh <- fmt.Errorf("Authorization = %q, want %q", gotAuth, wantAuth)
			http.Error(w, "unexpected authorization", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode([]string{}); err != nil {
			errCh <- fmt.Errorf("encode response: %w", err)
			return
		}
	}))

	t.Cleanup(func() {
		select {
		case err := <-errCh:
			t.Error(err)
		default:
		}
	})

	return server
}

func TestClassifyProbeError(t *testing.T) {
	unauthorized := &opengrok.StatusError{Code: http.StatusUnauthorized}
	forbidden := &opengrok.StatusError{Code: http.StatusForbidden}
	badRequest := &opengrok.StatusError{Code: http.StatusBadRequest}

	tlsErr := &tls.CertificateVerificationError{
		UnverifiedCertificates: []*x509.Certificate{{
			DNSNames: []string{"internal.example.com", "*.internal.example.com"},
		}},
	}

	tests := []struct {
		name         string
		err          error
		anyAuthed    bool
		wantCategory string
		wantSANs     []string
	}{
		{
			name:         "401 with prior authed probe",
			err:          fmt.Errorf("probe: %w", unauthorized),
			anyAuthed:    true,
			wantCategory: "endpoint_disabled",
		},
		{
			name:         "403 with prior authed probe",
			err:          fmt.Errorf("probe: %w", forbidden),
			anyAuthed:    true,
			wantCategory: "endpoint_disabled",
		},
		{
			name:         "401 without prior authed probe",
			err:          fmt.Errorf("probe: %w", unauthorized),
			anyAuthed:    false,
			wantCategory: "unauthorized",
		},
		{
			name:         "400 unsupported feature",
			err:          fmt.Errorf("probe: %w", badRequest),
			anyAuthed:    true,
			wantCategory: "feature_unsupported",
		},
		{
			name:         "TLS hostname mismatch",
			err:          fmt.Errorf("dial: %w", tlsErr),
			anyAuthed:    false,
			wantCategory: "tls_mismatch",
			wantSANs:     []string{"internal.example.com", "*.internal.example.com"},
		},
		{
			name:         "generic transport error",
			err:          errors.New("connection reset"),
			anyAuthed:    false,
			wantCategory: "transport_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCategory, gotSANs := classifyProbeError(tt.err, tt.anyAuthed)
			if gotCategory != tt.wantCategory {
				t.Fatalf("category = %q, want %q", gotCategory, tt.wantCategory)
			}
			if !slicesEqual(gotSANs, tt.wantSANs) {
				t.Fatalf("certSANs = %#v, want %#v", gotSANs, tt.wantSANs)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type fakeProjectResolver struct {
	listProjects      []string
	listProjectsErr   error
	scrapeProjects    []string
	scrapeProjectsErr error
	scrapeCalled      bool
	listCalled        bool
}

func (f *fakeProjectResolver) ListProjects(context.Context) ([]string, error) {
	f.listCalled = true
	if f.listProjectsErr != nil {
		return nil, f.listProjectsErr
	}
	return f.listProjects, nil
}

func (f *fakeProjectResolver) ScrapeProjects(context.Context) ([]string, error) {
	f.scrapeCalled = true
	if f.scrapeProjectsErr != nil {
		return nil, f.scrapeProjectsErr
	}
	return f.scrapeProjects, nil
}

func TestResolveProjectAllowlist(t *testing.T) {
	unauthorized := &opengrok.StatusError{Code: http.StatusUnauthorized}

	tests := []struct {
		name             string
		cfg              config.Config
		resolver         *fakeProjectResolver
		wantProjects     []string
		wantSource       string
		wantDefault      string
		wantErr          bool
		wantErrContains  string
		wantScrapeCalled bool
		wantListCalled   bool
	}{
		{
			name: "configured wins without API or scrape",
			cfg: config.Config{
				Projects:       []string{"x", "y"},
				DefaultProject: "x",
			},
			resolver:         &fakeProjectResolver{},
			wantProjects:     []string{"x", "y"},
			wantSource:       config.ProjectSourceConfigured,
			wantDefault:      "x",
			wantScrapeCalled: false,
			wantListCalled:   false,
		},
		{
			name: "configured single project replaces stale default",
			cfg: config.Config{
				Projects:       []string{"only"},
				DefaultProject: "stale",
			},
			resolver:         &fakeProjectResolver{},
			wantProjects:     []string{"only"},
			wantSource:       config.ProjectSourceConfigured,
			wantDefault:      "only",
			wantScrapeCalled: false,
			wantListCalled:   false,
		},
		{
			name: "configured rejects default outside allowlist",
			cfg: config.Config{
				Projects:       []string{"a", "b"},
				DefaultProject: "stale",
			},
			resolver:         &fakeProjectResolver{},
			wantProjects:     []string{"a", "b"},
			wantSource:       config.ProjectSourceConfigured,
			wantErr:          true,
			wantErrContains:  "not in the resolved OpenGrok project allowlist",
			wantScrapeCalled: false,
			wantListCalled:   false,
		},
		{
			name: "api non-empty wins without scrape",
			cfg: config.Config{
				ProjectScrapeEnabled: true,
				DefaultProject:       "a",
			},
			resolver:         &fakeProjectResolver{listProjects: []string{"a", "b"}},
			wantProjects:     []string{"a", "b"},
			wantSource:       config.ProjectSourceAPI,
			wantDefault:      "a",
			wantScrapeCalled: false,
			wantListCalled:   true,
		},
		{
			name: "single api project replaces stale default",
			cfg: config.Config{
				DefaultProject: "stale",
			},
			resolver:       &fakeProjectResolver{listProjects: []string{"only"}},
			wantProjects:   []string{"only"},
			wantSource:     config.ProjectSourceAPI,
			wantDefault:    "only",
			wantListCalled: true,
		},
		{
			name: "api rejects default outside resolved allowlist",
			cfg: config.Config{
				DefaultProject: "stale",
			},
			resolver:        &fakeProjectResolver{listProjects: []string{"a", "b"}},
			wantProjects:    []string{"a", "b"},
			wantSource:      config.ProjectSourceAPI,
			wantErr:         true,
			wantErrContains: "not in the resolved OpenGrok project allowlist",
			wantListCalled:  true,
		},
		{
			name: "api error with scrape on yields scraped",
			cfg: config.Config{
				ProjectScrapeEnabled: true,
				DefaultProject:       "s1",
			},
			resolver: &fakeProjectResolver{
				listProjectsErr: unauthorized,
				scrapeProjects:  []string{"s1", "s2"},
			},
			wantProjects:     []string{"s1", "s2"},
			wantSource:       config.ProjectSourceScraped,
			wantDefault:      "s1",
			wantScrapeCalled: true,
			wantListCalled:   true,
		},
		{
			name: "scrape rejects default outside resolved allowlist",
			cfg: config.Config{
				ProjectScrapeEnabled: true,
				DefaultProject:       "stale",
			},
			resolver: &fakeProjectResolver{
				listProjectsErr: unauthorized,
				scrapeProjects:  []string{"s1", "s2"},
			},
			wantProjects:     []string{"s1", "s2"},
			wantSource:       config.ProjectSourceScraped,
			wantErr:          true,
			wantErrContains:  "not in the resolved OpenGrok project allowlist",
			wantScrapeCalled: true,
			wantListCalled:   true,
		},
		{
			name: "api error with scrape off yields none",
			cfg:  config.Config{},
			resolver: &fakeProjectResolver{
				listProjectsErr: unauthorized,
			},
			wantProjects:     nil,
			wantSource:       config.ProjectSourceNone,
			wantScrapeCalled: false,
			wantListCalled:   true,
			wantErr:          true,
			wantErrContains:  "OPENGROK_MCP_DEFAULT_PROJECT",
		},
		{
			name: "empty api with scrape on yields scraped",
			cfg: config.Config{
				ProjectScrapeEnabled: true,
				DefaultProject:       "a",
			},
			resolver:         &fakeProjectResolver{scrapeProjects: []string{"a", "b", "c"}},
			wantProjects:     []string{"a", "b", "c"},
			wantSource:       config.ProjectSourceScraped,
			wantDefault:      "a",
			wantScrapeCalled: true,
			wantListCalled:   true,
		},
		{
			name:             "empty api with scrape off yields none",
			cfg:              config.Config{},
			resolver:         &fakeProjectResolver{listProjects: []string{}},
			wantProjects:     nil,
			wantSource:       config.ProjectSourceNone,
			wantScrapeCalled: false,
			wantListCalled:   true,
			wantErr:          true,
			wantErrContains:  "OPENGROK_MCP_DEFAULT_PROJECT",
		},
		{
			name:             "single resolved project sets default",
			cfg:              config.Config{ProjectScrapeEnabled: true},
			resolver:         &fakeProjectResolver{scrapeProjects: []string{"only"}},
			wantProjects:     []string{"only"},
			wantSource:       config.ProjectSourceScraped,
			wantDefault:      "only",
			wantScrapeCalled: true,
			wantListCalled:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			resolver := tt.resolver
			err := resolveProjectAllowlist(context.Background(), &cfg, resolver, func(string, ...any) {})
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolveProjectAllowlist() error = nil, want error")
				}
			} else if err != nil {
				t.Fatalf("resolveProjectAllowlist() error = %v", err)
			}
			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf("resolveProjectAllowlist() error = nil, want error containing %q", tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("resolveProjectAllowlist() error = %q, want containing %q", err, tt.wantErrContains)
				}
			}
			if !slicesEqual(cfg.Projects, tt.wantProjects) {
				t.Fatalf("Projects = %#v, want %#v", cfg.Projects, tt.wantProjects)
			}
			if cfg.ProjectSource != tt.wantSource {
				t.Fatalf("ProjectSource = %q, want %q", cfg.ProjectSource, tt.wantSource)
			}
			if tt.wantDefault != "" && cfg.DefaultProject != tt.wantDefault {
				t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, tt.wantDefault)
			}
			if resolver.scrapeCalled != tt.wantScrapeCalled {
				t.Fatalf("scrapeCalled = %t, want %t", resolver.scrapeCalled, tt.wantScrapeCalled)
			}
			if resolver.listCalled != tt.wantListCalled {
				t.Fatalf("listCalled = %t, want %t", resolver.listCalled, tt.wantListCalled)
			}
		})
	}
}
