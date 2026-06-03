// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var maxScrapeResponseBytes int64 = 8 << 20 // 8 MiB

// ScrapeProjects fetches the web UI landing page and extracts project names from
// the <select id="project"> element.
func (c *Client) ScrapeProjects(ctx context.Context) ([]string, error) {
	if c.webBaseURL == "" {
		return nil, fmt.Errorf("scrape projects: web base URL is not configured")
	}

	requestURL := c.webBaseURL + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("scrape projects: create request: %w", err)
	}
	c.addAuth(req)

	start := time.Now()
	c.logAPI("opengrok web request method=%s url=%s", req.Method, req.URL.Redacted())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logAPI(
			"opengrok web error method=%s url=%s duration=%s error=%v",
			req.Method,
			req.URL.Redacted(),
			time.Since(start),
			err,
		)
		return nil, fmt.Errorf("scrape projects: GET /: %w", err)
	}
	defer resp.Body.Close()

	c.logAPI(
		"opengrok web response method=%s url=%s status=%s duration=%s",
		req.Method,
		req.URL.Redacted(),
		resp.Status,
		time.Since(start),
	)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &StatusError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Path:   "/",
		}
	}

	limited := &limitErrorReader{r: resp.Body, max: maxScrapeResponseBytes}
	projects := parseProjectSelectOptions(limited)
	if err := limited.Err(); err != nil {
		return nil, err
	}

	return projects, nil
}

type limitErrorReader struct {
	r   io.Reader
	max int64
	n   int64
	err error
}

func (l *limitErrorReader) Read(p []byte) (int, error) {
	if l.err != nil {
		return 0, l.err
	}
	// Read up to one byte past the cap so a body of exactly max bytes is allowed
	// while anything larger is rejected.
	remaining := (l.max + 1) - l.n
	if remaining <= 0 {
		l.err = fmt.Errorf("scrape projects: response exceeds %d-byte limit", l.max)
		return 0, l.err
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := l.r.Read(p)
	l.n += int64(n)
	if l.n > l.max {
		l.err = fmt.Errorf("scrape projects: response exceeds %d-byte limit", l.max)
		return n, l.err
	}
	return n, err
}

func (l *limitErrorReader) Err() error {
	return l.err
}

func parseProjectSelectOptions(r io.Reader) []string {
	z := html.NewTokenizer(r)
	inTargetSelect := false
	inOption := false
	optionHasValueAttr := false
	var optionValue strings.Builder
	projects := []string{}

	// flushOption records the currently-open <option>. html.Tokenizer is a raw
	// tokenizer that performs no implicit tag closing, so an option must be
	// flushed not only on its explicit </option> but also when the next <option>
	// starts or the </select> closes — OpenGrok markup may omit </option>.
	flushOption := func() {
		if !inOption {
			return
		}
		if value := strings.TrimSpace(optionValue.String()); value != "" {
			projects = append(projects, value)
		}
		inOption = false
		optionHasValueAttr = false
		optionValue.Reset()
	}

	for {
		switch z.Next() {
		case html.ErrorToken:
			return projects
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "select" && !inTargetSelect {
				if id, ok := tagAttr(z, "id"); ok && id == "project" {
					inTargetSelect = true
				}
				continue
			}
			if !inTargetSelect {
				continue
			}
			if tag == "option" {
				flushOption()
				inOption = true
				optionHasValueAttr = false
				optionValue.Reset()
				if value, ok := tagAttr(z, "value"); ok {
					optionHasValueAttr = true
					optionValue.WriteString(value)
				}
			}
		case html.TextToken:
			if inTargetSelect && inOption && !optionHasValueAttr {
				optionValue.Write(z.Text())
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "select" && inTargetSelect {
				flushOption()
				return projects
			}
			if inTargetSelect && tag == "option" {
				flushOption()
			}
		}
	}
}

func tagAttr(z *html.Tokenizer, wantKey string) (string, bool) {
	for {
		key, val, moreAttr := z.TagAttr()
		if len(key) == 0 {
			break
		}
		if string(key) == wantKey {
			return string(val), true
		}
		if !moreAttr {
			break
		}
	}
	return "", false
}
