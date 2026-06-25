// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "encoding/json"

type SearchCodeInput struct {
	Project          string   `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Projects         []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	Query            string   `json:"query" jsonschema:"REQUIRED. The search query. Multi-word queries are auto-quoted as an exact phrase by default — usually the most precise match for code; set tokenized=true only to match the words as independent terms. Inline Lucene syntax is supported, e.g. -path:legacy or defs:ClassName."`
	Mode             string   `json:"mode,omitempty" jsonschema:"optional search field: full_text (default), path, history, definition, or reference. Use path for file-name searches."`
	PathPrefix       string   `json:"path_prefix,omitempty" jsonschema:"optional path substring to restrict results TO (e.g. \"src/\"); results must contain this in their path"`
	FileType         string   `json:"file_type,omitempty" jsonschema:"optional file extension/type filter (e.g. \"java\"); omit to search all file types"`
	Tokenized        *bool    `json:"tokenized,omitempty" jsonschema:"optional. By default a multi-word query with no operators is auto-quoted as an exact phrase (\"extends Foo\"), which returns far fewer, more relevant results. Set true to instead search the words as independent terms (bag-of-words)."`
	PathExclude      string   `json:"path_exclude,omitempty" jsonschema:"optional path substring(s) to EXCLUDE from results; space-separate multiple values (e.g. \"test legacy\") and each becomes a Lucene -path: exclusion. Distinct from path_prefix, which restricts results TO a path."`
	PageSize         int      `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	Cursor           *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor; pass the same query to fetch the next page"`
	IncludeLinks     *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
	IncludeSnippets  *bool    `json:"include_snippets,omitempty" jsonschema:"optional; set false to omit match snippets from results"`
	MaxHitsPerFile   int      `json:"max_hits_per_file,omitempty" jsonschema:"optional maximum results per file; 0 means no limit"`
	Sort             string   `json:"sort,omitempty" jsonschema:"optional sort order: relevance (default), path, or date"`
	ExpandContext    *bool    `json:"expand_context,omitempty" jsonschema:"optional; set true to include extra lines of file context around each match"`
	AllowAllProjects *bool    `json:"allow_all_projects,omitempty" jsonschema:"explicitly allow searching across all projects, bypassing the configured project list"`
	ResponseMode     string   `json:"response_mode,omitempty" jsonschema:"optional; compact skips context expansion and omits redundant URL/title fields (citation is kept); default full"`
	ContextBudget    string   `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
}

type SymbolSearchInput struct {
	Project          string   `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Projects         []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	Symbol           string   `json:"symbol" jsonschema:"REQUIRED. The symbol name to search for, matched as a Lucene term in OpenGrok's symbol index (e.g. PaymentProcessor or processPayment). Pass a bare name; do not wrap it in quotes."`
	PageSize         int      `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	Cursor           *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor; pass the same symbol to fetch the next page"`
	IncludeLinks     *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
	IncludeSnippets  *bool    `json:"include_snippets,omitempty" jsonschema:"optional; set false to omit match snippets from results"`
	MaxHitsPerFile   int      `json:"max_hits_per_file,omitempty" jsonschema:"optional maximum results per file; 0 means no limit"`
	Sort             string   `json:"sort,omitempty" jsonschema:"optional sort order: relevance (default), path, or date"`
	ExpandContext    *bool    `json:"expand_context,omitempty" jsonschema:"optional; set true to include extra lines of file context around each match"`
	AllowAllProjects *bool    `json:"allow_all_projects,omitempty" jsonschema:"explicitly allow searching across all projects, bypassing the configured project list"`
	ResponseMode     string   `json:"response_mode,omitempty" jsonschema:"optional; compact skips context expansion and omits redundant URL/title fields (citation is kept); default full"`
	ContextBudget    string   `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
}

// Pagination is embedded anonymously into paginated outputs so its fields
// appear at the top level of the JSON response. TotalHits is the global,
// unfiltered count reported by OpenGrok for the query.
type Pagination struct {
	PageSize   int     `json:"page_size"`
	Page       int     `json:"page"`        // 1-based index of the page in this response
	TotalPages int     `json:"total_pages"` // ceil(total_hits / page_size); 0 when total_hits == 0
	TotalHits  int     `json:"total_hits"`  // global, unfiltered hit count from OpenGrok
	HasMore    bool    `json:"has_more"`    // true when more pages exist (next_cursor != nil)
	NextCursor *string `json:"next_cursor"` // always present (null when no further pages)
}

type SearchOutput struct {
	Project    string   `json:"project"`
	Mode       string   `json:"mode"`
	Query      string   `json:"query"`
	Pagination          // embedded: page_size, page, total_pages, total_hits, has_more, next_cursor
	Results    []Result `json:"results"`
	WarningFields
	BestEffort  *bool                 `json:"best_effort,omitempty"`
	Diagnostics *Diagnostics          `json:"diagnostics,omitempty"`
	Expansion   *ExpansionDiagnostics `json:"expansion,omitempty"`
}

type Diagnostics struct {
	OffsetUsed         int `json:"offset_used"`
	OpenGrokStart      int `json:"opengrok_start"`
	OpenGrokMaxResults int `json:"opengrok_max_results"`
}

type ExpansionDiagnostics struct {
	Requested            int `json:"requested"`
	ExpandedResults      int `json:"expanded_results"`
	SkippedResults       int `json:"skipped_results"`
	FetchedFiles         int `json:"fetched_files"`
	SkippedFiles         int `json:"skipped_files"`
	FetchConcurrency     int `json:"fetch_concurrency"`
	ExpandedContextBytes int `json:"expanded_context_bytes"`
}

type Result struct {
	ResultID             string         `json:"result_id"`
	Project              string         `json:"project"`
	FilePath             string         `json:"file_path"`
	AttributionUncertain bool           `json:"attribution_uncertain,omitempty"`
	AttributionWarning   *string        `json:"attribution_warning,omitempty"`
	AttributionSource    string         `json:"attribution_source,omitempty"`
	LineNumber           int            `json:"line_number"`
	ColumnNumber         *int           `json:"column_number,omitempty"`
	Kind                 string         `json:"kind,omitempty"`
	Symbol               *string        `json:"symbol"`
	Snippet              *string        `json:"snippet,omitempty"`
	DisplayTitle         string         `json:"display_title,omitempty"`
	DisplayURL           string         `json:"display_url,omitempty"`
	RawURL               *string        `json:"raw_url"`
	Citation             Citation       `json:"citation"`
	ResourceURI          string         `json:"resource_uri,omitempty"`
	Score                *float64       `json:"score,omitempty"`
	Context              *ResultContext `json:"context,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type Citation struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Line  int    `json:"line,omitempty"`
	// Markdown is a ready-to-surface clickable link, "[title](url)", populated
	// only when a URL is available. Agents should render this in findings so the
	// user gets a clickable source citation rather than a bare or dropped URL.
	Markdown string `json:"markdown,omitempty"`
}

