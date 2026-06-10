// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

// baseValidConfig returns a config that passes Validate() except for the field
// under test, with DefaultProject and Projects left empty (the discovery-deferred
// case introduced by the project-resolution ladder).
func baseValidConfig() Config {
	return Config{
		Transport:          TransportStdio,
		ToolSurface:        ToolSurfaceFull,
		OpenGrokAPIBaseURL: "https://grok.example.com/api/v1",
		PageSizeDefault:    10,
		PageSizeMax:        50,
	}
}

func TestValidateAcceptsAuthHeaderWhenDefaultProjectEmpty(t *testing.T) {
	cfg := baseValidConfig()
	cfg.OpenGrokAuthHeader = "Bearer api-token"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsBadPageSizeWhenDefaultProjectEmpty(t *testing.T) {
	cfg := baseValidConfig()
	cfg.PageSizeDefault = 100
	cfg.PageSizeMax = 10

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want page-size rejection (deferred default must not skip page-size check)")
	}
}

func TestValidateAllowsDeferredDefaultWhenConfigOtherwiseValid(t *testing.T) {
	cfg := baseValidConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil (empty default+projects is deferred to discovery)", err)
	}
}

func TestValidateAllowsMultipleProjectsWithoutDefaultAtParseTime(t *testing.T) {
	cfg := baseValidConfig()
	cfg.Projects = []string{"a", "b"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil (default project optional at parse time)", err)
	}
}
