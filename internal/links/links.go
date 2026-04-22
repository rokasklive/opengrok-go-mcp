package links

import (
	"net/url"
	"strconv"
	"strings"
)

// FileLinks contains browser and MCP resource links for an OpenGrok file.
type FileLinks struct {
	DisplayURL  string
	RawURL      *string
	ResourceURI string
}

// Builder builds OpenGrok browser URLs and MCP resource URIs.
type Builder struct {
	webBaseURL    string
	enableRawLink bool
}

// NewBuilder returns a Builder with a normalized web base URL.
func NewBuilder(webBaseURL string, enableRawLink bool) Builder {
	return Builder{
		webBaseURL:    strings.TrimRight(webBaseURL, "/"),
		enableRawLink: enableRawLink,
	}
}

// File builds links for a file in an OpenGrok project.
func (b Builder) File(project string, filePath string, lineNumber int) FileLinks {
	escapedProject := url.PathEscape(project)
	escapedFilePath := escapePathSegments(filePath)

	displayURL := b.webBaseURL + "/xref/" + escapedProject + "/" + escapedFilePath
	resourceURI := "opengrok://project/" + escapedProject + "/files/" + escapedFilePath
	if lineNumber > 0 {
		line := strconv.Itoa(lineNumber)
		displayURL += "#" + line
		resourceURI += "#L" + line
	}

	var rawURL *string
	if b.enableRawLink {
		value := b.webBaseURL + "/raw/" + escapedProject + "/" + escapedFilePath
		rawURL = &value
	}

	return FileLinks{
		DisplayURL:  displayURL,
		RawURL:      rawURL,
		ResourceURI: resourceURI,
	}
}

// Project builds an MCP resource URI for an OpenGrok project.
func (b Builder) Project(project string) string {
	return "opengrok://project/" + url.PathEscape(project)
}

// Search builds a browser URL for an OpenGrok search.
func (b Builder) Search(project string, mode string, query string) string {
	values := url.Values{}
	values.Set(searchParam(mode), query)
	values.Set("project", project)

	return b.webBaseURL + "/search?" + values.Encode()
}

func escapePathSegments(filePath string) string {
	segments := strings.Split(filePath, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return strings.Join(segments, "/")
}

func searchParam(mode string) string {
	switch mode {
	case "definition":
		return "def"
	case "reference":
		return "symbol"
	case "path":
		return "path"
	case "history":
		return "hist"
	default:
		return "full"
	}
}
