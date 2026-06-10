// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Mode string

const (
	maxResponseBytes   = 32 << 20 // 32 MB
	maxFileListEntries = 5000

	ModeFullText   Mode = "full_text"
	ModeDefinition Mode = "definition"
	ModeReference  Mode = "reference"
	ModePath       Mode = "path"
	ModeHistory    Mode = "history"
)

type Client struct {
	baseURL             string
	webBaseURL          string
	defaultProject      string
	httpClient          *http.Client
	authorizationHeader string
	debugLogf           func(string, ...any)
	retryPolicy         *RetryPolicy
}

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

type Option func(*Client)

func WithAuthorizationHeader(value string) Option {
	return func(c *Client) {
		c.authorizationHeader = strings.TrimSpace(value)
	}
}

// WithAPIToken configures Bearer authorization. Prefer WithAuthorizationHeader for new code.
func WithAPIToken(token string) Option {
	return WithAuthorizationHeader(authHeaderValue("Bearer", token))
}

// WithBasicAuthToken configures Basic authorization. Prefer WithAuthorizationHeader for new code.
func WithBasicAuthToken(token string) Option {
	return WithAuthorizationHeader(authHeaderValue("Basic", token))
}

func WithDefaultProject(project string) Option {
	return func(c *Client) {
		c.defaultProject = project
	}
}

// SetDefaultProject updates the default project used for result-path attribution.
// Startup project resolution constructs the client before the allowlist (and a
// single discovered/derived default project) is known, so this lets the resolved
// default be applied afterward, keeping client attribution consistent with config.
func (c *Client) SetDefaultProject(project string) {
	c.defaultProject = project
}

func WithWebBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.webBaseURL = strings.TrimRight(baseURL, "/")
	}
}

func WithDebug(enabled bool) Option {
	return func(c *Client) {
		if enabled {
			c.debugLogf = log.Printf
		}
	}
}

func WithRetryPolicy(policy RetryPolicy) Option {
	return func(c *Client) {
		c.retryPolicy = &policy
	}
}

func WithDebugLogger(logf func(string, ...any)) Option {
	return func(c *Client) {
		c.debugLogf = logf
	}
}

func NewClient(baseURL string, httpClient *http.Client, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	client := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
	for _, opt := range opts {
		opt(client)
	}

	return client
}

type SearchRequest struct {
	Projects   []string
	Query      string
	Mode       Mode
	PathPrefix string
	FileType   string
	Limit      int
	Offset     int
	Sort       string
}

type SearchResult struct {
	TotalHits int
	Start     int
	End       int
	Hits      []Hit
}

type Hit struct {
	Project              string
	FilePath             string
	LineNumber           int
	Snippet              *string
	Tag                  string
	AttributionUncertain bool
	AttributionWarning   string
	AttributionSource    string `json:"attribution_source,omitempty"`
}

type ProjectOverview struct {
	Project      string
	TotalFiles   int
	TotalDirs    int
	TopDirs      []FileEntry
	TopFiles     []FileEntry
	Description  string
	TotalSymbols int
	LastIndexed  string
}

type FileEntry struct {
	Path            string `json:"path"`
	NumLines        int    `json:"numLines"`
	Loc             int    `json:"loc"`
	Date            int64  `json:"date"`
	Description     string `json:"description"`
	PathDescription string `json:"pathDescription"`
	IsDirectory     bool   `json:"isDirectory"`
	Size            *int64 `json:"size"`
}

func (c *Client) ListFiles(ctx context.Context, project string, path string) ([]FileEntry, error) {
	entries, _, err := c.ListFilesWithMetadata(ctx, project, path)
	return entries, err
}

// ListFilesWithMetadata returns file entries and whether the server-side
// safety cap omitted additional entries.
func (c *Client) ListFilesWithMetadata(ctx context.Context, project string, path string) ([]FileEntry, bool, error) {
	query := url.Values{}
	fullPath := project
	if path != "" {
		fullPath = project + "/" + path
	}
	query.Set("path", fullPath)

	body, err := c.do(ctx, "/list", query)
	if err != nil {
		return nil, false, fmt.Errorf("list files: %w", err)
	}

	var entries []FileEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, false, fmt.Errorf("list files: decode response: %w", err)
	}

	truncated := false
	if len(entries) > maxFileListEntries {
		entries = entries[:maxFileListEntries]
		truncated = true
		c.logAPI("opengrok: file list truncated at %d entries for project %s", maxFileListEntries, project)
	}

	return entries, truncated, nil
}

func (c *Client) GetProjectOverview(ctx context.Context, project string) (ProjectOverview, error) {
	entries, err := c.ListFiles(ctx, project, "")
	if err != nil {
		return ProjectOverview{}, fmt.Errorf("project overview: %w", err)
	}

	var overview ProjectOverview
	overview.Project = project
	overview.Description = fmt.Sprintf("Project %s overview", project)
	projectPrefix := project + "/"

	for _, entry := range entries {
		rel := strings.TrimPrefix(entry.Path, projectPrefix)
		if entry.IsDirectory {
			overview.TotalDirs++
			if !strings.Contains(rel, "/") {
				overview.TopDirs = append(overview.TopDirs, entry)
			}
		} else {
			overview.TotalFiles++
			if !strings.Contains(rel, "/") {
				overview.TopFiles = append(overview.TopFiles, entry)
			}
		}
	}

	return overview, nil
}

