// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const emptySearchJSON = `{"resultCount":0,"startDocument":0,"endDocument":0,"results":{}}`

// countingServer counts every request that reaches it.
func countingServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func rejectWithBasicChallenge(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("WWW-Authenticate", `Basic realm="EIS opengrok"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func TestAuthBreakerStopsAfterFirstRejection(t *testing.T) {
	server, requests := countingServer(t, rejectWithBasicChallenge)
	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithWebBaseURL(server.URL+"/source"),
		WithBasicAuthToken("dXNlcjp3cm9uZw=="),
		WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond}),
	)
	ctx := context.Background()

	_, err := client.ListProjects(ctx)
	var rejection *AuthRejectedError
	if !errors.As(err, &rejection) {
		t.Fatalf("ListProjects() error = %v, want *AuthRejectedError", err)
	}
	if rejection.Path != "/api/v1/projects/indexed" || rejection.Method != http.MethodGet {
		t.Fatalf("rejection = %+v, want GET /api/v1/projects/indexed", rejection)
	}

	// Every later request, on any endpoint, fails without reaching the server.
	if _, err := client.Search(ctx, SearchRequest{Query: "test", Mode: ModeFullText, Limit: 1}); !IsAuthRejection(err) {
		t.Fatalf("Search() error = %v, want the cached rejection", err)
	}
	if _, err := client.FileContent(ctx, "platform", "src/a.go"); !IsAuthRejection(err) {
		t.Fatalf("FileContent() error = %v, want the cached rejection", err)
	}
	if _, err := client.ScrapeProjects(ctx); !IsAuthRejection(err) {
		t.Fatalf("ScrapeProjects() error = %v, want the cached rejection", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests sent = %d, want 1: no retry and nothing after the rejection", got)
	}
	if client.AuthRejection() != rejection {
		t.Fatalf("AuthRejection() = %v, want the first rejection", client.AuthRejection())
	}
}

// OpenGrok's per-endpoint API-token check answers a bare 401 even with valid
// credentials; that must not stop the client.
func TestAuthBreakerIgnoresBare401OnRestrictedEndpoint(t *testing.T) {
	server, requests := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/projects/indexed":
			http.Error(w, "API token required", http.StatusUnauthorized)
		case "/api/v1/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(emptySearchJSON))
		default:
			http.NotFound(w, r)
		}
	})
	client := NewClient(server.URL+"/api/v1", server.Client(), WithBasicAuthToken("dXNlcjpwYXNz"))
	ctx := context.Background()

	_, err := client.ListProjects(ctx)
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != http.StatusUnauthorized || IsAuthRejection(err) {
		t.Fatalf("ListProjects() error = %v, want a plain 401 StatusError", err)
	}
	if _, err := client.Search(ctx, SearchRequest{Query: "test", Mode: ModeFullText, Limit: 1}); err != nil {
		t.Fatalf("Search() error = %v, want success", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests sent = %d, want 2", got)
	}
	if client.AuthRejection() != nil {
		t.Fatalf("AuthRejection() = %v, want nil", client.AuthRejection())
	}
}

// Search is never restricted to API tokens, so a 401 there rejects the
// credentials even without a challenge header.
func TestAuthBreakerTripsOnBare401FromSearch(t *testing.T) {
	server, requests := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
	client := NewClient(server.URL+"/api/v1", server.Client(), WithBasicAuthToken("dXNlcjp3cm9uZw=="))
	ctx := context.Background()

	if _, err := client.Search(ctx, SearchRequest{Query: "test", Mode: ModeFullText, Limit: 1}); !IsAuthRejection(err) {
		t.Fatalf("Search() error = %v, want *AuthRejectedError", err)
	}
	if _, err := client.ListProjects(ctx); !IsAuthRejection(err) {
		t.Fatalf("ListProjects() error = %v, want the cached rejection", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests sent = %d, want 1", got)
	}
}

func TestAuthBreakerIgnoresForbidden(t *testing.T) {
	server, requests := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="EIS opengrok"`)
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
	client := NewClient(server.URL+"/api/v1", server.Client(), WithBasicAuthToken("dXNlcjpwYXNz"))
	ctx := context.Background()

	for range 2 {
		if _, err := client.ListProjects(ctx); err == nil || IsAuthRejection(err) {
			t.Fatalf("ListProjects() error = %v, want a plain 403 StatusError", err)
		}
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests sent = %d, want 2: a 403 does not reject the credentials", got)
	}
}

func TestAuthBreakerNeedsConfiguredCredentials(t *testing.T) {
	server, requests := countingServer(t, rejectWithBasicChallenge)
	client := NewClient(server.URL+"/api/v1", server.Client())
	ctx := context.Background()

	for range 2 {
		if _, err := client.ListProjects(ctx); err == nil || IsAuthRejection(err) {
			t.Fatalf("ListProjects() error = %v, want a plain 401 StatusError", err)
		}
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests sent = %d, want 2: without credentials there is no login to protect", got)
	}
	if client.AuthRejection() != nil {
		t.Fatalf("AuthRejection() = %v, want nil", client.AuthRejection())
	}
}

func TestAuthBreakerLeavesSharedHTTPClientUntouched(t *testing.T) {
	shared := &http.Client{}
	_ = NewClient("https://grok.example.com/api/v1", shared, WithBasicAuthToken("dXNlcjpwYXNz"))
	if shared.Transport != nil {
		t.Fatalf("shared client Transport = %T, want it left nil", shared.Transport)
	}
}
