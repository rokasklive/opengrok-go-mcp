// SPDX-License-Identifier: Apache-2.0

package config

import (
	"flag"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Transport != TransportStdio {
		t.Fatalf("Transport = %q, want %q", cfg.Transport, TransportStdio)
	}
	if cfg.ToolSurface != ToolSurfaceFull {
		t.Fatalf("ToolSurface = %q, want %q", cfg.ToolSurface, ToolSurfaceFull)
	}
	if cfg.Listen != "127.0.0.1:8765" {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:8765")
	}
	if !cfg.ProjectRequired {
		t.Fatal("ProjectRequired = false, want true")
	}
	if cfg.PageSizeDefault != 20 {
		t.Fatalf("PageSizeDefault = %d, want %d", cfg.PageSizeDefault, 20)
	}
	if cfg.PageSizeMax != 100 {
		t.Fatalf("PageSizeMax = %d, want %d", cfg.PageSizeMax, 100)
	}
	if !cfg.IncludeLinksDefault {
		t.Fatal("IncludeLinksDefault = false, want true")
	}
	if !cfg.EnableRawLinks {
		t.Fatal("EnableRawLinks = false, want true")
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %s, want %s", cfg.ReadTimeout, 10*time.Second)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %s, want %s", cfg.WriteTimeout, 10*time.Second)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.Debug {
		t.Fatal("Debug = true, want false")
	}
	if cfg.OpenGrokAPIToken != "" {
		t.Fatal("OpenGrokAPIToken is non-empty, want empty")
	}
	if cfg.OpenGrokBasicAuthToken != "" {
		t.Fatal("OpenGrokBasicAuthToken is non-empty, want empty")
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("Projects = %#v, want empty", cfg.Projects)
	}
	if !cfg.Capabilities.ListProjects || !cfg.Capabilities.SearchCode ||
		!cfg.Capabilities.SearchSymbolDefinitions || !cfg.Capabilities.SearchSymbolReferences ||
		!cfg.Capabilities.GetFileContext {
		t.Fatalf("Capabilities = %#v, want all enabled", cfg.Capabilities)
	}
	if cfg.Capabilities.ListFiles {
		t.Fatal("ListFiles = true, want false (not yet probed)")
	}
	if !cfg.Capabilities.Memory {
		t.Fatal("Memory = false, want local memory capability enabled by default")
	}
}

