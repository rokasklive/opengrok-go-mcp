// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

const (
	ToolSurfaceFull    = "full"
	ToolSurfaceCompact = "compact"
	ToolSurfaceGateway = "gateway"
)

const (
	ContextBudgetMinimal = "minimal"
	ContextBudgetDefault = "default"
	ContextBudgetMaximal = "maximal"
)

const (
	ProjectSourceConfigured = "configured"
	ProjectSourceAPI        = "api"
	ProjectSourceScraped    = "scraped"
	ProjectSourceNone       = "none"
)

type Capabilities struct {
	ListProjects            bool
	SearchCode              bool
	SearchSymbolDefinitions bool
	SearchSymbolReferences  bool
	GetFileContext          bool
	ListSymbols             bool
	ListFiles               bool
	ServerSideSort          bool
	Memory                  bool
}

type BudgetValues struct {
	ContextBefore      int
	ContextAfter       int
	MaxExpandedResults int
	MaxExpandedFiles   int
}

type BudgetTiers struct {
	Minimal BudgetValues
	Default BudgetValues
	Maximal BudgetValues
}

type Config struct {
	Transport               string
	ToolSurface             string
	Debug                   bool
	Listen                  string
	OpenGrokAPIBaseURL      string
	OpenGrokWebBaseURL      string
	OpenGrokAPIToken        string
	OpenGrokBasicAuthToken  string
	Projects                []string
	ProjectSource           string
	ProjectScrapeEnabled    bool
	ProbeFile               string
	DefaultProject          string
	ProjectRequired         bool
	Capabilities            Capabilities
	PageSizeDefault         int
	PageSizeMax             int
	IncludeLinksDefault     bool
	EnableRawLinks          bool
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	LogLevel                string
	InsecureSkipTLSVerify   bool
	AutoExpandContext       bool
	ContextBefore           int
	ContextAfter            int
	MaxExpandedResults      int
	MaxExpandedFiles        int
	ContextFetchConcurrency int
	RetryMaxAttempts        int
	RetryBaseDelay          time.Duration
	CursorSecret            string
	CacheEnabled            bool
	CacheTTL                time.Duration
	CacheMaxSize            int
	BudgetTiers             BudgetTiers
}

func Default() Config {
	return Config{
		Transport:       TransportStdio,
		ToolSurface:     ToolSurfaceFull,
		Listen:          "127.0.0.1:8765",
		ProjectRequired: true,
		Capabilities: Capabilities{
			ListProjects:            true,
			SearchCode:              true,
			SearchSymbolDefinitions: true,
			SearchSymbolReferences:  true,
			GetFileContext:          true,
			Memory:                  true,
		},
		PageSizeDefault:         20,
		PageSizeMax:             100,
		IncludeLinksDefault:     true,
		EnableRawLinks:          true,
		ReadTimeout:             10 * time.Second,
		WriteTimeout:            10 * time.Second,
		LogLevel:                "info",
		AutoExpandContext:       true,
		ContextBefore:           5,
		ContextAfter:            10,
		MaxExpandedResults:      10,
		MaxExpandedFiles:        5,
		ContextFetchConcurrency: 3,
		RetryMaxAttempts:        2,
		RetryBaseDelay:          200 * time.Millisecond,
		CacheEnabled:            false,
		CacheTTL:                5 * time.Minute,
		CacheMaxSize:            1000,
		BudgetTiers: BudgetTiers{
			Minimal: BudgetValues{
				ContextBefore:      2,
				ContextAfter:       3,
				MaxExpandedResults: 3,
				MaxExpandedFiles:   2,
			},
			Default: BudgetValues{
				ContextBefore:      5,
				ContextAfter:       10,
				MaxExpandedResults: 10,
				MaxExpandedFiles:   5,
			},
			Maximal: BudgetValues{
				ContextBefore:      15,
				ContextAfter:       30,
				MaxExpandedResults: 25,
				MaxExpandedFiles:   10,
			},
		},
	}
}