type ResultContext struct {
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type ListProjectsInput struct {
	Cursor *string `json:"cursor,omitempty"`
}

type ProjectItem struct {
	Project     string `json:"project"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ProjectURL  string `json:"project_url"`
	ResourceURI string `json:"resource_uri"`
}

type ListProjectsOutput struct {
	Projects          []ProjectItem `json:"projects"`
	TotalProjects     int           `json:"total_projects"`
	NextCursor        *string       `json:"next_cursor"`
	CatalogSource     string        `json:"catalog_source"`
	CatalogIsSnapshot bool          `json:"catalog_is_snapshot"`
}

type FileContextInput struct {
	Project            string  `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	FilePath           string  `json:"file_path" jsonschema:"REQUIRED. Project-relative file path, e.g. a search result's file_path"`
	LineNumber         int     `json:"line_number,omitempty" jsonschema:"optional line to center the context window on (e.g. a search result's line_number); omit to read the whole file (paginated via cursor)"`
	Before             int     `json:"before,omitempty" jsonschema:"optional lines of context before line_number; only applies when line_number is set; 0 uses the budget default"`
	After              int     `json:"after,omitempty" jsonschema:"optional lines of context after line_number; only applies when line_number is set; 0 uses the budget default"`
	IncludeAnnotations bool    `json:"include_annotations,omitempty" jsonschema:"optional; set true to include per-line annotation/blame metadata when available"`
	IncludeLinks       *bool   `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from the result"`
	Cursor             *string `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor (for full-file reads); pass the same file_path to continue"`
	ContextBudget      string  `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
}

// ReadFileInput is the compact opengrok_read operation=file branch (whole-file read).
type ReadFileInput struct {
	Project            string  `json:"project,omitempty" jsonschema:"optional project"`
	FilePath           string  `json:"file_path" jsonschema:"REQUIRED. project-relative path"`
	Cursor             *string `json:"cursor,omitempty" jsonschema:"pagination cursor from next_cursor"`
	IncludeAnnotations bool    `json:"include_annotations,omitempty"`
	IncludeLinks       *bool   `json:"include_links,omitempty"`
	ContextBudget      string  `json:"context_budget,omitempty"`
}

// ReadContextInput is the compact opengrok_read operation=context branch.
type ReadContextInput struct {
	Project            string `json:"project,omitempty" jsonschema:"optional project"`
	FilePath           string `json:"file_path" jsonschema:"REQUIRED. project-relative path"`
	LineNumber         int    `json:"line_number" jsonschema:"REQUIRED. line from a search or symbol result"`
	Before             int    `json:"before,omitempty"`
	After              int    `json:"after,omitempty"`
	IncludeAnnotations bool   `json:"include_annotations,omitempty"`
	IncludeLinks       *bool  `json:"include_links,omitempty"`
	ContextBudget      string `json:"context_budget,omitempty"`
}

type FileContextOutput struct {
	Project              string   `json:"project"`
	FilePath             string   `json:"file_path"`
	LineNumber           int      `json:"line_number"`
	StartLine            int      `json:"start_line"`
	EndLine              int      `json:"end_line"`
	TotalLines           int      `json:"total_lines"`
	Truncated            bool     `json:"truncated"`
	Content              string   `json:"content"`
	DisplayURL           string   `json:"display_url"`
	RawURL               *string  `json:"raw_url"`
	Citation             Citation `json:"citation"`
	NextCursor           *string  `json:"next_cursor,omitempty"`
	Hint                 *string  `json:"hint,omitempty"`
	AnnotationsAvailable bool     `json:"annotations_available"`
	ResourceURI          string   `json:"resource_uri"`
}

type ListSymbolsInput struct {
	Project         string   `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Projects        []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	PathPrefix      string   `json:"path_prefix,omitempty" jsonschema:"optional path substring to scope the listing to (e.g. \"src/api/\")"`
	Kind            string   `json:"kind,omitempty" jsonschema:"optional ctags kind filter, e.g. class, interface, function, method, field; omit for all kinds"`
	Symbol          string   `json:"symbol,omitempty" jsonschema:"optional symbol-name filter, matched as a Lucene term; omit to list all symbols in scope"`
	FileType        string   `json:"file_type,omitempty" jsonschema:"optional file extension/type filter (e.g. \"java\"); omit for all file types"`
	PageSize        int      `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	IncludeLinks    *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
	IncludeSnippets *bool    `json:"include_snippets,omitempty" jsonschema:"optional; set false to omit snippets to reduce token cost on large sweeps"`
	Cursor          *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor"`
}

type ListSymbolsOutput struct {
	Symbols    []SymbolItem `json:"symbols"`
	Pagination              // page_size, page, total_pages, total_hits, has_more, next_cursor
	WarningFields
	KindFilterActive  bool   `json:"kind_filter_active,omitempty"`
	KindMatchesOnPage int    `json:"kind_matches_on_page"`
	TotalHitsScope    string `json:"total_hits_scope,omitempty"`
}

type SymbolItem struct {
	Project     string  `json:"project"`
	FilePath    string  `json:"file_path"`
	Kind        string  `json:"kind"`
	LineNumber  int     `json:"line_number"`
	Snippet     *string `json:"snippet"`
	ResourceURI string  `json:"resource_uri"`
	DisplayURL  string  `json:"display_url,omitempty"`
}

type ListFilesInput struct {
	Project      string  `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Path         string  `json:"path,omitempty" jsonschema:"optional project-relative directory to list (e.g. \"src/api\"); omit for the project root"`
	Kind         *string `json:"kind,omitempty" jsonschema:"optional filter: file, directory, or both (default)"`
	PageSize     int     `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	Cursor       *string `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor"`
	IncludeLinks *bool   `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
}

type FileItem struct {
	Project     string `json:"project"`
	Path        string `json:"path"`
	Name        string `json:"name"`
	IsDirectory bool   `json:"is_directory"`
	NumLines    int    `json:"num_lines,omitempty"`
	Loc         int    `json:"loc,omitempty"`
	Size        *int64 `json:"size,omitempty"`
	Description string `json:"description,omitempty"`
	DisplayURL  string `json:"display_url,omitempty"`
	ResourceURI string `json:"resource_uri,omitempty"`
}

type ListFilesOutput struct {
	Project    string     `json:"project"`
	Path       string     `json:"path"`
	Files      []FileItem `json:"files"`
	Pagination            // page_size, page, total_pages, total_hits, has_more, next_cursor
	Truncated  bool       `json:"truncated"`
	WarningFields
}

type ProjectOverviewInput struct {
	Project string `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
}

type LanguageStat struct {
	Language string  `json:"language"`
	Files    int     `json:"files"`
	Lines    int     `json:"lines"`
	Percent  float64 `json:"percent"`
}

type ProjectOverviewOutput struct {
	Project     string     `json:"project"`
	TotalFiles  int        `json:"total_files"`
	TotalDirs   int        `json:"total_dirs"`
	TopDirs     []FileItem `json:"top_dirs"`
	TopFiles    []FileItem `json:"top_files"`
	Description string     `json:"description,omitempty"`
	Truncated   bool       `json:"truncated"`
	WarningFields
	Languages    []LanguageStat `json:"languages,omitempty"`
	TotalSymbols int            `json:"total_symbols,omitempty"`
	LastIndexed  *string        `json:"last_indexed,omitempty"`
}

type ImplementationSearchInput struct {
	Project          string   `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Projects         []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	Symbol           string   `json:"symbol" jsonschema:"REQUIRED. The symbol name whose implementations/usages to find, matched as a Lucene term (e.g. PaymentProcessor). Pass a bare name; do not wrap it in quotes."`
	PageSize         int      `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	Cursor           *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor; pass the same symbol to fetch the next page"`
	IncludeLinks     *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
	ExpandContext    *bool    `json:"expand_context,omitempty" jsonschema:"optional; set true to include extra lines of file context around each match"`
	MaxHitsPerFile   int      `json:"max_hits_per_file,omitempty" jsonschema:"optional maximum results per file; 0 means no limit"`
	Sort             string   `json:"sort,omitempty" jsonschema:"optional sort order: relevance (default), path, or date"`
	AllowAllProjects *bool    `json:"allow_all_projects,omitempty" jsonschema:"explicitly allow searching across all projects, bypassing the configured project list"`
	ResponseMode     string   `json:"response_mode,omitempty" jsonschema:"optional; compact skips context expansion and omits redundant URL/title fields (citation is kept); default full"`
	ContextBudget    string   `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
}

type CrossProjectReferencesInput struct {
	Symbol           string   `json:"symbol" jsonschema:"REQUIRED. The symbol name to find references to across projects, matched as a Lucene term (e.g. PaymentProcessor). Pass a bare name; do not wrap it in quotes."`
	Projects         []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	PageSize         int      `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	Cursor           *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor; pass the same symbol to fetch the next page"`
	IncludeLinks     *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
	ExpandContext    *bool    `json:"expand_context,omitempty" jsonschema:"optional; set true to include extra lines of file context around each match"`
	MaxHitsPerFile   int      `json:"max_hits_per_file,omitempty" jsonschema:"optional maximum results per file; 0 means no limit"`
	Sort             string   `json:"sort,omitempty" jsonschema:"optional sort order: relevance (default), path, or date"`
	AllowAllProjects *bool    `json:"allow_all_projects,omitempty" jsonschema:"explicitly allow searching across all projects, bypassing the configured project list"`
	ResponseMode     string   `json:"response_mode,omitempty" jsonschema:"optional; compact skips context expansion and omits redundant URL/title fields (citation is kept); default full"`
	ContextBudget    string   `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
}

type ProjectReferenceGroup struct {
	Project string   `json:"project"`
	Results []Result `json:"results"`
	Total   int      `json:"total"`
}

type GatewayDiscoverInput struct{}

type GatewayOperation struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Examples    []string `json:"examples,omitempty"`
}

type GatewayDiscoverOutput struct {
	Operations []GatewayOperation `json:"operations"`
}

type GatewayCallInput struct {
	Operation string          `json:"operation"`
	Payload   json.RawMessage `json:"payload"`
}

type GatewayCallOutput struct {
	Operation string `json:"operation"`
	Result    any    `json:"result"`
}

type CrossProjectReferencesOutput struct {
	Symbol     string                  `json:"symbol"`
	Projects   []ProjectReferenceGroup `json:"projects"`
	TotalHits  int                     `json:"total_hits"`
	PageSize   int                     `json:"page_size"`
	NextCursor *string                 `json:"next_cursor,omitempty"`
	WarningFields
	Diagnostics *Diagnostics `json:"diagnostics,omitempty"`
}

type MemorySetInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MemorySetOutput struct {
	Success bool `json:"success"`
}

type MemoryGetInput struct {
	Key string `json:"key"`
}

type MemoryGetOutput struct {
	Value string `json:"value,omitempty"`
	Found bool   `json:"found"`
}

type MemoryListInput struct{}

type MemoryListOutput struct {
	Entries map[string]string `json:"entries"`
}

type MemoryDeleteInput struct {
	Key string `json:"key"`
}

type MemoryDeleteOutput struct {
	Found   bool `json:"found"`
	Deleted bool `json:"deleted"`
}

type MemoryClearInput struct{}

type MemoryClearOutput struct {
	Success bool `json:"success"`
}

type CompactMemoryInput struct {
	Operation string          `json:"operation" jsonschema:"one of: set, get, list, delete, clear"`
	Payload   json.RawMessage `json:"payload"`
}

type SearchAndReadInput struct {
	Project          string   `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Projects         []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	Query            string   `json:"query" jsonschema:"REQUIRED. The search query. Multi-word queries are auto-quoted as an exact phrase by default — usually the most precise match for code; set tokenized=true only to match the words as independent terms. Inline Lucene syntax is supported."`
	Mode             string   `json:"mode,omitempty" jsonschema:"optional search field: full_text (default), path, history, definition, or reference. Use path for file-name searches."`
	PathPrefix       string   `json:"path_prefix,omitempty" jsonschema:"optional path substring to restrict results TO"`
	FileType         string   `json:"file_type,omitempty" jsonschema:"optional file extension/type filter; omit to search all file types"`
	Tokenized        *bool    `json:"tokenized,omitempty" jsonschema:"optional. By default a multi-word query with no operators is auto-quoted as an exact phrase; set true to search the words as independent terms."`
	PathExclude      string   `json:"path_exclude,omitempty" jsonschema:"optional path substring(s) to EXCLUDE from results; space-separate multiple values and each becomes a Lucene -path: exclusion. Distinct from path_prefix, which restricts results TO a path."`
	MaxResults       int      `json:"max_results,omitempty" jsonschema:"optional maximum results to read; 0 means read all"`
	LinesBefore      int      `json:"lines_before,omitempty" jsonschema:"optional lines of context before each match; 0 uses the budget default"`
	LinesAfter       int      `json:"lines_after,omitempty" jsonschema:"optional lines of context after each match; 0 uses the budget default"`
	IncludeLinks     *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs"`
	IncludeSnippets  *bool    `json:"include_snippets,omitempty" jsonschema:"optional; set false to omit match snippets"`
	ResponseMode     string   `json:"response_mode,omitempty" jsonschema:"optional; compact skips context expansion and omits redundant URL/title fields (citation is kept); default full"`
	ContextBudget    string   `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
	Cursor           *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor"`
	PageSize         int      `json:"page_size,omitempty" jsonschema:"optional results per page; omit for the server default"`
	AllowAllProjects *bool    `json:"allow_all_projects,omitempty" jsonschema:"explicitly allow searching across all projects, bypassing the configured project list"`
}

type SearchAndReadOutput struct {
	Project    string                `json:"project"`
	Mode       string                `json:"mode"`
	Query      string                `json:"query"`
	TotalHits  int                   `json:"total_hits"`
	Results    []SearchAndReadResult `json:"results"`
	PageSize   int                   `json:"page_size"`
	NextCursor *string               `json:"next_cursor,omitempty"`
	WarningFields
	Diagnostics *Diagnostics `json:"diagnostics,omitempty"`
}

type SearchAndReadResult struct {
	ResultID    string   `json:"result_id"`
	Project     string   `json:"project"`
	FilePath    string   `json:"file_path"`
	LineNumber  int      `json:"line_number"`
	Kind        string   `json:"kind,omitempty"`
	Symbol      *string  `json:"symbol,omitempty"`
	Snippet     *string  `json:"snippet,omitempty"`
	Content     string   `json:"content"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	Citation    Citation `json:"citation"`
	ResourceURI string   `json:"resource_uri,omitempty"`
}

type FindSymbolAndReferencesInput struct {
	Project          string   `json:"project,omitempty" jsonschema:"optional OpenGrok project override; omit unless the user explicitly names an OpenGrok project"`
	Projects         []string `json:"projects,omitempty" jsonschema:"optional OpenGrok project overrides; omit unless the user explicitly names OpenGrok projects"`
	Symbol           string   `json:"symbol" jsonschema:"REQUIRED. The symbol name to look up (its definition plus references), matched as a Lucene term (e.g. PaymentProcessor). Pass a bare name; do not wrap it in quotes."`
	PageSize         int      `json:"page_size,omitempty" jsonschema:"optional results per page for the references list; omit for the server default"`
	Cursor           *string  `json:"cursor,omitempty" jsonschema:"optional pagination cursor from a previous response's next_cursor; pass the same symbol to fetch the next page of references"`
	IncludeLinks     *bool    `json:"include_links,omitempty" jsonschema:"optional; set false to omit display/raw URLs from results"`
	IncludeSnippets  *bool    `json:"include_snippets,omitempty" jsonschema:"optional; set false to omit match snippets from results"`
	ResponseMode     string   `json:"response_mode,omitempty" jsonschema:"optional; compact skips context expansion and omits redundant URL/title fields (citation is kept); default full"`
	ContextBudget    string   `json:"context_budget,omitempty" jsonschema:"optional context expansion budget tier: minimal (few lines, few results), default (balanced), or maximal (many lines, many results)"`
	AllowAllProjects *bool    `json:"allow_all_projects,omitempty" jsonschema:"explicitly allow searching across all projects, bypassing the configured project list"`
}

type FindSymbolAndReferencesOutput struct {
	Symbol     string               `json:"symbol"`
	Definition *SearchAndReadResult `json:"definition,omitempty"`
	References []Result             `json:"references"`
	TotalRefs  int                  `json:"total_references"`
	PageSize   int                  `json:"page_size"`
	NextCursor *string              `json:"next_cursor,omitempty"`
	WarningFields
	Diagnostics *Diagnostics `json:"diagnostics,omitempty"`
}
