// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const versionedProjectSelectHTML = `<!DOCTYPE html>
<html><body>
<form>
<select name="project" class="project-picker" id="project" multiple size="5" tabindex="1">
<option value="app-1.2-full">app-1.2-full</option>
<option value="app-1.2-minimal">app-1.2-minimal</option>
</select>
</form>
</body></html>`

const flatProjectSelectHTML = `<!DOCTYPE html>
<html><body>
<select id="project" name="project" multiple>
<option value="alpha">alpha</option>
<option value="beta">beta</option>
</select>
</body></html>`

const xrefNoiseHTML = `<!DOCTYPE html>
<html><body>
<a href="/source/xref/alpha/src/main.go">alpha/src</a>
<a href="/source/xref/beta/lib">beta/lib</a>
<select id="project" name="project" multiple>
<option value="alpha">alpha</option>
<option value="beta">beta</option>
</select>
<p>more content that should not be read after </select></p>
</body></html>`

func TestParseProjectSelectOptionsVersioned(t *testing.T) {
	got := parseProjectSelectOptions(strings.NewReader(versionedProjectSelectHTML))
	want := []string{"app-1.2-full", "app-1.2-minimal"}
	if !slicesEqualStrings(got, want) {
		t.Fatalf("parseProjectSelectOptions() = %#v, want %#v", got, want)
	}
}

func TestParseProjectSelectOptionsFlat(t *testing.T) {
	got := parseProjectSelectOptions(strings.NewReader(flatProjectSelectHTML))
	want := []string{"alpha", "beta"}
	if !slicesEqualStrings(got, want) {
		t.Fatalf("parseProjectSelectOptions() = %#v, want %#v", got, want)
	}
}

func TestParseProjectSelectOptionsReorderedAttributes(t *testing.T) {
	html := `<select class="x" name="project" multiple id="project" size="3">
<option value="only">only</option>
</select>`
	got := parseProjectSelectOptions(strings.NewReader(html))
	want := []string{"only"}
	if !slicesEqualStrings(got, want) {
		t.Fatalf("parseProjectSelectOptions() = %#v, want %#v", got, want)
	}
}

func TestParseProjectSelectOptionsIgnoresXrefNoise(t *testing.T) {
	got := parseProjectSelectOptions(strings.NewReader(xrefNoiseHTML))
	want := []string{"alpha", "beta"}
	if !slicesEqualStrings(got, want) {
		t.Fatalf("parseProjectSelectOptions() = %#v, want %#v", got, want)
	}
}

func TestParseProjectSelectOptionsMissingSelect(t *testing.T) {
	html := `<html><body><a href="/source/xref/alpha">alpha</a></body></html>`
	got := parseProjectSelectOptions(strings.NewReader(html))
	if len(got) != 0 {
		t.Fatalf("parseProjectSelectOptions() = %#v, want empty", got)
	}
}

func TestParseProjectSelectOptionsMalformedHTML(t *testing.T) {
	got := parseProjectSelectOptions(strings.NewReader(`<select id="project"><option value="a"`))
	if len(got) != 0 {
		t.Fatalf("parseProjectSelectOptions() = %#v, want empty", got)
	}
}

func TestParseProjectSelectOptionsValueFallbackToText(t *testing.T) {
	html := `<select id="project"><option>gamma</option><option value="delta">delta</option></select>`
	got := parseProjectSelectOptions(strings.NewReader(html))
	want := []string{"gamma", "delta"}
	if !slicesEqualStrings(got, want) {
		t.Fatalf("parseProjectSelectOptions() = %#v, want %#v", got, want)
	}
}

func TestParseProjectSelectOptionsIgnoresExplicitEmptyOptionValue(t *testing.T) {
	html := `<select id="project">
<option value="">All projects</option>
<option value="alpha">alpha</option>
<option>beta</option>
</select>`
	got := parseProjectSelectOptions(strings.NewReader(html))
	want := []string{"alpha", "beta"}
	if !slicesEqualStrings(got, want) {
		t.Fatalf("parseProjectSelectOptions() = %#v, want %#v", got, want)
	}
}

func TestScrapeProjectsSendsAuthHeader(t *testing.T) {
	const wantAuth = "Basic scrape-token"
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("path = %q, want /", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(flatProjectSelectHTML))
	}))
	defer server.Close()

	client := NewClient(
		server.URL+"/api/v1",
		server.Client(),
		WithWebBaseURL(server.URL),
		WithBasicAuthToken("scrape-token"),
	)

	projects, err := client.ScrapeProjects(context.Background())
	if err != nil {
		t.Fatalf("ScrapeProjects() error = %v", err)
	}
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	if !slicesEqualStrings(projects, []string{"alpha", "beta"}) {
		t.Fatalf("ScrapeProjects() = %#v, want alpha/beta", projects)
	}
}

func TestLimitErrorReaderEnforcesCap(t *testing.T) {
	body := strings.Repeat("x", 2048)
	limited := &limitErrorReader{r: strings.NewReader(body), max: 1024}
	buf := make([]byte, 4096)
	for i := 0; i < 100; i++ {
		_, err := limited.Read(buf)
		if err != nil {
			if limited.Err() == nil {
				t.Fatalf("Read error %v but Err() is nil", err)
			}
			return
		}
	}
	t.Fatal("Read never returned error for oversized body")
}

func TestParseProjectSelectOptionsEnforcesCap(t *testing.T) {
	originalMax := maxScrapeResponseBytes
	maxScrapeResponseBytes = 1024
	t.Cleanup(func() { maxScrapeResponseBytes = originalMax })

	body := `<select id="project">` + strings.Repeat("x", 1025)
	limited := &limitErrorReader{r: strings.NewReader(body), max: maxScrapeResponseBytes}
	_ = parseProjectSelectOptions(limited)
	if limited.Err() == nil {
		t.Fatal("limitErrorReader.Err() = nil, want size-cap error")
	}
}

func TestScrapeProjectsEnforcesSizeCap(t *testing.T) {
	originalMax := maxScrapeResponseBytes
	maxScrapeResponseBytes = 1024
	t.Cleanup(func() { maxScrapeResponseBytes = originalMax })

	payload := []byte(`<select id="project">` + strings.Repeat("x", int(maxScrapeResponseBytes+1)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write(payload); err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client(), WithWebBaseURL(server.URL))
	_, err := client.ScrapeProjects(context.Background())
	if err == nil {
		t.Fatalf("ScrapeProjects() error = nil, want size-cap error (payload %d bytes, cap %d)", len(payload), maxScrapeResponseBytes)
	}
}

func TestScrapeProjectsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/api/v1", server.Client(), WithWebBaseURL(server.URL))
	_, err := client.ScrapeProjects(context.Background())
	if err == nil {
		t.Fatal("ScrapeProjects() error = nil, want error")
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != http.StatusUnauthorized {
		t.Fatalf("ScrapeProjects() error = %v, want *StatusError 401", err)
	}
}

func slicesEqualStrings(a, b []string) bool {
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