func FromEnv() Config {
	cfg := Default()

	if value := os.Getenv("OPENGROK_MCP_LISTEN"); value != "" {
		cfg.Listen = value
	}
	if value := os.Getenv("OPENGROK_MCP_TRANSPORT"); value != "" {
		cfg.Transport = strings.ToLower(value)
	}
	if value := os.Getenv("OPENGROK_MCP_TOOL_SURFACE"); value != "" {
		cfg.ToolSurface = strings.ToLower(value)
	}
	if value := os.Getenv("OPENGROK_MCP_MEMORY_ENABLED"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Capabilities.Memory = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_BASE_URL"); value != "" {
		cfg.OpenGrokAPIBaseURL = value
	}
	if value := os.Getenv("OPENGROK_MCP_WEB_BASE_URL"); value != "" {
		cfg.OpenGrokWebBaseURL = value
	}
	if value := os.Getenv("OPENGROK_MCP_API_TOKEN"); value != "" {
		cfg.OpenGrokAPIToken = value
	}
	if value := os.Getenv("OPENGROK_MCP_BASIC_AUTH_TOKEN"); value != "" {
		cfg.OpenGrokBasicAuthToken = value
	}
	if value := os.Getenv("OPENGROK_MCP_PROJECTS"); value != "" {
		cfg.Projects = splitCSV(value)
	}
	if value := os.Getenv("OPENGROK_MCP_PROBE_FILE"); value != "" {
		cfg.ProbeFile = value
	}
	if value := os.Getenv("OPENGROK_MCP_DEFAULT_PROJECT"); value != "" {
		cfg.DefaultProject = value
	}
	if value := os.Getenv("OPENGROK_MCP_LOG_LEVEL"); value != "" {
		cfg.LogLevel = value
	}
	if value := os.Getenv("OPENGROK_MCP_PROJECT_REQUIRED"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.ProjectRequired = parsed
		}
	}
	if value := os.Getenv("DEBUG"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Debug = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY"); value != "" {
		cfg.InsecureSkipTLSVerify = parseBoolEnv(value, cfg.InsecureSkipTLSVerify)
	}
	if value := os.Getenv("OPENGROK_MCP_PROJECT_SCRAPE"); value != "" {
		cfg.ProjectScrapeEnabled = parseBoolEnv(value, cfg.ProjectScrapeEnabled)
	}
	if value := os.Getenv("OPENGROK_MCP_AUTO_EXPAND_CONTEXT"); value != "" {
		cfg.AutoExpandContext = parseBoolEnv(value, cfg.AutoExpandContext)
	}
	if value := os.Getenv("OPENGROK_MCP_CONTEXT_BEFORE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			cfg.ContextBefore = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_CONTEXT_AFTER"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			cfg.ContextAfter = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_MAX_EXPANDED_RESULTS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			cfg.MaxExpandedResults = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_MAX_EXPANDED_FILES"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			cfg.MaxExpandedFiles = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_CONTEXT_FETCH_CONCURRENCY"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.ContextFetchConcurrency = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_RETRY_MAX_ATTEMPTS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.RetryMaxAttempts = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_RETRY_BASE_DELAY"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			cfg.RetryBaseDelay = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_CURSOR_SECRET"); value != "" {
		cfg.CursorSecret = value
	}
	if value := os.Getenv("OPENGROK_MCP_CACHE_ENABLED"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.CacheEnabled = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_CACHE_TTL"); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			cfg.CacheTTL = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_CACHE_MAX_SIZE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.CacheMaxSize = parsed
		}
	}

	cfg.BudgetTiers.Minimal = parseBudgetTierEnv("MINIMAL", cfg.BudgetTiers.Minimal)
	cfg.BudgetTiers.Default = parseBudgetTierEnv("DEFAULT", cfg.BudgetTiers.Default)
	cfg.BudgetTiers.Maximal = parseBudgetTierEnv("MAXIMAL", cfg.BudgetTiers.Maximal)

	return cfg
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) error {
	if fs == nil {
		return errors.New("flag set is nil")
	}

	fs.StringVar(&c.Listen, "listen", c.Listen, "address for the MCP server to listen on")
	fs.StringVar(&c.Transport, "transport", c.Transport, "MCP transport: stdio or http")
	fs.StringVar(&c.OpenGrokAPIBaseURL, "base-url", c.OpenGrokAPIBaseURL, "OpenGrok API base URL")
	fs.StringVar(&c.OpenGrokWebBaseURL, "web-base-url", c.OpenGrokWebBaseURL, "OpenGrok web base URL")
	fs.StringVar(&c.DefaultProject, "default-project", c.DefaultProject, "default OpenGrok project")
	fs.BoolVar(&c.ProjectRequired, "project-required", c.ProjectRequired, "require project parameter")
	fs.DurationVar(&c.ReadTimeout, "read-timeout", c.ReadTimeout, "server read timeout")
	fs.DurationVar(&c.WriteTimeout, "write-timeout", c.WriteTimeout, "server write timeout")
	fs.StringVar(&c.LogLevel, "log-level", c.LogLevel, "log level")

	return nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func deriveWebBaseURL(apiBaseURL string) string {
	trimmed := strings.TrimRight(apiBaseURL, "/")
	return strings.TrimSuffix(trimmed, "/api/v1")
}

func (c *Config) applyDerivedDefaults() {
	if c.OpenGrokWebBaseURL == "" && c.OpenGrokAPIBaseURL != "" {
		c.OpenGrokWebBaseURL = deriveWebBaseURL(c.OpenGrokAPIBaseURL)
	}
	if c.DefaultProject == "" && len(c.Projects) == 1 {
		c.DefaultProject = c.Projects[0]
	}
}

func (c *Config) Validate() error {
	c.Transport = strings.ToLower(c.Transport)
	switch c.Transport {
	case TransportStdio:
	case TransportHTTP:
		if c.Listen == "" {
			return errors.New("listen address is required")
		}
	default:
		return fmt.Errorf("unsupported transport %q", c.Transport)
	}
	c.ToolSurface = strings.ToLower(c.ToolSurface)
	switch c.ToolSurface {
	case ToolSurfaceFull, ToolSurfaceCompact, ToolSurfaceGateway:
	default:
		return fmt.Errorf("unsupported tool surface %q", c.ToolSurface)
	}
	if c.OpenGrokAPIBaseURL == "" {
		return errors.New("OpenGrok API base URL is required")
	}
	if c.OpenGrokWebBaseURL == "" {
		trimmedAPIBaseURL := strings.TrimRight(c.OpenGrokAPIBaseURL, "/")
		if !strings.HasSuffix(trimmedAPIBaseURL, "/api/v1") {
			return errors.New("OPENGROK_MCP_WEB_BASE_URL is required when OPENGROK_MCP_BASE_URL does not end in /api/v1")
		}
	}
	c.applyDerivedDefaults()
	if c.OpenGrokWebBaseURL == "" {
		return errors.New("OpenGrok web base URL is required")
	}
	// A missing default project is only a hard error at config-parse time when
	// multiple projects are explicitly configured. The single-project and
	// empty-projects cases are deferred to post-discovery resolution (FR-013),
	// so fall through here rather than returning — the remaining auth and
	// page-size checks below must still run for those configs.
	if c.DefaultProject == "" && len(c.Projects) > 1 {
		return errors.New("OPENGROK_MCP_DEFAULT_PROJECT is required when multiple OPENGROK_MCP_PROJECTS are configured")
	}
	if c.OpenGrokAPIToken != "" && c.OpenGrokBasicAuthToken != "" {
		return errors.New("only one OpenGrok auth token may be configured")
	}
	if c.PageSizeDefault < 1 {
		return fmt.Errorf("page size default must be at least 1: %d", c.PageSizeDefault)
	}
	if c.PageSizeMax < c.PageSizeDefault {
		return fmt.Errorf(
			"page size max must be greater than or equal to default: max %d, default %d",
			c.PageSizeMax,
			c.PageSizeDefault,
		)
	}

	return nil
}

func parseBudgetTierEnv(tier string, defaults BudgetValues) BudgetValues {
	result := defaults
	if value := os.Getenv("OPENGROK_MCP_BUDGET_" + tier + "_BEFORE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			result.ContextBefore = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_BUDGET_" + tier + "_AFTER"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			result.ContextAfter = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_BUDGET_" + tier + "_RESULTS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			result.MaxExpandedResults = parsed
		}
	}
	if value := os.Getenv("OPENGROK_MCP_BUDGET_" + tier + "_FILES"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			result.MaxExpandedFiles = parsed
		}
	}
	return result
}

func parseBoolEnv(value string, defaultValue bool) bool {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
