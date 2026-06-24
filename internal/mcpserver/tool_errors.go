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
	codeFileNotFound   = "FILE_NOT_FOUND"
	codeUpstreamHTTP   = "UPSTREAM_HTTP_ERROR"
	codeSearchFailed   = "SEARCH_FAILED"
	codeInvalidSort    = "INVALID_SORT"
)

// ToolErrorBody is the structured tool-error payload returned on failed calls.
type ToolErrorBody struct {
	ErrorCode string         `json:"error_code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
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
	body := mapToolError(err)
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal tool error: %w", err)
	}
	return &mcp.CallToolResult{
		IsError:           true,
		StructuredContent: json.RawMessage(raw),
		Content:           []mcp.Content{&mcp.TextContent{Text: body.Message}},
	}, nil
}

func mapToolError(err error) ToolErrorBody {
	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return ToolErrorBody{
			ErrorCode: serviceErr.Code,
			Message:   serviceErr.Message,
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
		case http.StatusUnauthorized, http.StatusForbidden:
			message = fmt.Sprintf(
				"OpenGrok denied access to %s (%s). Check API credentials and project permissions.",
				status.Path,
				status.Status,
			)
		}
		return ToolErrorBody{
			ErrorCode: code,
			Message:   message,
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

func mapFileContentError(err error) error {
	if mapped := mapToolError(err); mapped.ErrorCode == codeFileNotFound || mapped.ErrorCode == codeUpstreamHTTP {
		return &Error{Code: mapped.ErrorCode, Message: mapped.Message}
	}
	return fmt.Errorf("file context: %w", err)
}
