// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func compactInputSchema(operationDescription string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": operationDescription,
			},
			"payload": map[string]any{
				"type":        "object",
				"description": "The selected operation's input object.",
			},
		},
		"required": []any{"operation"},
	}
}

func registerCompactTools(server *mcp.Server, coercer *scalarCoercer, service *Service, cfg config.Config) {
	if cfg.Capabilities.ListProjects {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_projects",
			Description: "List indexed OpenGrok projects. Results are paginated; pass next_cursor to retrieve subsequent pages.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
			output, err := service.ListProjects(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.SearchCode || cfg.Capabilities.SearchSymbolDefinitions || cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_search",
			Description: "Search OpenGrok code and symbols. operation=code searches text/path/history/definition/reference; operation=definitions finds symbol definitions; operation=references finds symbol references. Payload is the selected operation's input object.\n\nFor operation=code, the payload query field follows Lucene syntax. Wrap multi-word queries in quotes: \"extends PaymentProcessor\" returns ~7 exact results; unquoted `extends PaymentProcessor` returns 1000+ tokenised hits. Bare multi-word queries are auto-quoted by default; set payload.tokenized=true to search words as independent terms. Payload fields: query, mode, path_prefix, path_exclude, file_type, tokenized. Inline field syntax also works: -path:legacy, +path:domain, defs:ClassName, date:[20230101 TO 20261231] (history mode only; ignored elsewhere and flagged in the response warning).",
			InputSchema: compactInputSchema("one of: code, definitions, references"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.CompactSearch(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.ListSymbols || cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_symbols",
			Description: "Work with OpenGrok symbols. operation=list lists symbols (requires list_symbols capability); operation=implementations finds candidate implementations of a symbol; operation=cross_project_references finds references across projects. Each operation payload matches the corresponding full tool input.",
			InputSchema: compactInputSchema("one of: list, implementations, cross_project_references"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactSymbolsInput) (*mcp.CallToolResult, any, error) {
			output, err := service.CompactSymbols(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.GetFileContext {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_read",
			Description: "Read OpenGrok files or line windows. operation=file and operation=context both use a file-context payload.",
			InputSchema: compactInputSchema("one of: file, context"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactReadInput) (*mcp.CallToolResult, FileContextOutput, error) {
			output, err := service.CompactRead(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.GetFileContext &&
		(cfg.Capabilities.SearchCode || (cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences)) {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_compound",
			Description: "Compound OpenGrok operations. operation=search_and_read searches and reads file content around matches; operation=find_symbol_and_references finds a symbol's definition and references. Each operation payload matches the corresponding full tool input.\n\nFor operation=search_and_read, the query field follows the same Lucene syntax as opengrok_search: wrap multi-word queries in quotes (\"extends PaymentProcessor\"); bare multi-word queries are auto-quoted by default, set payload.tokenized=true to opt out. Use -path:legacy / +path:domain / defs:ClassName inline, or the path_exclude payload field. date: only works in history mode (ignored elsewhere, but flagged in the response warning).",
			InputSchema: compactInputSchema("one of: search_and_read, find_symbol_and_references"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactCompoundInput) (*mcp.CallToolResult, any, error) {
			output, err := service.CompactCompound(ctx, input)
			return nil, output, err
		})
	}

	if memoryToolsEnabled(cfg) {
		addTool(server, coercer, &mcp.Tool{
			Name:        "opengrok_memory",
			Description: "Interact with the server's process-scoped memory bank. Available only for stdio servers with memory enabled.",
			InputSchema: compactInputSchema("one of: set, get, list, delete, clear"),
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CompactMemoryInput) (*mcp.CallToolResult, any, error) {
			output, err := service.CompactMemory(ctx, input)
			return nil, output, err
		})
	}
}
