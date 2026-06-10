// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	errAuthTokenEmpty         = errors.New(`OPENGROK_MCP_API_TOKEN is empty after trimming whitespace`)
	errAuthTokenMissingScheme = errors.New(`OPENGROK_MCP_API_TOKEN must start with "Bearer " or "Basic "`)
	errAuthTokenUnknownScheme = errors.New(`OPENGROK_MCP_API_TOKEN scheme must be Bearer or Basic`)
	errAuthTokenMissingValue  = errors.New(`OPENGROK_MCP_API_TOKEN must include credentials after the scheme`)
	errLegacyBasicAuthEnv     = errors.New(`OPENGROK_MCP_BASIC_AUTH_TOKEN is no longer supported; set OPENGROK_MCP_API_TOKEN to "Basic <credentials>" instead`)
)

// ParseAuthToken normalizes OPENGROK_MCP_API_TOKEN into a full Authorization header value.
// The input must be "Bearer <token>" or "Basic <credentials>". Errors never include token text.
func ParseAuthToken(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errAuthTokenEmpty
	}

	idx := strings.IndexByte(raw, ' ')
	if idx < 0 {
		switch strings.ToLower(raw) {
		case "bearer", "basic":
			return "", errAuthTokenMissingValue
		default:
			return "", errAuthTokenMissingScheme
		}
	}

	scheme := strings.TrimSpace(raw[:idx])
	credential := strings.TrimSpace(raw[idx+1:])
	if credential == "" {
		return "", errAuthTokenMissingValue
	}

	switch strings.ToLower(scheme) {
	case "bearer":
		return "Bearer " + credential, nil
	case "basic":
		return "Basic " + credential, nil
	default:
		return "", errAuthTokenUnknownScheme
	}
}

func applyAuthFromEnv(cfg *Config) error {
	if _, ok := osLookupNonEmpty("OPENGROK_MCP_BASIC_AUTH_TOKEN"); ok {
		return errLegacyBasicAuthEnv
	}
	if value, ok := osLookupNonEmpty("OPENGROK_MCP_API_TOKEN"); ok {
		header, err := ParseAuthToken(value)
		if err != nil {
			return fmt.Errorf("parse auth token: %w", err)
		}
		cfg.OpenGrokAuthHeader = header
	}
	return nil
}

func osLookupNonEmpty(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}
