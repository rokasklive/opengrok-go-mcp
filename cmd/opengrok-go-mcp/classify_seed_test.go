// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
	"github.com/rokasklive/opengrok-go-mcp/internal/opengrok"
)

// A successful /projects/indexed call (source=api) proves the credentials work,
// so a 401 on a later probe must classify as endpoint_disabled, not unauthorized
// (R5 / FR-016) — even when the failing probe runs before any search succeeds.
func TestDetectCapabilitiesAPISourceSeedsAuthedProbe(t *testing.T) {
	unauthorized := &opengrok.StatusError{Code: http.StatusUnauthorized}
	backend := &capabilityBackend{
		searchResults: map[opengrok.Mode]error{
			opengrok.ModeFullText:   fmt.Errorf("probe: %w", unauthorized),
			opengrok.ModeDefinition: nil,
			opengrok.ModeReference:  nil,
		},
	}

	cfg := config.Default()
	cfg.DefaultProject = "platform"
	cfg.ProjectSource = config.ProjectSourceAPI

	var logs strings.Builder
	logf := func(format string, args ...any) {
		fmt.Fprintf(&logs, format+"\n", args...)
	}

	if _, err := detectCapabilities(context.Background(), backend, cfg, logf); err != nil {
		t.Fatalf("detectCapabilities error: %v", err)
	}

	out := logs.String()
	if !strings.Contains(out, "search_code") || !strings.Contains(out, "endpoint_disabled") {
		t.Fatalf("want search_code classified endpoint_disabled for api source, got:\n%s", out)
	}
	if strings.Contains(out, "classification=unauthorized") {
		t.Fatalf("search_code misclassified as unauthorized despite api source proving auth:\n%s", out)
	}
}
