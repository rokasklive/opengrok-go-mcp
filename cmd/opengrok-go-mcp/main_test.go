package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

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

func TestCheckOpenGrokAccessFailsWhenListProjectsFails(t *testing.T) {
	wantErr := errors.New("unauthorized")
	err := checkOpenGrokAccess(context.Background(), failingProjectLister{err: wantErr})

	if err == nil {
		t.Fatal("checkOpenGrokAccess() error = nil, want error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("checkOpenGrokAccess() error = %v, want to wrap %v", err, wantErr)
	}
}

type failingProjectLister struct {
	err error
}

func (l failingProjectLister) ListProjects(context.Context) ([]string, error) {
	return nil, l.err
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
