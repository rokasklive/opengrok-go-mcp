// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
)

func TestParseAuthTokenAcceptsBearerAndBasic(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "Bearer with spaces trimmed",
			raw:  "  Bearer my-token  ",
			want: "Bearer my-token",
		},
		{
			name: "Basic credentials",
			raw:  "Basic dXNlcjpwYXNz",
			want: "Basic dXNlcjpwYXNz",
		},
		{
			name: "case insensitive scheme",
			raw:  "bearer token-value",
			want: "Bearer token-value",
		},
		{
			name: "basic lowercase",
			raw:  "basic abc123",
			want: "Basic abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAuthToken(tt.raw)
			if err != nil {
				t.Fatalf("ParseAuthToken() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ParseAuthToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAuthTokenRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty", raw: "   ", wantErr: "empty"},
		{name: "bare token", raw: "secret-token", wantErr: "Bearer"},
		{name: "unknown scheme", raw: "Digest abc", wantErr: "Bearer or Basic"},
		{name: "scheme only", raw: "Bearer", wantErr: "credentials"},
		{name: "basic scheme only", raw: "Basic", wantErr: "credentials"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAuthToken(tt.raw)
			if err == nil {
				t.Fatalf("ParseAuthToken() = %q, want error", got)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseAuthToken() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseAuthTokenRejectsEmptyCredentialAfterScheme(t *testing.T) {
	_, err := ParseAuthToken("Bearer ")
	if err == nil {
		t.Fatal("ParseAuthToken() error = nil, want error")
	}
	if err != errAuthTokenMissingValue {
		t.Fatalf("ParseAuthToken() error = %v, want missing credentials", err)
	}
}

func TestParseAuthTokenErrorsNeverEchoCredentials(t *testing.T) {
	const secret = "super-secret-credential-xyz"
	_, err := ParseAuthToken(secret)
	if err == nil {
		t.Fatal("ParseAuthToken() error = nil, want error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error %q must not echo credential input", err)
	}
}

func TestValidateRejectsLegacyBasicAuthEnv(t *testing.T) {
	t.Setenv("OPENGROK_MCP_BASIC_AUTH_TOKEN", "dXNlcjpwYXNz")
	t.Setenv("OPENGROK_MCP_API_TOKEN", "")

	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want legacy env rejection")
	}
	if !strings.Contains(err.Error(), "OPENGROK_MCP_BASIC_AUTH_TOKEN") {
		t.Fatalf("Validate() error = %v, want legacy env guidance", err)
	}
	if strings.Contains(err.Error(), "dXNlcjpwYXNz") {
		t.Fatalf("Validate() error must not echo token: %v", err)
	}
}

func TestValidateParsesAPIAuthTokenEnv(t *testing.T) {
	t.Setenv("OPENGROK_MCP_API_TOKEN", "Bearer env-token")
	t.Setenv("OPENGROK_MCP_BASIC_AUTH_TOKEN", "")

	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if cfg.OpenGrokAuthHeader != "Bearer env-token" {
		t.Fatalf("OpenGrokAuthHeader = %q, want Bearer env-token", cfg.OpenGrokAuthHeader)
	}
}

func TestValidateRejectsBareAPIAuthTokenEnv(t *testing.T) {
	t.Setenv("OPENGROK_MCP_API_TOKEN", "bare-token-without-scheme")

	cfg := Default()
	cfg.OpenGrokAPIBaseURL = "https://grok.example.com/source/api/v1"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want parse error")
	}
	if strings.Contains(err.Error(), "bare-token-without-scheme") {
		t.Fatalf("Validate() error must not echo token: %v", err)
	}
}
