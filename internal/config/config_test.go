package config

import (
	"flag"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Listen != "127.0.0.1:8765" {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, "127.0.0.1:8765")
	}
	if !cfg.ProjectRequired {
		t.Fatal("ProjectRequired = false, want true")
	}
	if cfg.PageSizeDefault != 20 {
		t.Fatalf("PageSizeDefault = %d, want %d", cfg.PageSizeDefault, 20)
	}
	if cfg.PageSizeMax != 100 {
		t.Fatalf("PageSizeMax = %d, want %d", cfg.PageSizeMax, 100)
	}
	if !cfg.IncludeLinksDefault {
		t.Fatal("IncludeLinksDefault = false, want true")
	}
	if !cfg.EnableRawLinks {
		t.Fatal("EnableRawLinks = false, want true")
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %s, want %s", cfg.ReadTimeout, 10*time.Second)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %s, want %s", cfg.WriteTimeout, 10*time.Second)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestRegisterFlagsOverridesConfig(t *testing.T) {
	cfg := Default()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	if err := cfg.RegisterFlags(fs); err != nil {
		t.Fatalf("RegisterFlags() error = %v", err)
	}

	err := fs.Parse([]string{
		"--listen", "0.0.0.0:9000",
		"--base-url", "http://localhost:8080/api",
		"--web-base-url", "http://localhost:8080/source",
		"--default-project", "demo",
		"--project-required=false",
		"--read-timeout", "15s",
		"--write-timeout", "20s",
		"--log-level", "debug",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Listen != "0.0.0.0:9000" {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, "0.0.0.0:9000")
	}
	if cfg.OpenGrokAPIBaseURL != "http://localhost:8080/api" {
		t.Fatalf("OpenGrokAPIBaseURL = %q, want %q", cfg.OpenGrokAPIBaseURL, "http://localhost:8080/api")
	}
	if cfg.OpenGrokWebBaseURL != "http://localhost:8080/source" {
		t.Fatalf("OpenGrokWebBaseURL = %q, want %q", cfg.OpenGrokWebBaseURL, "http://localhost:8080/source")
	}
	if cfg.DefaultProject != "demo" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "demo")
	}
	if cfg.ProjectRequired {
		t.Fatal("ProjectRequired = true, want false")
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Fatalf("ReadTimeout = %s, want %s", cfg.ReadTimeout, 15*time.Second)
	}
	if cfg.WriteTimeout != 20*time.Second {
		t.Fatalf("WriteTimeout = %s, want %s", cfg.WriteTimeout, 20*time.Second)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestFromEnvAppliesSupportedEnvVars(t *testing.T) {
	t.Setenv("OPENGROK_MCP_LISTEN", "0.0.0.0:9000")
	t.Setenv("OPENGROK_MCP_BASE_URL", "http://localhost:8080/api")
	t.Setenv("OPENGROK_MCP_WEB_BASE_URL", "http://localhost:8080/source")
	t.Setenv("OPENGROK_MCP_DEFAULT_PROJECT", "demo")
	t.Setenv("OPENGROK_MCP_LOG_LEVEL", "debug")
	t.Setenv("OPENGROK_MCP_PROJECT_REQUIRED", "false")

	cfg := FromEnv()

	if cfg.Listen != "0.0.0.0:9000" {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, "0.0.0.0:9000")
	}
	if cfg.OpenGrokAPIBaseURL != "http://localhost:8080/api" {
		t.Fatalf("OpenGrokAPIBaseURL = %q, want %q", cfg.OpenGrokAPIBaseURL, "http://localhost:8080/api")
	}
	if cfg.OpenGrokWebBaseURL != "http://localhost:8080/source" {
		t.Fatalf("OpenGrokWebBaseURL = %q, want %q", cfg.OpenGrokWebBaseURL, "http://localhost:8080/source")
	}
	if cfg.DefaultProject != "demo" {
		t.Fatalf("DefaultProject = %q, want %q", cfg.DefaultProject, "demo")
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.ProjectRequired {
		t.Fatal("ProjectRequired = true, want false")
	}
}

func TestFromEnvIgnoresInvalidProjectRequired(t *testing.T) {
	t.Setenv("OPENGROK_MCP_PROJECT_REQUIRED", "definitely")

	cfg := FromEnv()

	if !cfg.ProjectRequired {
		t.Fatal("ProjectRequired = false, want true")
	}
}

func TestValidateRequiresBaseURLs(t *testing.T) {
	cfg := Default()

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}

	cfg.OpenGrokAPIBaseURL = "http://localhost:8080/api"
	cfg.OpenGrokWebBaseURL = "http://localhost:8080/source"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsInvalidConfig(t *testing.T) {
	valid := Default()
	valid.OpenGrokAPIBaseURL = "http://localhost:8080/api"
	valid.OpenGrokWebBaseURL = "http://localhost:8080/source"

	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "empty Listen",
			mutate: func(cfg *Config) {
				cfg.Listen = ""
			},
		},
		{
			name: "empty OpenGrokAPIBaseURL",
			mutate: func(cfg *Config) {
				cfg.OpenGrokAPIBaseURL = ""
			},
		},
		{
			name: "empty OpenGrokWebBaseURL",
			mutate: func(cfg *Config) {
				cfg.OpenGrokWebBaseURL = ""
			},
		},
		{
			name: "PageSizeDefault below minimum",
			mutate: func(cfg *Config) {
				cfg.PageSizeDefault = 0
			},
		},
		{
			name: "PageSizeMax below PageSizeDefault",
			mutate: func(cfg *Config) {
				cfg.PageSizeDefault = 20
				cfg.PageSizeMax = 19
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}
