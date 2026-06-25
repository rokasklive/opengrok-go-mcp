// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/cache"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/cursor"
	"github.com/rokasklive/opengrok-go-mcp/internal/mcpserver"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

// Set at link time by GoReleaser (-X main.version={{.Tag}}); "dev" for local builds.
var version = "dev"

const authRemediationLog = "OpenGrok returned unauthorized responses and no auth token is configured. " +
	"Set OPENGROK_MCP_API_TOKEN to \"Bearer <token>\" or \"Basic <credentials>\" and restart."

func main() {
	if err := run(); err != nil {
		log.Printf("opengrok-go-mcp: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.FromEnv()

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	if err := cfg.RegisterFlags(fs); err != nil {
		return fmt.Errorf("register flags: %w", err)
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	cursor.Secret = cfg.CursorSecret
	if cursor.Secret == "" {
		log.Printf("WARNING: cursor signing disabled; set OPENGROK_MCP_CURSOR_SECRET for integrity")
	}

	httpClient := newHTTPClient(cfg)
	rawClient := opengrok.NewClient(
		cfg.OpenGrokAPIBaseURL,
		httpClient,
		opengrokOptions(cfg)...,
	)

	checkCtx, cancel := context.WithTimeout(context.Background(), cfg.ReadTimeout)
	defer cancel()
	if err := resolveProjectAllowlist(checkCtx, &cfg, rawClient, log.Printf); err != nil {
		return err
	}
	// The client was constructed before discovery; apply the resolved default
	// project so result-path attribution matches the resolved snapshot.
	rawClient.SetDefaultProject(cfg.DefaultProject)

	var backend mcpserver.Backend = rawClient

	var cacheStats string
	if cfg.CacheEnabled {
		cacheInstance := cache.New(cfg.CacheTTL)
		backend = mcpserver.NewCachingBackend(backend, cacheInstance, cfg.CacheMaxSize)
		cacheStats = fmt.Sprintf(" enabled ttl=%s max_size=%d", cfg.CacheTTL, cfg.CacheMaxSize)
	}

	caps, err := detectCapabilities(checkCtx, backend, cfg, log.Printf)
	if err != nil {
		return err
	}
	cfg.Capabilities = caps
	cfg.CapabilityReport = mcpserver.BuildCapabilityReport(cfg)
	logStartupDiagnostics(cfg, cacheStats)

	mcpServer := mcpserver.NewMCPServer(cfg, backend, version)
	if cfg.Transport == config.TransportStdio {
		return mcpServer.Run(context.Background(), &mcp.StdioTransport{})
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	server := newHTTPServer(cfg.Listen, mux, cfg.ReadTimeout, cfg.WriteTimeout)
	return serve(server)
}

func newHTTPClient(cfg config.Config) *http.Client {
	client := &http.Client{
		Timeout: cfg.ReadTimeout,
	}
	if cfg.InsecureSkipTLSVerify {
		log.Printf("WARNING: TLS certificate verification is disabled (OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY). Do not use in production.")
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		client.Transport = transport
	}
	return client
}

func newHTTPServer(
	addr string,
	handler http.Handler,
	readTimeout time.Duration,
	writeTimeout time.Duration,
) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}

func serve(server *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err, ok := <-errCh:
		if !ok {
			return nil
		}
		return err
	case <-ctx.Done():
		stop()
	}

	shutdownTimeout := server.WriteTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	if err, ok := <-errCh; ok {
		return err
	}
	return nil
}

type projectResolver interface {
	ListProjects(context.Context) ([]string, error)
	ScrapeProjects(context.Context) ([]string, error)
}

func resolveProjectAllowlist(
	ctx context.Context,
	cfg *config.Config,
	resolver projectResolver,
	logf func(string, ...any),
) error {
	if len(cfg.Projects) > 0 {
		cfg.ProjectSource = config.ProjectSourceConfigured
		logf("startup config: project source=%s count=%d", cfg.ProjectSource, len(cfg.Projects))
		return validateDefaultProjectAfterDiscovery(cfg)
	}

	apiProjects, apiErr := resolver.ListProjects(ctx)
	if apiErr == nil && len(apiProjects) > 0 {
		cfg.Projects = apiProjects
		cfg.ProjectSource = config.ProjectSourceAPI
		logf("startup config: project source=%s count=%d", cfg.ProjectSource, len(cfg.Projects))
		return validateDefaultProjectAfterDiscovery(cfg)
	}

	if apiErr == nil && len(apiProjects) == 0 {
		logf("startup config: projects API returned empty list")
	} else if apiErr != nil {
		logf("startup config: projects API unavailable: %v", apiErr)
	}

	if !cfg.ProjectScrapeEnabled {
		if apiErr != nil || len(apiProjects) == 0 {
			logf("startup config: web-UI project discovery disabled (OPENGROK_MCP_DISABLE_PROJECT_SCRAPE)")
		}
		cfg.Projects = nil
		cfg.ProjectSource = config.ProjectSourceNone
		logf("startup config: project source=%s count=0", cfg.ProjectSource)
		return validateDefaultProjectAfterDiscovery(cfg)
	}

	logf("startup config: web-UI project discovery enabled; fetching landing page")
	scraped, scrapeErr := resolver.ScrapeProjects(ctx)
	if scrapeErr != nil {
		logf("startup config: web-UI project scrape failed: %v", scrapeErr)
		cfg.Projects = nil
		cfg.ProjectSource = config.ProjectSourceNone
		logf("startup config: project source=%s count=0", cfg.ProjectSource)
		return validateDefaultProjectAfterDiscovery(cfg)
	}
	if len(scraped) == 0 {
		logf("startup config: web-UI project scrape returned no projects")
		cfg.Projects = nil
		cfg.ProjectSource = config.ProjectSourceNone
		logf("startup config: project source=%s count=0", cfg.ProjectSource)
		return validateDefaultProjectAfterDiscovery(cfg)
	}

	if apiErr == nil && len(apiProjects) == 0 {
		logf(
			"startup config: projects API returned empty list but web-UI scrape found %d projects; using scraped list",
			len(scraped),
		)
	}

	cfg.Projects = scraped
	cfg.ProjectSource = config.ProjectSourceScraped
	logf(
		"startup config: project source=%s count=%d (web-UI discovery; best-effort project list)",
		cfg.ProjectSource,
		len(cfg.Projects),
	)
	return validateDefaultProjectAfterDiscovery(cfg)
}

func validateDefaultProjectAfterDiscovery(cfg *config.Config) error {
	switch len(cfg.Projects) {
	case 0:
		return nil
	case 1:
		cfg.DefaultProject = cfg.Projects[0]
		return nil
	default:
		if cfg.DefaultProject == "" {
			return nil
		}
		for _, project := range cfg.Projects {
			if project == cfg.DefaultProject {
				return nil
			}
		}
		return fmt.Errorf(
			"validate config: OPENGROK_MCP_DEFAULT_PROJECT %q is not in the resolved OpenGrok project allowlist",
			cfg.DefaultProject,
		)
	}
}

type capabilityChecker interface {
	ListProjects(context.Context) ([]string, error)
	ListFiles(context.Context, string, string) ([]opengrok.FileEntry, error)
	Search(context.Context, opengrok.SearchRequest) (opengrok.SearchResult, error)
	FileContent(context.Context, string, string) (string, error)
}

func detectCapabilities(
	ctx context.Context,
	backend capabilityChecker,
	cfg config.Config,
	logf func(string, ...any),
) (config.Capabilities, error) {
	var caps config.Capabilities
	caps.ListProjects = true
	caps.Memory = cfg.Capabilities.Memory
	switch cfg.ProjectSource {
	case config.ProjectSourceConfigured:
		logCapability(logf, "list_projects", true, "using operator-configured projects")
	case config.ProjectSourceScraped:
		logCapability(logf, "list_projects", true, "using web-UI discovery snapshot")
	case config.ProjectSourceNone:
		logCapability(logf, "list_projects", true, "falling back to default project")
	default:
		logCapability(logf, "list_projects", true, "")
	}

	// A successful /projects/indexed call (source=api) is itself an authenticated
	// probe success, so seed the flag — a later 401 then classifies as
	// endpoint_disabled rather than unauthorized (R5 / FR-016).
	anyAuthedProbeSucceeded := cfg.ProjectSource == config.ProjectSourceAPI
	probeProjects := capabilityProbeProjects(cfg)
	searchOutcomes := make([]searchProbeOutcome, 0, 3)
	var searchErr error
	caps.SearchCode, searchErr = probeSearchCapability(
		ctx,
		backend,
		opengrok.ModeFullText,
		"test",
		probeProjects,
		logf,
		"search_code",
		&anyAuthedProbeSucceeded,
	)
	searchOutcomes = appendSearchOutcome(searchOutcomes, caps.SearchCode, searchErr)
	caps.SearchSymbolDefinitions, searchErr = probeSearchCapability(
		ctx,
		backend,
		opengrok.ModeDefinition,
		"test",
		probeProjects,
		logf,
		"search_symbol_definitions",
		&anyAuthedProbeSucceeded,
	)
	searchOutcomes = appendSearchOutcome(searchOutcomes, caps.SearchSymbolDefinitions, searchErr)
	caps.SearchSymbolReferences, searchErr = probeSearchCapability(
		ctx,
		backend,
		opengrok.ModeReference,
		"test",
		probeProjects,
		logf,
		"search_symbol_references",
		&anyAuthedProbeSucceeded,
	)
	searchOutcomes = appendSearchOutcome(searchOutcomes, caps.SearchSymbolReferences, searchErr)
	caps.GetFileContext = probeFileCapability(ctx, backend, cfg, logf, anyAuthedProbeSucceeded)
	caps.ListFiles = probeFileListCapability(ctx, backend, cfg, logf, anyAuthedProbeSucceeded)
	caps.ListSymbols = caps.SearchSymbolDefinitions
	if caps.ListSymbols {
		logCapability(logf, "list_symbols", true, "enabled via search_symbol_definitions")
	}

	caps.ServerSideSort = probeSortCapability(ctx, backend, probeProjects, logf, anyAuthedProbeSucceeded)

	if !caps.SearchCode && !caps.SearchSymbolDefinitions && !caps.SearchSymbolReferences {
		if authRemediationNeeded(cfg, searchOutcomes, anyAuthedProbeSucceeded) {
			logf("startup config: %s", authRemediationLog)
			return caps, nil
		}
		return caps, errors.New("check OpenGrok access: no search capabilities are available")
	}
	return caps, nil
}

type searchProbeOutcome struct {
	ok  bool
	err error
}

func appendSearchOutcome(outcomes []searchProbeOutcome, ok bool, err error) []searchProbeOutcome {
	return append(outcomes, searchProbeOutcome{ok: ok, err: err})
}

func hasAuthToken(cfg config.Config) bool {
	return cfg.OpenGrokAuthHeader != ""
}

func authRemediationNeeded(cfg config.Config, outcomes []searchProbeOutcome, anyAuthedProbeSucceeded bool) bool {
	if hasAuthToken(cfg) || len(outcomes) == 0 {
		return false
	}
	for _, outcome := range outcomes {
		if outcome.ok {
			return false
		}
		category, _ := classifyProbeError(outcome.err, anyAuthedProbeSucceeded)
		switch category {
		case "unauthorized", "endpoint_disabled":
			continue
		default:
			return false
		}
	}
	return true
}

func capabilityProbeProjects(cfg config.Config) []string {
	switch {
	case len(cfg.Projects) > 0:
		return []string{cfg.Projects[0]}
	case cfg.DefaultProject != "":
		return []string{cfg.DefaultProject}
	default:
		return []string{}
	}
}

func probeSearchCapability(
	ctx context.Context,
	backend capabilityChecker,
	mode opengrok.Mode,
	query string,
	projects []string,
	logf func(string, ...any),
	name string,
	anyAuthedProbeSucceeded *bool,
) (bool, error) {
	_, err := backend.Search(ctx, opengrok.SearchRequest{
		Projects: projects,
		Query:    query,
		Mode:     mode,
		Limit:    1,
		Offset:   0,
	})
	if err != nil {
		logCapability(logf, name, false, formatProbeFailure(err, *anyAuthedProbeSucceeded))
		return false, err
	}

	*anyAuthedProbeSucceeded = true
	logCapability(logf, name, true, "")
	return true, nil
}

func probeSortCapability(
	ctx context.Context,
	backend capabilityChecker,
	projects []string,
	logf func(string, ...any),
	anyAuthedProbeSucceeded bool,
) bool {
	_, err := backend.Search(ctx, opengrok.SearchRequest{
		Projects: projects,
		Query:    "test",
		Mode:     opengrok.ModeFullText,
		Limit:    1,
		Offset:   0,
		Sort:     "path",
	})
	if err != nil {
		logCapability(logf, "server_side_sort", false, formatProbeFailure(err, anyAuthedProbeSucceeded))
		return false
	}

	logCapability(logf, "server_side_sort", true, "")
	return true
}

func probeFileCapability(
	ctx context.Context,
	backend capabilityChecker,
	cfg config.Config,
	logf func(string, ...any),
	anyAuthedProbeSucceeded bool,
) bool {
	probeFile := cfg.ProbeFile
	if probeFile == "" {
		if cfg.OpenGrokWebBaseURL != "" {
			logCapability(logf, "get_file_context", true, "raw web fallback configured without probe file")
			return true
		}
		logCapability(logf, "get_file_context", false, "OPENGROK_MCP_PROBE_FILE and OPENGROK_MCP_WEB_BASE_URL are not configured")
		return false
	}

	project, filePath, ok := strings.Cut(strings.Trim(probeFile, "/"), "/")
	if !ok || project == "" || filePath == "" {
		logCapability(logf, "get_file_context", false, "OPENGROK_MCP_PROBE_FILE must be project/path")
		return false
	}
	if _, err := backend.FileContent(ctx, project, filePath); err != nil {
		logCapability(logf, "get_file_context", false, formatProbeFailure(err, anyAuthedProbeSucceeded))
		return false
	}

	logCapability(logf, "get_file_context", true, "")
	return true
}

func probeFileListCapability(
	ctx context.Context,
	backend capabilityChecker,
	cfg config.Config,
	logf func(string, ...any),
	anyAuthedProbeSucceeded bool,
) bool {
	probeProjects := capabilityProbeProjects(cfg)
	var project string
	if len(probeProjects) > 0 {
		project = probeProjects[0]
	} else {
		project = cfg.DefaultProject
	}
	if project == "" {
		logCapability(logf, "list_files", false, "no project configured for probe")
		return false
	}

	if _, err := backend.ListFiles(ctx, project, ""); err != nil {
		logCapability(logf, "list_files", false, formatProbeFailure(err, anyAuthedProbeSucceeded))
		return false
	}

	logCapability(logf, "list_files", true, "")
	return true
}

func logCapability(logf func(string, ...any), name string, enabled bool, reason string) {
	if reason == "" {
		logf("opengrok capability %s: enabled", name)
		return
	}
	if enabled {
		logf("opengrok capability %s: enabled: %s", name, reason)
		return
	}
	logf("opengrok capability %s: disabled: %s", name, reason)
}

func classifyProbeError(err error, anyAuthedProbeSucceeded bool) (string, []string) {
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) && len(certErr.UnverifiedCertificates) > 0 {
		return "tls_mismatch", certErr.UnverifiedCertificates[0].DNSNames
	}

	var statusErr *opengrok.StatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.Code == http.StatusUnauthorized || statusErr.Code == http.StatusForbidden:
			if anyAuthedProbeSucceeded {
				return "endpoint_disabled", nil
			}
			return "unauthorized", nil
		case statusErr.Code >= 400 && statusErr.Code < 500:
			return "feature_unsupported", nil
		}
	}

	return "transport_error", nil
}

