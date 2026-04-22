package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// Config contains runtime settings for the OpenGrok MCP server.
type Config struct {
	Transport              string
	Debug                  bool
	Listen                 string
	OpenGrokAPIBaseURL     string
	OpenGrokWebBaseURL     string
	OpenGrokAPIToken       string
	OpenGrokBasicAuthToken string
	DefaultProject         string
	ProjectRequired        bool
	PageSizeDefault        int
	PageSizeMax            int
	IncludeLinksDefault    bool
	EnableRawLinks         bool
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	LogLevel               string
}

// Default returns the baseline configuration.
func Default() Config {
	return Config{
		Transport:           TransportStdio,
		Listen:              "127.0.0.1:8765",
		ProjectRequired:     true,
		PageSizeDefault:     20,
		PageSizeMax:         100,
		IncludeLinksDefault: true,
		EnableRawLinks:      true,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		LogLevel:            "info",
	}
}

// FromEnv returns configuration with supported environment variable overrides.
func FromEnv() Config {
	cfg := Default()

	if value := os.Getenv("OPENGROK_MCP_LISTEN"); value != "" {
		cfg.Listen = value
	}
	if value := os.Getenv("OPENGROK_MCP_TRANSPORT"); value != "" {
		cfg.Transport = strings.ToLower(value)
	}
	if value := os.Getenv("OPENGROK_MCP_BASE_URL"); value != "" {
		cfg.OpenGrokAPIBaseURL = value
	}
	if value := os.Getenv("OPENGROK_MCP_WEB_BASE_URL"); value != "" {
		cfg.OpenGrokWebBaseURL = value
	}
	if value := os.Getenv("OPENGROK_MCP_API_TOKEN"); value != "" {
		cfg.OpenGrokAPIToken = value
	}
	if value := os.Getenv("OPENGROK_MCP_BASIC_AUTH_TOKEN"); value != "" {
		cfg.OpenGrokBasicAuthToken = value
	}
	if value := os.Getenv("OPENGROK_MCP_DEFAULT_PROJECT"); value != "" {
		cfg.DefaultProject = value
	}
	if value := os.Getenv("OPENGROK_MCP_LOG_LEVEL"); value != "" {
		cfg.LogLevel = value
	}
	if value := os.Getenv("OPENGROK_MCP_PROJECT_REQUIRED"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.ProjectRequired = parsed
		}
	}
	if value := os.Getenv("DEBUG"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Debug = parsed
		}
	}

	return cfg
}

// RegisterFlags binds command-line flags to the configuration.
func (c *Config) RegisterFlags(fs *flag.FlagSet) error {
	if fs == nil {
		return errors.New("flag set is nil")
	}

	fs.StringVar(&c.Listen, "listen", c.Listen, "address for the MCP server to listen on")
	fs.StringVar(&c.Transport, "transport", c.Transport, "MCP transport: stdio or http")
	fs.StringVar(&c.OpenGrokAPIBaseURL, "base-url", c.OpenGrokAPIBaseURL, "OpenGrok API base URL")
	fs.StringVar(&c.OpenGrokWebBaseURL, "web-base-url", c.OpenGrokWebBaseURL, "OpenGrok web base URL")
	fs.StringVar(&c.DefaultProject, "default-project", c.DefaultProject, "default OpenGrok project")
	fs.BoolVar(&c.ProjectRequired, "project-required", c.ProjectRequired, "require project parameter")
	fs.DurationVar(&c.ReadTimeout, "read-timeout", c.ReadTimeout, "server read timeout")
	fs.DurationVar(&c.WriteTimeout, "write-timeout", c.WriteTimeout, "server write timeout")
	fs.StringVar(&c.LogLevel, "log-level", c.LogLevel, "log level")

	return nil
}

// Validate checks whether the configuration is usable.
func (c *Config) Validate() error {
	c.Transport = strings.ToLower(c.Transport)
	switch c.Transport {
	case TransportStdio:
	case TransportHTTP:
		if c.Listen == "" {
			return errors.New("listen address is required")
		}
	default:
		return fmt.Errorf("unsupported transport %q", c.Transport)
	}
	if c.OpenGrokAPIBaseURL == "" {
		return errors.New("OpenGrok API base URL is required")
	}
	if c.OpenGrokWebBaseURL == "" {
		return errors.New("OpenGrok web base URL is required")
	}
	if c.OpenGrokAPIToken != "" && c.OpenGrokBasicAuthToken != "" {
		return errors.New("only one OpenGrok auth token may be configured")
	}
	if c.PageSizeDefault < 1 {
		return fmt.Errorf("page size default must be at least 1: %d", c.PageSizeDefault)
	}
	if c.PageSizeMax < c.PageSizeDefault {
		return fmt.Errorf(
			"page size max must be greater than or equal to default: max %d, default %d",
			c.PageSizeMax,
			c.PageSizeDefault,
		)
	}

	return nil
}
