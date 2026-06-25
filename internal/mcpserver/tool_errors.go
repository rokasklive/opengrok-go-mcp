// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

const (
	codeFileNotFound         = "FILE_NOT_FOUND"
	codeUpstreamHTTP         = "UPSTREAM_HTTP_ERROR"
	codeSearchFailed         = "SEARCH_FAILED"
	codeInvalidSort          = "INVALID_SORT"
	codeMissingRequiredField = "MISSING_REQUIRED_FIELD"
	codeInvalidFieldType     = "INVALID_FIELD_TYPE"
	codeUnknownField         = "UNKNOWN_FIELD"
	codeQueryParserFailed    = "QUERY_PARSER_FAILED"
)

// ToolErrorBody is the structured tool-error payload returned on failed calls.
type ToolErrorBody struct {
	ErrorCode  string         `json:"error_code"`
	Message    string         `json:"message"`
	Suggestion string         `json:"suggestion,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

type queryParserError struct {
	query string
	err   error
}

func (e *queryParserError) Error() string {
	return fmt.Sprintf("opengrok query parser rejected %q: %v", e.query, e.err)
}

func (e *queryParserError) Unwrap() error {
	return e.err
}

func wrapToolHandler[In, Out any](handler mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		res, out, err := handler(ctx, req, in)
		if err == nil {
			return res, out, nil
		}
		errRes, mapErr := structuredToolErrorResult(err)
		if mapErr != nil {
			var zero Out
			return nil, zero, mapErr
		}
		var zero Out
		return errRes, zero, nil
	}
}

func structuredToolErrorResult(err error) (*mcp.CallToolResult, error) {
	return structuredToolErrorBodyResult(mapToolError(err))
}

func structuredToolErrorBodyResult(body ToolErrorBody) (*mcp.CallToolResult, error) {
	if _, err := json.Marshal(body); err != nil {
		return nil, fmt.Errorf("marshal tool error: %w", err)
	}
	return &mcp.CallToolResult{
		IsError:           true,
		StructuredContent: body,
		Content:           []mcp.Content{&mcp.TextContent{Text: body.Message}},
	}, nil
}

func mapToolError(err error) ToolErrorBody {
	var queryErr *queryParserError
	if errors.As(err, &queryErr) {
		details := map[string]any{"query": queryErr.query}
		var status *opengrok.StatusError
		if errors.As(err, &status) {
			details["http_status"] = status.Code
			details["path"] = status.Path
		}
		return ToolErrorBody{
			ErrorCode: codeQueryParserFailed,
			Message:   fmt.Sprintf("OpenGrok could not parse query %q.", queryErr.query),
			Suggestion: fmt.Sprintf(
				"OpenGrok could not parse %q. Wrap regex in `/.../`; quote phrases; `*` and `?` are wildcards, not regex; see opengrok://capabilities.",
				queryErr.query,
			),
			Details: details,
		}
	}

	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return ToolErrorBody{
			ErrorCode:  serviceErr.Code,
			Message:    serviceErr.Message,
			Suggestion: serviceErr.Suggestion,
		}
	}

	var status *opengrok.StatusError
	if errors.As(err, &status) {
		code := codeUpstreamHTTP
		message := status.Error()
		switch status.Code {
		case http.StatusNotFound:
			code = codeFileNotFound
			message = fmt.Sprintf(
				"Resource not found at %s. Verify project and file_path from a search result, or narrow the query.",
				status.Path,
			)
		case http.StatusBadRequest:
			code = codeQueryParserFailed
			message = fmt.Sprintf("OpenGrok could not parse the query sent to %s.", status.Path)
		case http.StatusUnauthorized, http.StatusForbidden:
			message = fmt.Sprintf(
				"OpenGrok denied access to %s (%s). Check API credentials and project permissions.",
				status.Path,
				status.Status,
			)
		}
		return ToolErrorBody{
			ErrorCode:  code,
			Message:    message,
			Suggestion: suggestionForStatus(code),
			Details: map[string]any{
				"http_status": status.Code,
				"path":        status.Path,
			},
		}
	}

	return ToolErrorBody{
		ErrorCode: codeSearchFailed,
		Message:   err.Error(),
	}
}

func newQueryParserError(query string, err error) error {
	return &queryParserError{query: query, err: err}
}

func isQueryParserStatus(err error) bool {
	var status *opengrok.StatusError
	return errors.As(err, &status) && status.Code == http.StatusBadRequest
}

func suggestionForStatus(code string) string {
	switch code {
	case codeQueryParserFailed:
		return "Wrap regex in `/.../`; quote phrases; `*` and `?` are wildcards, not regex; see opengrok://capabilities."
	default:
		return ""
	}
}

func mapFileContentError(err error) error {
	if mapped := mapToolError(err); mapped.ErrorCode == codeFileNotFound || mapped.ErrorCode == codeUpstreamHTTP {
		return &Error{Code: mapped.ErrorCode, Message: mapped.Message}
	}
	return fmt.Errorf("file context: %w", err)
}
