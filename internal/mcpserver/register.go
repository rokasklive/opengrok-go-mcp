// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func NewMCPServer(cfg config.Config, backend Backend, version string) *mcp.Server {
	service := NewService(cfg, backend)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "opengrok-go-mcp",
		Version: version,
	}, nil)

	coercer := &scalarCoercer{}
	validator := &compactValidator{}

	switch cfg.ToolSurface {
	case config.ToolSurfaceCompact:
		registerCompactTools(server, coercer, validator, service, cfg)
		registerResources(server, service, cfg)
	case config.ToolSurfaceGateway:
		registerGatewayTools(server, coercer, service, cfg)
		registerResources(server, service, cfg)
	default:
		registerFullTools(server, coercer, service, cfg)
		registerResources(server, service, cfg)
	}

	// Coerce string-encoded booleans (e.g. include_links:"true") before the SDK
	// validates tool arguments, tolerating clients that serialize scalars as
	// strings.
	server.AddReceivingMiddleware(coercer.middleware(), validator.middleware())

	return server
}
