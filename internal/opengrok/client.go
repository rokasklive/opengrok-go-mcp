package opengrok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Mode string

const (
	ModeFullText   Mode = "full_text"
	ModeDefinition Mode = "definition"
	ModeReference  Mode = "reference"
	ModePath       Mode = "path"
	ModeHistory    Mode = "history"
)

type Client struct {
	baseURL        string
	httpClient     *http.Client
	apiToken       string
	basicAuthToken string
}

type Option func(*Client)

func WithAPIToken(token string) Option {
	return func(c *Client) {
		c.apiToken = token
	}
}

func WithBasicAuthToken(token string) Option {
	return func(c *Client) {
		c.basicAuthToken = token
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
}

type SearchResult struct {
	TotalHits int
	Start     int
	End       int
	Hits      []Hit
}

type Hit struct {
	Project    string
	FilePath   string
	LineNumber int
	Snippet    string
	Tag        string
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

	body, err := c.do(ctx, "/search", query)
	if err != nil {
		return SearchResult{Hits: []Hit{}}, fmt.Errorf("search: %w", err)
	}

	var response searchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return SearchResult{Hits: []Hit{}}, fmt.Errorf("search: decode response: %w", err)
	}

	return response.toResult(req.Projects), nil
}

func (c *Client) FileContent(ctx context.Context, project string, filePath string) (string, error) {
	query := url.Values{}
	query.Set("project", project)
	query.Set("path", filePath)

	body, err := c.do(ctx, "/file/content", query)
	if err != nil {
		return "", fmt.Errorf("file content: %w", err)
	}

	var response struct {
		Contents string `json:"contents"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return string(body), nil
	}

	return response.Contents, nil
}

func (c *Client) do(ctx context.Context, path string, query url.Values) ([]byte, error) {
	requestURL := c.baseURL + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET %s: %w", path, err)
	}
	c.addAuth(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: read response: %w", path, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("GET %s: unexpected status %s", path, response.Status)
	}

	return body, nil
}

func (c *Client) addAuth(request *http.Request) {
	switch {
	case c.basicAuthToken != "":
		request.Header.Set("Authorization", "Basic "+c.basicAuthToken)
	case c.apiToken != "":
		request.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
}

func modeQueryParam(mode Mode) string {
	switch mode {
	case ModeDefinition:
		return "def"
	case ModeReference:
		return "symbol"
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

func (r searchResponse) toResult(projects []string) SearchResult {
	result := SearchResult{
		TotalHits: r.ResultCount,
		Start:     r.StartDocument,
		End:       r.EndDocument,
		Hits:      []Hit{},
	}

	for path, hits := range r.Results {
		project, filePath := normalizePath(path, projects)
		for _, hit := range hits {
			result.Hits = append(result.Hits, Hit{
				Project:    project,
				FilePath:   filePath,
				LineNumber: int(hit.LineNumber),
				Snippet:    hit.Line,
				Tag:        hit.Tag,
			})
		}
	}

	return result
}

func normalizePath(path string, projects []string) (string, string) {
	cleanPath := strings.TrimPrefix(path, "/")
	for _, project := range projects {
		prefix := project + "/"
		if strings.HasPrefix(cleanPath, prefix) {
			return project, strings.TrimPrefix(cleanPath, prefix)
		}
	}

	if len(projects) > 0 {
		return projects[0], cleanPath
	}

	return "", cleanPath
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