func (c *Client) ListProjects(ctx context.Context) ([]string, error) {
	body, err := c.do(ctx, "/projects/indexed", nil)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects := []string{}
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("list projects: decode response: %w", err)
	}

	return projects, nil
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	query := url.Values{}
	for _, project := range req.Projects {
		query.Add("projects", project)
	}

	searchParam := modeQueryParam(req.Mode)
	query.Set(searchParam, req.Query)
	if req.Mode != ModePath && req.PathPrefix != "" {
		query.Set("path", req.PathPrefix)
	}
	if req.FileType != "" {
		query.Set("type", req.FileType)
	}
	if req.Limit > 0 {
		query.Set("maxresults", strconv.Itoa(req.Limit))
	}
	query.Set("start", strconv.Itoa(req.Offset))
	if req.Sort != "" {
		query.Set("sort", req.Sort)
	}

	body, err := c.do(ctx, "/search", query)
	if err != nil {
		return SearchResult{Hits: []Hit{}}, fmt.Errorf("search: %w", err)
	}

	var response searchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return SearchResult{Hits: []Hit{}}, fmt.Errorf("search: decode response: %w", err)
	}

	return response.toResult(req.Projects, c.defaultProject), nil
}

func (c *Client) FileContent(ctx context.Context, project string, filePath string) (string, error) {
	query := url.Values{}
	contentPath := filePath
	if project != "" {
		contentPath = strings.TrimSuffix(project, "/") + "/" + strings.TrimPrefix(filePath, "/")
	}
	query.Set("path", contentPath)

	body, err := c.do(ctx, "/file/content", query)
	if err != nil {
		if c.webBaseURL == "" {
			return "", fmt.Errorf("file content: %w", err)
		}
		body, err = c.doRaw(ctx, rawFilePath(project, filePath))
		if err != nil {
			return "", fmt.Errorf("file content: %w", err)
		}
	}

	var response struct {
		Contents string `json:"contents"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return string(body), nil
	}

	return response.Contents, nil
}

func rawFilePath(project string, filePath string) string {
	return "/raw/" + strings.TrimSuffix(project, "/") + "/" + strings.TrimPrefix(filePath, "/")
}

func (c *Client) do(ctx context.Context, path string, query url.Values) ([]byte, error) {
	requestURL := c.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	body, _, err := c.doGET(ctx, requestURL, path, "api")
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *Client) doRaw(ctx context.Context, path string) ([]byte, error) {
	requestURL := c.webBaseURL + path
	body, _, err := c.doGET(ctx, requestURL, path, "web")
	if err != nil {
		return nil, err
	}
	return body, nil
}

// doGET is a status-aware GET helper with retry/backoff for transient failures.
// It builds the request, adds auth, executes it, reads the capped body, and
// returns the body, HTTP status code, and any error. Retryable errors include
// transport errors, HTTP 429 (Too Many Requests), and HTTP 5xx. Non-retryable
// 4xx statuses are returned immediately.
func (c *Client) doGET(ctx context.Context, requestURL string, pathDesc string, logKind string) ([]byte, int, error) {
	maxAttempts := 1
	baseDelay := time.Duration(0)
	if c.retryPolicy != nil {
		maxAttempts = c.retryPolicy.MaxAttempts
		baseDelay = c.retryPolicy.BaseDelay
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastBody []byte
	var lastStatusCode int

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("create GET %s: %w", pathDesc, err)
		}
		c.addAuth(req)

		start := time.Now()
		c.logAPI("opengrok %s request method=%s url=%s", logKind, req.Method, req.URL.Redacted())
		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.logAPI(
				"opengrok %s error method=%s url=%s duration=%s error=%v",
				logKind,
				req.Method,
				req.URL.Redacted(),
				time.Since(start),
				err,
			)
			lastStatusCode = 0
			if attempt < maxAttempts {
				c.logAPI(
					"opengrok %s retry method=GET url=%s error=%v attempt=%d delay=%s",
					logKind, req.URL.Redacted(), err, attempt+1, baseDelay,
				)
				if err := c.sleepWithContext(ctx, baseDelay, attempt); err != nil {
					return nil, lastStatusCode, err
				}
				continue
			}
			return nil, lastStatusCode, fmt.Errorf("GET %s: %w", pathDesc, err)
		}

		c.logAPI(
			"opengrok %s response method=%s url=%s status=%s duration=%s",
			logKind,
			req.Method,
			req.URL.Redacted(),
			resp.Status,
			time.Since(start),
		)

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("GET %s: read response: %w", pathDesc, readErr)
		}
		if len(body) > maxResponseBytes {
			return nil, resp.StatusCode, fmt.Errorf("GET %s: response exceeds %d-byte limit", pathDesc, maxResponseBytes)
		}

		lastBody = body
		lastStatusCode = resp.StatusCode

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return body, resp.StatusCode, nil
		}

		if attempt < maxAttempts && isRetryableStatus(resp.StatusCode) {
			c.logAPI(
				"opengrok %s retry method=GET url=%s status=%s attempt=%d delay=%s",
				logKind, req.URL.Redacted(), resp.Status, attempt+1, baseDelay,
			)
			if err := c.sleepWithContext(ctx, baseDelay, attempt); err != nil {
				return body, resp.StatusCode, err
			}
			continue
		}

		return body, resp.StatusCode, &StatusError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Path:   pathDesc,
		}
	}

	return lastBody, lastStatusCode, &StatusError{
		Code: lastStatusCode,
		Path: pathDesc,
	}
}

// sleepWithContext waits for an exponentially increasing delay while respecting
// context cancellation. The delay is baseDelay * 2^(attempt-1).
func (c *Client) sleepWithContext(ctx context.Context, baseDelay time.Duration, attempt int) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	delay := baseDelay * (1 << (attempt - 1))
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableStatus(statusCode int) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	return false
}

func (c *Client) logAPI(format string, args ...any) {
	if c.debugLogf == nil {
		return
	}

	c.debugLogf(format, args...)
}

func (c *Client) addAuth(request *http.Request) {
	if c.authorizationHeader != "" {
		request.Header.Set("Authorization", c.authorizationHeader)
	}
}

func authHeaderValue(scheme string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	prefix := scheme + " "
	if len(value) > len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
		return scheme + " " + strings.TrimSpace(value[len(prefix):])
	}

	return scheme + " " + value
}

func modeQueryParam(mode Mode) string {
	switch mode {
	case ModeDefinition:
		return "def"
	case ModeReference:
		return "refs"
	case ModePath:
		return "path"
	case ModeHistory:
		return "hist"
	default:
		return "full"
	}
}

type searchResponse struct {
	ResultCount   int                    `json:"resultCount"`
	StartDocument int                    `json:"startDocument"`
	EndDocument   int                    `json:"endDocument"`
	Results       map[string][]searchHit `json:"results"`
}

type searchHit struct {
	Line       string             `json:"line"`
	LineNumber flexibleLineNumber `json:"lineNumber"`
	Tag        string             `json:"tag"`
}

func (r searchResponse) toResult(projects []string, defaultProject string) SearchResult {
	result := SearchResult{
		TotalHits: r.ResultCount,
		Start:     r.StartDocument,
		End:       r.EndDocument,
		Hits:      []Hit{},
	}

	for path, hits := range r.Results {
		project, filePath, source, uncertain := normalizePath(path, projects, defaultProject)
		var attributionWarning string
		if uncertain {
			attributionWarning = "OpenGrok result path did not match any requested project; pass an explicit project or narrow the query."
		}
		for _, hit := range hits {
			snippet := hit.Line
			result.Hits = append(result.Hits, Hit{
				Project:              project,
				FilePath:             filePath,
				LineNumber:           int(hit.LineNumber),
				Snippet:              &snippet,
				Tag:                  hit.Tag,
				AttributionUncertain: uncertain,
				AttributionWarning:   attributionWarning,
				AttributionSource:    source,
			})
		}
	}

	return result
}

func normalizePath(path string, projects []string, defaultProject string) (string, string, string, bool) {
	cleanPath := strings.TrimPrefix(path, "/")
	orderedProjects := append([]string{}, projects...)
	sort.SliceStable(orderedProjects, func(i, j int) bool {
		return len(orderedProjects[i]) > len(orderedProjects[j])
	})
	for _, project := range orderedProjects {
		prefix := project + "/"
		if strings.HasPrefix(cleanPath, prefix) {
			return project, strings.TrimPrefix(cleanPath, prefix), "matched_prefix", false
		}
	}

	if len(projects) == 0 {
		if slash := strings.Index(cleanPath, "/"); slash > 0 && slash < len(cleanPath)-1 {
			return cleanPath[:slash], cleanPath[slash+1:], "path_first_segment", false
		}
		if defaultProject != "" {
			return defaultProject, cleanPath, "default_project_fallback", false
		}
		return "", cleanPath, "unknown", false
	}

	if len(projects) == 1 {
		return projects[0], cleanPath, "first_project_fallback", false
	}

	return "", cleanPath, "unknown", true
}

type flexibleLineNumber int

func (n *flexibleLineNumber) UnmarshalJSON(data []byte) error {
	var number int
	if err := json.Unmarshal(data, &number); err == nil {
		*n = flexibleLineNumber(number)
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("lineNumber: %w", err)
	}
	if text == "" {
		*n = 0
		return nil
	}

	parsed, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("lineNumber %q: %w", text, err)
	}
	*n = flexibleLineNumber(parsed)
	return nil
}