func formatProbeFailure(err error, anyAuthedProbeSucceeded bool) string {
	category, certSANs := classifyProbeError(err, anyAuthedProbeSucceeded)
	if category == "tls_mismatch" && len(certSANs) > 0 {
		return fmt.Sprintf(
			"classification=%s cert_valid_for=%s: %s",
			category,
			strings.Join(certSANs, ", "),
			err.Error(),
		)
	}
	return fmt.Sprintf("classification=%s: %s", category, err.Error())
}

func opengrokOptions(cfg config.Config) []opengrok.Option {
	options := []opengrok.Option{}
	if cfg.DefaultProject != "" {
		options = append(options, opengrok.WithDefaultProject(cfg.DefaultProject))
	}
	if cfg.OpenGrokAuthHeader != "" {
		options = append(options, opengrok.WithAuthorizationHeader(cfg.OpenGrokAuthHeader))
	}
	if cfg.OpenGrokWebBaseURL != "" {
		options = append(options, opengrok.WithWebBaseURL(cfg.OpenGrokWebBaseURL))
	}
	if cfg.Debug {
		options = append(options, opengrok.WithDebug(true))
	}
	options = append(options, opengrok.WithRetryPolicy(opengrok.RetryPolicy{
		MaxAttempts: cfg.RetryMaxAttempts,
		BaseDelay:   cfg.RetryBaseDelay,
	}))

	return options
}

