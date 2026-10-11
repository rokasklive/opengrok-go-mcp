// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

func TestGetFileContextRejectsParentSegments(t *testing.T) {
	backend := &fakeBackend{fileErr: fmt.Errorf("file content: %w", opengrok.ErrInvalidFilePath)}
	service := NewService(testConfig(), backend)

	_, err := service.GetFileContext(context.Background(), FileContextInput{
		Project:  "platform",
		FilePath: "../other/secret.txt",
	})
	if err == nil {
		t.Fatal("GetFileContext error = nil, want INVALID_FILE_PATH")
	}
	body := mapToolError(err)
	if body.ErrorCode != codeInvalidFilePath || body.Suggestion == "" {
		t.Fatalf("error = %+v, want %s with a suggestion", body, codeInvalidFilePath)
	}
}
