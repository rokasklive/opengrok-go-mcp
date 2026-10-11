// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

// codeAuthRejected marks a call that failed because OpenGrok rejected the
// configured credentials; no request was sent with them after the first
// rejection.
const codeAuthRejected = "AUTH_REJECTED"

// authRejectedErrorBody tells the agent what happened and that only the user
// can fix it, so it reports the problem instead of retrying.
func authRejectedErrorBody(rejection *opengrok.AuthRejectedError) ToolErrorBody {
	at := rejection.At.Format(time.RFC3339)
	return ToolErrorBody{
		ErrorCode: codeAuthRejected,
		Message: fmt.Sprintf(
			"OpenGrok rejected the configured credentials (HTTP 401 at %s). To avoid locking the account, "+
				"the server sends no further requests with them, so every OpenGrok tool fails until it is restarted.",
			at,
		),
		Suggestion: "Tell the user their OpenGrok login was rejected and that the OpenGrok tools stay unavailable until they act: " +
			"set OPENGROK_MCP_API_TOKEN to \"Basic \" + base64 of login:password (no trailing newline) and restart the OpenGrok MCP server. " +
			"If they changed their password to match the existing token instead, they only need to restart the server. " +
			"Do not retry OpenGrok tools before then.",
		Details: map[string]any{
			"http_status": http.StatusUnauthorized,
			"path":        rejection.Path,
			"rejected_at": at,
		},
	}
}

// authRejectionReporter is implemented by backends that can report a
// credential rejection: the OpenGrok client and the caching wrapper around it.
type authRejectionReporter interface {
	AuthRejection() *opengrok.AuthRejectedError
}

// authRejection returns the backend's credential rejection as an error, or nil.
func (s *Service) authRejection() error {
	if reporter, ok := s.backend.(authRejectionReporter); ok {
		if rejection := reporter.AuthRejection(); rejection != nil {
			return rejection
		}
	}
	return nil
}

// AuthRejection forwards the wrapped backend's credential rejection.
func (c *CachingBackend) AuthRejection() *opengrok.AuthRejectedError {
	if reporter, ok := c.backend.(authRejectionReporter); ok {
		return reporter.AuthRejection()
	}
	return nil
}