func logStartupDiagnostics(cfg config.Config, cacheStats string) {
	// Derived values
	derivedWebURL := strings.TrimSuffix(strings.TrimRight(cfg.OpenGrokAPIBaseURL, "/"), "/api/v1")
	if derivedWebURL == cfg.OpenGrokWebBaseURL && os.Getenv("OPENGROK_MCP_WEB_BASE_URL") == "" {
		log.Printf("startup config: web URL=%s (derived from API URL)", cfg.OpenGrokWebBaseURL)
	} else {
		log.Printf("startup config: web URL=%s", cfg.OpenGrokWebBaseURL)
	}

	if len(cfg.Projects) == 1 && cfg.DefaultProject == cfg.Projects[0] && os.Getenv("OPENGROK_MCP_DEFAULT_PROJECT") == "" {
		log.Printf("startup config: default project=%s (derived from single project list)", cfg.DefaultProject)
	} else {
		log.Printf("startup config: default project=%s", cfg.DefaultProject)
	}

	if cfg.ProjectScrapeEnabled {
		log.Printf("startup config: project scrape=default-on")
	} else {
		log.Printf("startup config: project scrape=disabled")
	}

	// Explicit overrides
	envVars := []string{
		"OPENGROK_MCP_TRANSPORT", "OPENGROK_MCP_TOOL_SURFACE", "OPENGROK_MCP_LISTEN",
		"OPENGROK_MCP_BASE_URL", "OPENGROK_MCP_WEB_BASE_URL",
		"OPENGROK_MCP_API_TOKEN",
		"OPENGROK_MCP_PROJECTS", "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE", "OPENGROK_MCP_PROJECT_SCRAPE",
		"OPENGROK_MCP_PROBE_FILE", "OPENGROK_MCP_DEFAULT_PROJECT",
		"OPENGROK_MCP_LOG_LEVEL", "OPENGROK_MCP_PROJECT_REQUIRED",
		"OPENGROK_MCP_DIAGNOSTICS",
		"OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY", "OPENGROK_MCP_AUTO_EXPAND_CONTEXT",
		"OPENGROK_MCP_CONTEXT_BEFORE", "OPENGROK_MCP_CONTEXT_AFTER",
		"OPENGROK_MCP_MAX_EXPANDED_RESULTS", "OPENGROK_MCP_MAX_EXPANDED_FILES",
		"OPENGROK_MCP_CONTEXT_FETCH_CONCURRENCY",
		"OPENGROK_MCP_RETRY_MAX_ATTEMPTS", "OPENGROK_MCP_RETRY_BASE_DELAY",
		"OPENGROK_MCP_MEMORY_ENABLED", "DEBUG",
	}
	var overrideNames []string
	for _, env := range envVars {
		if os.Getenv(env) != "" {
			overrideNames = append(overrideNames, env)
		}
	}
	if len(overrideNames) > 0 {
		log.Printf("startup config: explicit overrides: %s", strings.Join(overrideNames, ", "))
	}

	// Enabled capabilities
	log.Printf("startup config: capabilities list_projects=%t search_code=%t search_symbol_definitions=%t search_symbol_references=%t get_file_context=%t list_symbols=%t list_files=%t server_side_sort=%t memory=%t",
		cfg.Capabilities.ListProjects, cfg.Capabilities.SearchCode, cfg.Capabilities.SearchSymbolDefinitions,
		cfg.Capabilities.SearchSymbolReferences, cfg.Capabilities.GetFileContext, cfg.Capabilities.ListSymbols,
		cfg.Capabilities.ListFiles, cfg.Capabilities.ServerSideSort, cfg.Capabilities.Memory)

	// Tool surface
	log.Printf("startup config: tool surface=%s", cfg.ToolSurface)

	// Retry policy
	log.Printf("startup config: retry policy max_attempts=%d base_delay=%s", cfg.RetryMaxAttempts, cfg.RetryBaseDelay)

	log.Printf("startup config: diagnostics=%t", cfg.Diagnostics)

	// Expansion budgets
	log.Printf("startup config: expansion budgets max_expanded_results=%d max_expanded_files=%d context_before=%d context_after=%d",
		cfg.MaxExpandedResults, cfg.MaxExpandedFiles, cfg.ContextBefore, cfg.ContextAfter)

	log.Printf("startup config: cache%s", cacheStats)
}
