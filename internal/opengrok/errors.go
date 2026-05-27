// SPDX-License-Identifier: Apache-2.0

package opengrok

import "fmt"

// StatusError carries an HTTP status code from a non-2xx OpenGrok response.
type StatusError struct {
	Code   int
	Status string
	Path   string
}

func (e *StatusError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("GET %s: unexpected status %s", e.Path, e.Status)
	}
	return fmt.Sprintf("GET %s: unexpected status code %d", e.Path, e.Code)
}
