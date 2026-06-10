// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
)

type route struct {
	Path    string            `json:"path"`
	Match   map[string]string `json:"match"`
	Default bool              `json:"default"`
	Fixture string            `json:"fixture"`
}

type manifest struct {
	Routes []route `json:"routes"`
}

// startBackend serves fixtures from testdata according to manifest.json.
func startBackend(_ context.Context, testdataDir string) (env []string, stop func(), err error) {
	raw, err := os.ReadFile(filepath.Join(testdataDir, "manifest.json"))
	if err != nil {
		return nil, nil, err
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, err
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/source/raw/") {
			serveRawFixture(w, testdataDir, "opengrok/file_content_engine.json")
			return
		}

		q := r.URL.Query()
		var fallback string
		for _, rt := range m.Routes {
			if rt.Path != r.URL.Path {
				continue
			}
			if rt.Default {
				if fallback == "" {
					fallback = rt.Fixture
				}
				continue
			}
			if routeMatches(q, rt.Match) {
				serveFixture(w, filepath.Join(testdataDir, rt.Fixture))
				return
			}
		}
		if fallback != "" {
			serveFixture(w, filepath.Join(testdataDir, fallback))
			return
		}
		http.Error(w, "no route", http.StatusNotFound)
	}))

	env = []string{
		"OPENGROK_MCP_BASE_URL=" + srv.URL + "/api/v1",
		"OPENGROK_MCP_WEB_BASE_URL=" + srv.URL + "/source",
		"OPENGROK_MCP_DEFAULT_PROJECT=platform",
		"OPENGROK_MCP_PROBE_FILE=platform/src/Engine.swift",
		"OPENGROK_MCP_PROJECTS=platform,infra",
		"OPENGROK_MCP_CURSOR_SECRET=eval-harness-secret",
	}
	return env, srv.Close, nil
}

func routeMatches(q map[string][]string, want map[string]string) bool {
	for k, v := range want {
		got, ok := q[k]
		if !ok || len(got) == 0 || got[0] != v {
			return false
		}
	}
	return true
}

func serveFixture(w http.ResponseWriter, path string) {
	body, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func serveRawFixture(w http.ResponseWriter, testdataDir, fixtureRel string) {
	body, err := os.ReadFile(filepath.Join(testdataDir, fixtureRel))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var payload struct {
		Contents string `json:"contents"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(payload.Contents))
}
