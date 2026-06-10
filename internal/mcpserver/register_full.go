// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func registerFullTools(server *mcp.Server, coercer *scalarCoercer, service *Service, cfg config.Config) {
	readOnlyAnnotations := &mcp.ToolAnnotations{ReadOnlyHint: true}

	if cfg.Capabilities.ListProjects {
		addTool(server, coercer, &mcp.Tool{
			Name:        "list_projects",
			Description: "List indexed OpenGrok projects. Results are paginated (50 per page); pass next_cursor to retrieve subsequent pages. total_projects is always returned so agents know the full count.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectsInput) (*mcp.CallToolResult, ListProjectsOutput, error) {
			output, err := service.ListProjects(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchCode {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_code",
			Description: "Search reference/base code in OpenGrok (Apache Lucene backend). Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. Use mode full_text, path, history, definition, or reference; for file-name searches use mode=path.\n\nQUERY SYNTAX — wrap multi-word queries in quotes for exact-phrase matching. Unquoted `extends PaymentProcessor` tokenises into independent terms and returns 1000+ noisy hits; quoted \"extends PaymentProcessor\" returns ~7 exact hits. This server AUTO-QUOTES bare multi-word queries by default and notes it in the response warning; pass tokenized:true to search the words as independent terms instead. Use path_exclude to drop matches under a path (e.g. path_exclude=test) and path_prefix to restrict to a path.\n\nInline Lucene syntax also works in the query string: -path:legacy (exclude), +path:domain (require), defs:ClassName (symbol definition), refs:ClassName (symbol reference), hist:bugfix (commit messages, history mode), date:[20230101 TO 20261231] (history mode only). GOTCHAS: date: only works in history mode — used elsewhere it is ignored, but the response warning flags it. Wildcards (* ?) cannot be used inside quoted phrases (this silently matches nothing).\n\nUse returned file_path/project with read_file instead of fetching display_url/raw_url yourself. When answering about a specific file or class, include the selected result's citation.url.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchCodeInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchCode(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_and_read",
			Description: "Search OpenGrok and read the file content around each match in a single call, reducing round trips. Uses the same query interface as search_code.\n\nQUERY SYNTAX — wrap multi-word queries in quotes (\"extends PaymentProcessor\", not `extends PaymentProcessor`); bare multi-word queries are auto-quoted by default, pass tokenized:true to opt out. Inline Lucene syntax works: -path:legacy, +path:domain, defs:ClassName. Use path_exclude to drop matches under a path. date: only works in history mode (ignored elsewhere, but flagged in the response warning); wildcards cannot be used inside quoted phrases.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SearchAndReadInput) (*mcp.CallToolResult, SearchAndReadOutput, error) {
			output, err := service.SearchAndRead(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchSymbolDefinitions {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_symbol_definitions",
			Description: "Search symbol definitions in reference/base OpenGrok code. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. Use returned file_path/project with read_file to read the matched file; do not use WebFetch for display_url/raw_url because browser URLs may require auth. When answering about a class/interface, include citation.url for the definition.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SymbolSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchSymbolDefinitions(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_symbol_references",
			Description: "Search symbol references in reference/base OpenGrok code. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. Use returned file_path/project with read_file to read the matched file; avoid calling this for broad symbols unless you need many references. If discussing a specific reference, include citation.url.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input SymbolSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchSymbolReferences(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext {
		addTool(server, coercer, &mcp.Tool{
			Name:        "find_symbol_and_references",
			Description: "Find a symbol's definition and all its references in a single call. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Returns the definition with surrounding context plus a paginated reference list.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input FindSymbolAndReferencesInput) (*mcp.CallToolResult, FindSymbolAndReferencesOutput, error) {
			output, err := service.FindSymbolAndReferences(ctx, input)
			return nil, output, err
		})
	}
	if cfg.Capabilities.GetFileContext {
		readFile := func(ctx context.Context, req *mcp.CallToolRequest, input FileContextInput) (*mcp.CallToolResult, FileContextOutput, error) {
			output, err := service.GetFileContext(ctx, input)
			return nil, output, err
		}
		addTool(server, coercer, &mcp.Tool{
			Name:        "get_file_context",
			Description: "Read a line window around a specific line in an OpenGrok file. Pass line_number (e.g. from a search result) to center the window and before/after to size it; if you omit line_number the whole file is returned, but prefer read_file for that. Omit project unless the user explicitly names an OpenGrok project; do not infer project from the local repository name. When answering the user about this file, include citation.url.",
			Annotations: readOnlyAnnotations,
		}, readFile)
		addTool(server, coercer, &mcp.Tool{
			Name:        "read_file",
			Description: "Read full file content from OpenGrok. Returns up to 500 lines per call; if truncated is true, pass next_cursor to read the next section. total_lines is always returned. Use project and file_path from search results; omit project otherwise unless the user explicitly names one. Do not use WebFetch on display_url/raw_url; this tool sends configured auth and falls back to /raw. For a targeted line window use get_file_context with line_number. When summarizing a class or file, include citation.url in the final answer.",
			Annotations: readOnlyAnnotations,
		}, readFile)
	}

	if cfg.Capabilities.ListSymbols {
		addTool(server, coercer, &mcp.Tool{
			Name:        "list_symbols",
			Description: "List symbol definitions in OpenGrok, optionally filtered by ctags kind (class, interface, function, method, etc.) and scoped to a path. Use this for structural, architect-oriented queries: \"what classes exist in this package?\", \"find all interfaces under src/api/\". Combine path_prefix and kind for precise structural inventory. For broad sweeps across a large codebase, set include_snippets=false to reduce token cost — the warning field will tell you if the result set is large and how many additional calls full enumeration would require. Results are lean — use read_file or get_file_context to drill into a specific symbol. Omit project unless the user explicitly names one.",
			Annotations: readOnlyAnnotations,
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListSymbolsInput) (*mcp.CallToolResult, ListSymbolsOutput, error) {
			output, err := service.ListSymbols(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.ListFiles {
		addTool(server, coercer, &mcp.Tool{
			Name:        "list_files",
			Description: "List files in an OpenGrok project directory. Results are paginated; use page_size to control page size and next_cursor for subsequent pages.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ListFilesInput) (*mcp.CallToolResult, ListFilesOutput, error) {
			output, err := service.ListFiles(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.ListFiles {
		addTool(server, coercer, &mcp.Tool{
			Name:        "get_project_overview",
			Description: "Get a high-level overview of an OpenGrok project: total file/directory counts, a per-language breakdown (files, lines, and percent per language), and top-level directory and file entries. Use this to answer questions like \"what languages does this project use?\".",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ProjectOverviewInput) (*mcp.CallToolResult, ProjectOverviewOutput, error) {
			output, err := service.GetProjectOverview(ctx, input)
			return nil, output, err
		})
	}

	if cfg.Capabilities.SearchSymbolReferences {
		addTool(server, coercer, &mcp.Tool{
			Name:        "search_implementations",
			Description: "Search candidate implementations and usages of a symbol. Pass a bare symbol name (e.g. PaymentProcessor), not quoted. Delegates to symbol-reference search; results are best-effort since OpenGrok does not provide language-semantic implementation mapping.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input ImplementationSearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			output, err := service.SearchImplementations(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "search_cross_project_references",
			Description: "Search for references to a symbol across multiple projects, grouped by project for cross-project analysis. Pass a bare symbol name (e.g. PaymentProcessor), not quoted.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input CrossProjectReferencesInput) (*mcp.CallToolResult, CrossProjectReferencesOutput, error) {
			output, err := service.SearchCrossProjectReferences(ctx, input)
			return nil, output, err
		})
	}

	if memoryToolsEnabled(cfg) {
		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_set",
			Description: "Store a key-value pair in the server's memory bank. Values persist for the lifetime of the server process.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemorySetInput) (*mcp.CallToolResult, MemorySetOutput, error) {
			output, err := service.MemorySet(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_get",
			Description: "Retrieve a value from the memory bank by key.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryGetInput) (*mcp.CallToolResult, MemoryGetOutput, error) {
			output, err := service.MemoryGet(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_list",
			Description: "List all entries in the memory bank.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryListInput) (*mcp.CallToolResult, MemoryListOutput, error) {
			output, err := service.MemoryList(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_delete",
			Description: "Delete a key from the memory bank.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryDeleteInput) (*mcp.CallToolResult, MemoryDeleteOutput, error) {
			output, err := service.MemoryDelete(ctx, input)
			return nil, output, err
		})

		addTool(server, coercer, &mcp.Tool{
			Name:        "memory_clear",
			Description: "Clear all entries from the memory bank.",
		}, func(ctx context.Context, req *mcp.CallToolRequest, input MemoryClearInput) (*mcp.CallToolResult, MemoryClearOutput, error) {
			output, err := service.MemoryClear(ctx, input)
			return nil, output, err
		})
	}
}