func TestRegisterFlagsOverridesConfig(t *testing.T) {
	cfg := Default()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	if err := cfg.RegisterFlags(fs); err != nil {
		t.Fatalf("RegisterFlags() error = %v", err)
	}

	err := fs.Parse([]string{
		"--listen", "0.0.0.0:9000",
		"--transport", "http",
		"--base-url", "http://localhost:8080/api",
		"--web-base-url", "http://localhost:8080/source",
		"--default-project", "demo",
		"--project-required=false",
		"--read-timeout", "15s",
		"--write-timeout", "20s",
		"--log-level", "debug",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Listen != "0.0.0.0:9000" {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, "0.0.0.0:9000")
	}
	if cfg.Transport != TransportHTTP {
		t.Fatalf("Transport = %q, want %q", cfg.Transport, TransportHTTP)
	}
	if cfg.OpenGrokAPIBaseURL != "http://localhost:8080/api" {
		t.Fatalf("OpenGrokAPIBaseURL = %q, want %q", cfg.OpenGrokAPIBaseURL, "http://localhost:8080/api")
	}
	if cfg.OpenGrokWebBaseURL != "http://localhost:8080/source" {
		t.Fatalf("OpenGrokWebBaseURL = %q, want %q", cfg.OpenGrokWebBaseURL, "http://localhost:8080/source")
	}
	if cfg.DefaultProject != "demo" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "demo")
	}
	if cfg.ProjectRequired {
		t.Fatal("ProjectRequired = true, want false")
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Fatalf("ReadTimeout = %s, want %s", cfg.ReadTimeout, 15*time.Second)
	}
	if cfg.WriteTimeout != 20*time.Second {
		t.Fatalf("WriteTimeout = %s, want %s", cfg.WriteTimeout, 20*time.Second)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestRegisterFlagsDoesNotExposeAuthTokenFlags(t *testing.T) {
	cfg := Default()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	if err := cfg.RegisterFlags(fs); err != nil {
		t.Fatalf("RegisterFlags() error = %v", err)
	}

	for _, name := range []string{"api-token", "basic-auth-token"} {
		if fs.Lookup(name) != nil {
			t.Fatalf("flag %q exists, want absent", name)
		}
	}
}

func TestFromEnvAppliesSupportedEnvVars(t *testing.T) {
	t.Setenv("OPENGROK_MCP_LISTEN", "0.0.0.0:9000")
	t.Setenv("OPENGROK_MCP_TRANSPORT", "http")
	t.Setenv("OPENGROK_MCP_TOOL_SURFACE", "compact")
	t.Setenv("OPENGROK_MCP_BASE_URL", "http://localhost:8080/api")
	t.Setenv("OPENGROK_MCP_WEB_BASE_URL", "http://localhost:8080/source")
	t.Setenv("OPENGROK_MCP_DEFAULT_PROJECT", "demo")
	t.Setenv("OPENGROK_MCP_LOG_LEVEL", "debug")
	t.Setenv("OPENGROK_MCP_PROJECT_REQUIRED", "false")
	t.Setenv("DEBUG", "1")
	t.Setenv("OPENGROK_MCP_PROJECTS", " platform, tools ,,infra ")
	t.Setenv("OPENGROK_MCP_PROBE_FILE", "platform/src/Engine.swift")

	cfg := FromEnv()

	if cfg.Listen != "0.0.0.0:9000" {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, "0.0.0.0:9000")
	}
	if cfg.Transport != TransportHTTP {
		t.Fatalf("Transport = %q, want %q", cfg.Transport, TransportHTTP)
	}
	if cfg.ToolSurface != ToolSurfaceCompact {
		t.Fatalf("ToolSurface = %q, want %q", cfg.ToolSurface, ToolSurfaceCompact)
	}
	if cfg.OpenGrokAPIBaseURL != "http://localhost:8080/api" {
		t.Fatalf("OpenGrokAPIBaseURL = %q, want %q", cfg.OpenGrokAPIBaseURL, "http://localhost:8080/api")
	}
	if cfg.OpenGrokWebBaseURL != "http://localhost:8080/source" {
		t.Fatalf("OpenGrokWebBaseURL = %q, want %q", cfg.OpenGrokWebBaseURL, "http://localhost:8080/source")
	}
	if cfg.DefaultProject != "demo" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "demo")
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.ProjectRequired {
		t.Fatal("ProjectRequired = true, want false")
	}
	if !cfg.Debug {
		t.Fatal("Debug = false, want true")
	}
	wantProjects := []string{"platform", "tools", "infra"}
	if len(cfg.Projects) != len(wantProjects) {
		t.Fatalf("Projects = %#v, want %#v", cfg.Projects, wantProjects)
	}
	for i, want := range wantProjects {
		if cfg.Projects[i] != want {
			t.Fatalf("Projects[%d] = %q, want %q", i, cfg.Projects[i], want)
		}
	}
	if cfg.ProbeFile != "platform/src/Engine.swift" {
		t.Fatalf("ProbeFile = %q, want probe file", cfg.ProbeFile)
	}
}

func TestFromEnvAppliesAuthTokenEnvVars(t *testing.T) {
	tests := []struct {
		name      string
		envName   string
		envValue  string
		assertion func(*testing.T, Config)
	}{
		{
			name:     "API token",
			envName:  "OPENGROK_MCP_API_TOKEN",
			envValue: "api-token-value",
			assertion: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.OpenGrokAPIToken != "api-token-value" {
					t.Fatalf("OpenGrokAPIToken = %q, want %q", cfg.OpenGrokAPIToken, "api-token-value")
				}
				if cfg.OpenGrokBasicAuthToken != "" {
					t.Fatal("OpenGrokBasicAuthToken is non-empty, want empty")
				}
			},
		},
		{
			name:     "Basic auth token",
			envName:  "OPENGROK_MCP_BASIC_AUTH_TOKEN",
			envValue: "basic-token-value",
			assertion: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.OpenGrokBasicAuthToken != "basic-token-value" {
					t.Fatalf("OpenGrokBasicAuthToken = %q, want %q", cfg.OpenGrokBasicAuthToken, "basic-token-value")
				}
				if cfg.OpenGrokAPIToken != "" {
					t.Fatal("OpenGrokAPIToken is non-empty, want empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envName, tt.envValue)

			cfg := FromEnv()

			tt.assertion(t, cfg)
		})
	}
}

func TestFromEnvIgnoresInvalidProjectRequired(t *testing.T) {
	t.Setenv("OPENGROK_MCP_PROJECT_REQUIRED", "definitely")

	cfg := FromEnv()

	if !cfg.ProjectRequired {
		t.Fatal("ProjectRequired = false, want true")
	}
}

func TestValidateRequiresBaseURLs(t *testing.T) {
	cfg := Default()

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}

	cfg.OpenGrokAPIBaseURL = "http://localhost:8080/api"
	cfg.OpenGrokWebBaseURL = "http://localhost:8080/source"
	cfg.DefaultProject = "platform"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateDerivesWebBaseURLFromAPIBaseURL(t *testing.T) {
	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.DefaultProject = "platform"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if cfg.OpenGrokWebBaseURL != "https://grok.example.com/source" {
		t.Fatalf("OpenGrokWebBaseURL = %q, want derived source URL", cfg.OpenGrokWebBaseURL)
	}
}

