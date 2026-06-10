// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

type gatewayOperation struct {
	Manifest GatewayOperation
	Call     func(context.Context, json.RawMessage) (any, error)
}

func buildGatewayRegistry(service *Service, cfg config.Config) map[string]gatewayOperation {
	registry := make(map[string]gatewayOperation)

	if cfg.Capabilities.ListProjects {
		registry["projects.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "projects.list",
				Description: "List indexed OpenGrok projects.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ListProjectsInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.ListProjects(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchCode {
		registry["search.code"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.code",
				Description: "Search code in OpenGrok (full-text, path, history, definition, or reference).",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SearchCodeInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchCode(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolDefinitions {
		registry["search.definitions"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.definitions",
				Description: "Search symbol definitions in OpenGrok.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SymbolSearchInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchSymbolDefinitions(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolReferences {
		registry["search.references"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.references",
				Description: "Search symbol references in OpenGrok.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SymbolSearchInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchSymbolReferences(ctx, input)
			},
		}
	}

	if cfg.Capabilities.ListSymbols {
		registry["symbols.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "symbols.list",
				Description: "List symbol definitions in OpenGrok, optionally filtered by kind and path.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ListSymbolsInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.ListSymbols(ctx, input)
			},
		}
	}

	if cfg.Capabilities.ListFiles {
		registry["files.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "files.list",
				Description: "List files in an OpenGrok project directory.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ListFilesInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.ListFiles(ctx, input)
			},
		}

		registry["project.overview"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "project.overview",
				Description: "Get project overview with file and directory counts.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ProjectOverviewInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.GetProjectOverview(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolReferences {
		registry["search.implementations"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.implementations",
				Description: "Search candidate implementations and usages of a symbol.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input ImplementationSearchInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchImplementations(ctx, input)
			},
		}

		registry["search.cross_project_references"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "search.cross_project_references",
				Description: "Search symbol references across multiple projects.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input CrossProjectReferencesInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchCrossProjectReferences(ctx, input)
			},
		}
	}

	if cfg.Capabilities.GetFileContext {
		registry["file.read"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "file.read",
				Description: "Read full file content from OpenGrok.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input FileContextInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.GetFileContext(ctx, input)
			},
		}

		registry["file.context"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "file.context",
				Description: "Read a line window around a specific line number in a file.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input FileContextInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.GetFileContext(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext {
		registry["compound.search_and_read"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "compound.search_and_read",
				Description: "Search OpenGrok and read the file content around each match in a single call.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input SearchAndReadInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.SearchAndRead(ctx, input)
			},
		}
	}

	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext {
		registry["compound.find_symbol_and_references"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "compound.find_symbol_and_references",
				Description: "Find a symbol's definition and all its references in a single call.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input FindSymbolAndReferencesInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.FindSymbolAndReferences(ctx, input)
			},
		}
	}

	if memoryToolsEnabled(cfg) {
		registry["memory.set"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.set",
				Description: "Store a key-value pair in the server's memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input MemorySetInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.MemorySet(ctx, input)
			},
		}

		registry["memory.get"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.get",
				Description: "Retrieve a value from the memory bank by key.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input MemoryGetInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.MemoryGet(ctx, input)
			},
		}

		registry["memory.list"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.list",
				Description: "List all entries in the memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				return service.MemoryList(ctx, MemoryListInput{})
			},
		}

		registry["memory.delete"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.delete",
				Description: "Delete a key from the memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				var input MemoryDeleteInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, fmt.Errorf("decode payload: %w", err)
				}
				return service.MemoryDelete(ctx, input)
			},
		}

		registry["memory.clear"] = gatewayOperation{
			Manifest: GatewayOperation{
				Name:        "memory.clear",
				Description: "Clear all entries from the memory bank.",
			},
			Call: func(ctx context.Context, raw json.RawMessage) (any, error) {
				return service.MemoryClear(ctx, MemoryClearInput{})
			},
		}
	}

	return registry
}

func memoryToolsEnabled(cfg config.Config) bool {
	return cfg.Capabilities.Memory && cfg.Transport != config.TransportHTTP
}

func registerGatewayTools(server *mcp.Server, coercer *scalarCoercer, service *Service, cfg config.Config) {
	registry := buildGatewayRegistry(service, cfg)

	addTool(server, coercer, &mcp.Tool{
		Name:        "opengrok_discover",
		Description: "List available gateway operations for OpenGrok. Returns the full operation manifest with names and descriptions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GatewayDiscoverInput) (*mcp.CallToolResult, GatewayDiscoverOutput, error) {
		operations := make([]GatewayOperation, 0, len(registry))
		names := make([]string, 0, len(registry))
		for name := range registry {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			operations = append(operations, registry[name].Manifest)
		}
		return nil, GatewayDiscoverOutput{Operations: operations}, nil
	})

	addTool(server, coercer, &mcp.Tool{
		Name:        "opengrok_call",
		Description: "Call an OpenGrok gateway operation. Use opengrok_discover to list available operations and their payload schemas.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"operation": map[string]any{"type": "string"},
				"payload":   map[string]any{}, // any valid JSON
			},
			"required": []any{"operation"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GatewayCallInput) (*mcp.CallToolResult, GatewayCallOutput, error) {
		op, ok := registry[input.Operation]
		if !ok {
			enabledOps := make([]string, 0, len(registry))
			for name := range registry {
				enabledOps = append(enabledOps, name)
			}
			sort.Strings(enabledOps)
			return nil, GatewayCallOutput{}, &Error{
				Code:    codeUnknownOperation,
				Message: fmt.Sprintf("unknown operation %q; enabled operations: %v", input.Operation, enabledOps),
			}
		}
		result, err := op.Call(ctx, input.Payload)
		if err != nil {
			return nil, GatewayCallOutput{}, err
		}
		return nil, GatewayCallOutput{
			Operation: input.Operation,
			Result:    result,
		}, nil
	})
}
