package mcpserver

type SearchCodeInput struct {
	Project      string   `json:"project"`
	Projects     []string `json:"projects"`
	Query        string   `json:"query"`
	Mode         string   `json:"mode"`
	PathPrefix   string   `json:"path_prefix"`
	FileType     string   `json:"file_type"`
	PageSize     int      `json:"page_size"`
	Cursor       string   `json:"cursor"`
	IncludeLinks *bool    `json:"include_links"`
}

type SymbolSearchInput struct {
	Project      string   `json:"project"`
	Projects     []string `json:"projects"`
	Symbol       string   `json:"symbol"`
	PageSize     int      `json:"page_size"`
	Cursor       string   `json:"cursor"`
	IncludeLinks *bool    `json:"include_links"`
}

type SearchOutput struct {
	Project     string      `json:"project"`
	Mode        string      `json:"mode"`
	Query       string      `json:"query"`
	TotalHits   int         `json:"total_hits"`
	Results     []Result    `json:"results"`
	PageSize    int         `json:"page_size"`
	NextCursor  *string     `json:"next_cursor"`
	Diagnostics Diagnostics `json:"diagnostics"`
}

type Diagnostics struct {
	OffsetUsed         int `json:"offset_used"`
	OpenGrokStart      int `json:"opengrok_start"`
	OpenGrokMaxResults int `json:"opengrok_max_results"`
}

type Result struct {
	ResultID     string         `json:"result_id"`
	Project      string         `json:"project"`
	FilePath     string         `json:"file_path"`
	LineNumber   int            `json:"line_number"`
	ColumnNumber *int           `json:"column_number"`
	Kind         string         `json:"kind"`
	Symbol       *string        `json:"symbol"`
	Snippet      string         `json:"snippet"`
	DisplayTitle string         `json:"display_title"`
	DisplayURL   string         `json:"display_url"`
	RawURL       *string        `json:"raw_url"`
	ResourceURI  string         `json:"resource_uri"`
	Score        *float64       `json:"score"`
	Metadata     map[string]any `json:"metadata"`
}

type ListProjectsInput struct {
	Cursor string `json:"cursor"`
}

type ProjectItem struct {
	Project     string `json:"project"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ProjectURL  string `json:"project_url"`
	ResourceURI string `json:"resource_uri"`
}

type ListProjectsOutput struct {
	Projects   []ProjectItem `json:"projects"`
	NextCursor *string       `json:"next_cursor"`
}

type FileContextInput struct {
	Project  string `json:"project"`
	FilePath string `json:"file_path"`
}

type FileContextOutput struct {
	Project     string  `json:"project"`
	FilePath    string  `json:"file_path"`
	Content     string  `json:"content"`
	DisplayURL  string  `json:"display_url"`
	RawURL      *string `json:"raw_url"`
	ResourceURI string  `json:"resource_uri"`
}