func TestValidateDerivesDefaultProjectFromSingleConfiguredProject(t *testing.T) {
	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.Projects = []string{"platform"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if cfg.DefaultProject != "platform" {
		t.Fatalf("DefaultProject = %q, want platform", cfg.DefaultProject)
	}
}

func TestValidateRejectsMultipleProjectsWithoutDefaultProject(t *testing.T) {
	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.Projects = []string{"platform", "infra"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "OPENGROK_MCP_DEFAULT_PROJECT") {
		t.Fatalf("Validate() error = %v, want default project guidance", err)
	}
}

func TestValidateRejectsDerivedWebBaseURLWithoutAPIv1Suffix(t *testing.T) {
	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source"
	cfg.DefaultProject = "platform"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "OPENGROK_MCP_WEB_BASE_URL") {
		t.Fatalf("Validate() error = %v, want explicit web base URL guidance", err)
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	valid := Default()
	valid.OpenGrokAPIBaseURL = "http://localhost:8080/api"
	valid.OpenGrokWebBaseURL = "http://localhost:8080/source"
	valid.DefaultProject = "platform"

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "empty Listen in HTTP mode",
			mutate: func(cfg *Config) {
				cfg.Transport = TransportHTTP
				cfg.Listen = ""
			},
		},
		{
			name: "unsupported transport",
			mutate: func(cfg *Config) {
				cfg.Transport = "websocket"
			},
		},
		{
			name: "unsupported tool surface",
			mutate: func(cfg *Config) {
				cfg.ToolSurface = "wide"
			},
		},
		{
			name: "empty OpenGrokAPIBaseURL",
			mutate: func(cfg *Config) {
				cfg.OpenGrokAPIBaseURL = ""
			},
		},
		{
			name: "empty OpenGrokWebBaseURL",
			mutate: func(cfg *Config) {
				cfg.OpenGrokWebBaseURL = ""
			},
		},
		{
			name: "PageSizeDefault below minimum",
			mutate: func(cfg *Config) {
				cfg.PageSizeDefault = 0
			},
		},
		{
			name: "PageSizeMax below PageSizeDefault",
			mutate: func(cfg *Config) {
				cfg.PageSizeDefault = 20
				cfg.PageSizeMax = 19
			},
		},
		{
			name: "both auth tokens set",
			mutate: func(cfg *Config) {
				cfg.OpenGrokAPIToken = "api-token-value"
				cfg.OpenGrokBasicAuthToken = "basic-token-value"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}

func TestValidateAllowsSingleAuthToken(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "API token only",
			mutate: func(cfg *Config) {
				cfg.OpenGrokAPIToken = "api-token-value"
			},
		},
		{
			name: "Basic auth token only",
			mutate: func(cfg *Config) {
				cfg.OpenGrokBasicAuthToken = "basic-token-value"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.OpenGrokAPIBaseURL = "http://localhost:8080/api"
			cfg.OpenGrokWebBaseURL = "http://localhost:8080/source"
			cfg.DefaultProject = "platform"
			tt.mutate(&cfg)

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestDefaultAutoExpandContext(t *testing.T) {
	cfg := Default()
	if !cfg.AutoExpandContext {
		t.Fatal("AutoExpandContext default = false, want true")
	}
	if cfg.ContextBefore != 5 {
		t.Fatalf("ContextBefore default = %d, want 5", cfg.ContextBefore)
	}
	if cfg.ContextAfter != 10 {
		t.Fatalf("ContextAfter default = %d, want 10", cfg.ContextAfter)
	}
	if cfg.MaxExpandedResults != 10 {
		t.Fatalf("MaxExpandedResults default = %d, want 10", cfg.MaxExpandedResults)
	}
	if cfg.MaxExpandedFiles != 5 {
		t.Fatalf("MaxExpandedFiles default = %d, want 5", cfg.MaxExpandedFiles)
	}
	if cfg.ContextFetchConcurrency != 3 {
		t.Fatalf("ContextFetchConcurrency default = %d, want 3", cfg.ContextFetchConcurrency)
	}
	if cfg.RetryMaxAttempts != 2 {
		t.Fatalf("RetryMaxAttempts default = %d, want 2", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBaseDelay != 200*time.Millisecond {
		t.Fatalf("RetryBaseDelay default = %s, want %s", cfg.RetryBaseDelay, 200*time.Millisecond)
	}
}

func TestFromEnvAutoExpandContextFalse(t *testing.T) {
	t.Setenv("OPENGROK_MCP_AUTO_EXPAND_CONTEXT", "false")
	cfg := FromEnv()
	if cfg.AutoExpandContext {
		t.Fatal("AutoExpandContext = true, want false when env var is false")
	}
}

func TestFromEnvContextWindow(t *testing.T) {
	t.Setenv("OPENGROK_MCP_CONTEXT_BEFORE", "3")
	t.Setenv("OPENGROK_MCP_CONTEXT_AFTER", "7")
	cfg := FromEnv()
	if cfg.ContextBefore != 3 {
		t.Fatalf("ContextBefore = %d, want 3", cfg.ContextBefore)
	}
	if cfg.ContextAfter != 7 {
		t.Fatalf("ContextAfter = %d, want 7", cfg.ContextAfter)
	}
}

func TestFromEnvExpansionBudgets(t *testing.T) {
	t.Setenv("OPENGROK_MCP_MAX_EXPANDED_RESULTS", "7")
	t.Setenv("OPENGROK_MCP_MAX_EXPANDED_FILES", "3")
	t.Setenv("OPENGROK_MCP_CONTEXT_FETCH_CONCURRENCY", "2")

	cfg := FromEnv()

	if cfg.MaxExpandedResults != 7 {
		t.Fatalf("MaxExpandedResults = %d, want 7", cfg.MaxExpandedResults)
	}
	if cfg.MaxExpandedFiles != 3 {
		t.Fatalf("MaxExpandedFiles = %d, want 3", cfg.MaxExpandedFiles)
	}
	if cfg.ContextFetchConcurrency != 2 {
		t.Fatalf("ContextFetchConcurrency = %d, want 2", cfg.ContextFetchConcurrency)
	}
}

func TestFromEnvRetryDefaults(t *testing.T) {
	cfg := FromEnv()
	if cfg.RetryMaxAttempts != 2 {
		t.Fatalf("RetryMaxAttempts = %d, want default 2", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBaseDelay != 200*time.Millisecond {
		t.Fatalf("RetryBaseDelay = %s, want default 200ms", cfg.RetryBaseDelay)
	}
}

func TestFromEnvRetryOverrides(t *testing.T) {
	t.Setenv("OPENGROK_MCP_RETRY_MAX_ATTEMPTS", "5")
	t.Setenv("OPENGROK_MCP_RETRY_BASE_DELAY", "1s")
	cfg := FromEnv()
	if cfg.RetryMaxAttempts != 5 {
		t.Fatalf("RetryMaxAttempts = %d, want 5", cfg.RetryMaxAttempts)
	}
	if cfg.RetryBaseDelay != time.Second {
		t.Fatalf("RetryBaseDelay = %s, want 1s", cfg.RetryBaseDelay)
	}
}

func TestFromEnvRetryIgnoresInvalidAttempts(t *testing.T) {
	t.Setenv("OPENGROK_MCP_RETRY_MAX_ATTEMPTS", "-1")
	cfg := FromEnv()
	if cfg.RetryMaxAttempts != 2 {
		t.Fatalf("RetryMaxAttempts = %d, want default 2", cfg.RetryMaxAttempts)
	}
}

func TestFromEnvRetryIgnoresInvalidDelay(t *testing.T) {
	t.Setenv("OPENGROK_MCP_RETRY_BASE_DELAY", "not-a-duration")
	cfg := FromEnv()
	if cfg.RetryBaseDelay != 200*time.Millisecond {
		t.Fatalf("RetryBaseDelay = %s, want default 200ms", cfg.RetryBaseDelay)
	}
}

func TestFromEnvDisablesMemoryCapability(t *testing.T) {
	t.Setenv("OPENGROK_MCP_MEMORY_ENABLED", "false")

	cfg := FromEnv()

	if cfg.Capabilities.Memory {
		t.Fatal("Memory = true, want disabled from OPENGROK_MCP_MEMORY_ENABLED")
	}
}

func TestValidateAllowsEmptyProjectsAndDefaultProjectForDeferredDiscovery(t *testing.T) {
	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.Projects = nil
	cfg.DefaultProject = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil (deferred default-project check)", err)
	}
}

func TestValidateStillRejectsMultipleProjectsWithoutDefaultProject(t *testing.T) {
	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"
	cfg.Projects = []string{"alpha", "beta"}
	cfg.DefaultProject = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "OPENGROK_MCP_DEFAULT_PROJECT") {
		t.Fatalf("Validate() error = %v, want default project guidance", err)
	}
}

func TestFromEnvProjectScrapeBoolConvention(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true", value: "true", want: true},
		{name: "TRUE", value: "TRUE", want: true},
		{name: "1", value: "1", want: true},
		{name: "t", value: "t", want: true},
		{name: "false", value: "false", want: false},
		{name: "0", value: "0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENGROK_MCP_PROJECT_SCRAPE", tt.value)
			cfg := FromEnv()
			if cfg.ProjectScrapeEnabled != tt.want {
				t.Fatalf("ProjectScrapeEnabled = %t, want %t", cfg.ProjectScrapeEnabled, tt.want)
			}
		})
	}
}
