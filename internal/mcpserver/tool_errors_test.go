// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"errors"
	"net/http"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestMapToolErrorServiceError(t *testing.T) {
	body := mapToolError(&Error{Code: "INVALID_CURSOR", Message: "cursor mismatch"})
	if body.ErrorCode != "INVALID_CURSOR" {
		t.Fatalf("ErrorCode = %q, want INVALID_CURSOR", body.ErrorCode)
	}
}

func TestMapToolErrorFileNotFound(t *testing.T) {
	body := mapToolError(&opengrok.StatusError{
		Code:   http.StatusNotFound,
		Path:   "/raw/platform/src/missing.go",
		Status: "404 Not Found",
	})
	if body.ErrorCode != codeFileNotFound {
		t.Fatalf("ErrorCode = %q, want %s", body.ErrorCode, codeFileNotFound)
	}
	if body.Details["http_status"] != http.StatusNotFound {
		t.Fatalf("details.http_status = %v, want 404", body.Details["http_status"])
	}
}

func TestMapFileContentError(t *testing.T) {
	err := mapFileContentError(&opengrok.StatusError{
		Code:   http.StatusNotFound,
		Path:   "/raw/platform/src/missing.go",
		Status: "404 Not Found",
	})
	var serviceErr *Error
	if !errors.As(err, &serviceErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if serviceErr.Code != codeFileNotFound {
		t.Fatalf("Code = %q, want %s", serviceErr.Code, codeFileNotFound)
	}
}
