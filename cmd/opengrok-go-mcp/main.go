package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/mcpserver"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

const version = "0.1.0"

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

	httpClient := &http.Client{
		Timeout: cfg.ReadTimeout,
	}
	backend := opengrok.NewClient(
		cfg.OpenGrokAPIBaseURL,
		httpClient,
		opengrokOptions(cfg)...,
	)
	checkCtx, cancel := context.WithTimeout(context.Background(), cfg.ReadTimeout)
	defer cancel()
	if err := checkOpenGrokAccess(checkCtx, backend); err != nil {
		return err
	}

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

type projectLister interface {
	ListProjects(context.Context) ([]string, error)
}

func checkOpenGrokAccess(ctx context.Context, backend projectLister) error {
	if _, err := backend.ListProjects(ctx); err != nil {
		return fmt.Errorf("check OpenGrok access: %w", err)
	}
	return nil
}

func opengrokOptions(cfg config.Config) []opengrok.Option {
	options := []opengrok.Option{}
	if cfg.OpenGrokAPIToken != "" {
		options = append(options, opengrok.WithAPIToken(cfg.OpenGrokAPIToken))
	}
	if cfg.OpenGrokBasicAuthToken != "" {
		options = append(options, opengrok.WithBasicAuthToken(cfg.OpenGrokBasicAuthToken))
	}
	if cfg.Debug {
		options = append(options, opengrok.WithDebug(true))
	}

	return options
}
