// SPDX-License-Identifier: Apache-2.0

package opengrok

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// AuthRejectedError reports that OpenGrok rejected the configured credentials.
// Once it happens the client sends no further requests with them for the life
// of the process: behind OpenGrok is usually a directory (LDAP/AD), where every
// rejected request counts as a failed login, so startup probes, agent retries
// and reconnects would otherwise lock the account. Restarting the server is the
// deliberate way to try the credentials again.
type AuthRejectedError struct {
	Method string
	Path   string
	At     time.Time
}

func (e *AuthRejectedError) Error() string {
	return fmt.Sprintf(
		"OpenGrok rejected the configured credentials (HTTP 401 on %s %s at %s); no further requests are sent with them until the server restarts",
		e.Method, e.Path, e.At.Format(time.RFC3339),
	)
}

// IsAuthRejection reports whether err is, or wraps, an *AuthRejectedError.
func IsAuthRejection(err error) bool {
	var rejection *AuthRejectedError
	return errors.As(err, &rejection)
}

// AuthRejection returns the credential rejection that stopped this client, or
// nil while its credentials are still accepted (or none are configured).
func (c *Client) AuthRejection() *AuthRejectedError {
	if c.breaker == nil {
		return nil
	}
	return c.breaker.rejected.Load()
}

// installAuthBreaker routes the client's requests through an authBreaker when
// credentials are configured. It copies the http.Client so a shared one (such
// as http.DefaultClient) is never modified.
func (c *Client) installAuthBreaker() {
	if c.authorizationHeader == "" {
		return
	}
	next := c.httpClient.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	c.breaker = &authBreaker{next: next}
	httpClient := *c.httpClient
	httpClient.Transport = c.breaker
	c.httpClient = &httpClient
}

// authBreaker is an http.RoundTripper that stops all traffic after the first
// response rejecting the credentials, and answers every later request with
// that same rejection without contacting the server.
type authBreaker struct {
	next     http.RoundTripper
	rejected atomic.Pointer[AuthRejectedError]
}

func (b *authBreaker) RoundTrip(req *http.Request) (*http.Response, error) {
	if rejection := b.rejected.Load(); rejection != nil {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, rejection
	}

	resp, err := b.next.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || !credentialsRejected(req, resp) {
		return resp, err
	}

	rejection := &AuthRejectedError{Method: req.Method, Path: req.URL.Path, At: time.Now()}
	if !b.rejected.CompareAndSwap(nil, rejection) {
		rejection = b.rejected.Load()
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	return nil, rejection
}

// credentialsRejected reports whether a 401 rejects the credentials themselves
// rather than restricting one endpoint. An authentication layer (Apache,
// Tomcat) answers rejected credentials with a challenge for the scheme that was
// sent, e.g. `WWW-Authenticate: Basic realm="..."`. OpenGrok's own per-endpoint
// API-token check can answer a bare 401 even when the credentials are valid, so
// that does not count — except on search, which OpenGrok never restricts to API
// tokens.
func credentialsRejected(req *http.Request, resp *http.Response) bool {
	scheme, _, ok := strings.Cut(strings.TrimSpace(req.Header.Get("Authorization")), " ")
	if !ok {
		return false
	}
	for _, header := range resp.Header.Values("WWW-Authenticate") {
		for _, challenge := range strings.Split(header, ",") {
			token, _, _ := strings.Cut(strings.TrimSpace(challenge), " ")
			if !strings.Contains(token, "=") && strings.EqualFold(token, scheme) {
				return true
			}
		}
	}
	return strings.HasSuffix(req.URL.Path, "/search")
}
