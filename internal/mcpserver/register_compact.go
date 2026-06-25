// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func registerCompactTools(server *mcp.Server, coercer *scalarCoercer, validator *compactValidator, service *Service, cfg config.Config) {
	registerCompactProjects(server, coercer, validator, service, cfg)
	registerCompactSearch(server, coercer, validator, service, cfg)
	registerCompactSymbols(server, coercer, validator, service, cfg)
	registerCompactRead(server, coercer, validator, service, cfg)
}

func registerCompactProjects(server *mcp.Server, coercer *scalarCoercer, validator *compactValidator, service *Service, cfg config.Config) {
	ops := compactProjectsOperations(cfg)
	if len(ops) == 0 {
		return
	}

	schema, err := compactProjectsSchema(cfg)
	if err != nil {
		panic(fmt.Sprintf("compact projects schema: %v", err))
	}
	coercer.registerUnion("opengrok_projects",
		reflect.TypeFor[ListProjectsInput](),
		reflect.TypeFor[ListFilesInput](),
		reflect.TypeFor[ProjectOverviewInput](),
	)
	validator.registerOperations("opengrok_projects", ops, map[string]reflect.Type{
		"list":     reflect.TypeFor[ListProjectsInput](),
		"files":    reflect.TypeFor[ListFilesInput](),
		"overview": reflect.TypeFor[ProjectOverviewInput](),
	})
	addCompactTool(server, &mcp.Tool{
		Name:        "opengrok_projects",
		Description: compactProjectsDescription(cfg),
		InputSchema: schema,
		Annotations: readOnlyToolAnnotations,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input compactCallInput) (*mcp.CallToolResult, any, error) {
		output, err := service.CompactProjects(ctx, input)
		return nil, output, err
	})
}

func registerCompactSearch(server *mcp.Server, coercer *scalarCoercer, validator *compactValidator, service *Service, cfg config.Config) {
	ops := compactSearchOperations(cfg)
	if len(ops) == 0 {
		return
	}

	schema, err := compactSearchSchema(cfg)
	if err != nil {
		panic(fmt.Sprintf("compact search schema: %v", err))
	}
	coercer.registerUnion("opengrok_search",
		reflect.TypeFor[SearchCodeInput](),
		reflect.TypeFor[SearchAndReadInput](),
	)
	validator.registerOperations("opengrok_search", ops, map[string]reflect.Type{
		"code": reflect.TypeFor[SearchCodeInput](),
		"read": reflect.TypeFor[SearchAndReadInput](),
	})
	addCompactTool(server, &mcp.Tool{
		Name:        "opengrok_search",
		Description: compactSearchDescription(cfg),
		InputSchema: schema,
		Annotations: readOnlyToolAnnotations,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input compactCallInput) (*mcp.CallToolResult, any, error) {
		output, err := service.CompactSearch(ctx, input)
		return nil, output, err
	})
}

func registerCompactSymbols(server *mcp.Server, coercer *scalarCoercer, validator *compactValidator, service *Service, cfg config.Config) {
	ops := compactSymbolsOperations(cfg)
	if len(ops) == 0 {
		return
	}

	schema, err := compactSymbolsSchema(cfg)
	if err != nil {
		panic(fmt.Sprintf("compact symbols schema: %v", err))
	}
	coercer.registerUnion("opengrok_symbols",
		reflect.TypeFor[SymbolSearchInput](),
		reflect.TypeFor[FindSymbolAndReferencesInput](),
		reflect.TypeFor[ImplementationSearchInput](),
		reflect.TypeFor[CrossProjectReferencesInput](),
		reflect.TypeFor[ListSymbolsInput](),
	)
	validator.registerOperations("opengrok_symbols", ops, map[string]reflect.Type{
		"definitions":     reflect.TypeFor[SymbolSearchInput](),
		"references":      reflect.TypeFor[SymbolSearchInput](),
		"find":            reflect.TypeFor[FindSymbolAndReferencesInput](),
		"implementations": reflect.TypeFor[ImplementationSearchInput](),
		"cross_project":   reflect.TypeFor[CrossProjectReferencesInput](),
		"list":            reflect.TypeFor[ListSymbolsInput](),
	})
	addCompactTool(server, &mcp.Tool{
		Name:        "opengrok_symbols",
		Description: compactSymbolsDescription(cfg),
		InputSchema: schema,
		Annotations: readOnlyToolAnnotations,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input compactCallInput) (*mcp.CallToolResult, any, error) {
		output, err := service.CompactSymbols(ctx, input)
		return nil, output, err
	})
}

func registerCompactRead(server *mcp.Server, coercer *scalarCoercer, validator *compactValidator, service *Service, cfg config.Config) {
	ops := compactReadOperations(cfg)
	if len(ops) == 0 {
		return
	}

	schema, err := compactReadSchema()
	if err != nil {
		panic(fmt.Sprintf("compact read schema: %v", err))
	}
	coercer.registerUnion("opengrok_read",
		reflect.TypeFor[ReadFileInput](),
		reflect.TypeFor[ReadContextInput](),
	)
	validator.registerOperations("opengrok_read", ops, map[string]reflect.Type{
		"file":    reflect.TypeFor[ReadFileInput](),
		"context": reflect.TypeFor[ReadContextInput](),
	})
	addCompactTool(server, &mcp.Tool{
		Name:        "opengrok_read",
		Description: compactReadDescription(cfg),
		InputSchema: schema,
		Annotations: readOnlyToolAnnotations,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input compactCallInput) (*mcp.CallToolResult, FileContextOutput, error) {
		output, err := service.CompactRead(ctx, input)
		return nil, output, err
	})
}

func addCompactTool[Out any](server *mcp.Server, tool *mcp.Tool, handler func(context.Context, *mcp.CallToolRequest, compactCallInput) (*mcp.CallToolResult, Out, error)) {
	mcp.AddTool(server, tool, wrapToolHandler(handler))
}

func compactProjectsSchema(cfg config.Config) (*jsonschema.Schema, error) {
	ops := []compactOperationSchema{}
	if cfg.Capabilities.ListProjects {
		s, err := schemaForType[ListProjectsInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "list", Schema: s})
	}
	if cfg.Capabilities.ListFiles {
		filesSchema, err := schemaForType[ListFilesInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "files", Schema: filesSchema})
		overviewSchema, err := schemaForType[ProjectOverviewInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "overview", Schema: overviewSchema})
	}
	return composeDiscriminatedSchema(ops)
}

func compactSearchSchema(cfg config.Config) (*jsonschema.Schema, error) {
	ops := []compactOperationSchema{}
	if cfg.Capabilities.SearchCode {
		codeSchema, err := schemaForType[SearchCodeInput]()
		if err != nil {
			return nil, err
		}
		patchExpandContextDescription(codeSchema, cfg.AgentProfile)
		ops = append(ops, compactOperationSchema{Name: "code", Schema: codeSchema})
	}
	if cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext {
		readSchema, err := schemaForType[SearchAndReadInput]()
		if err != nil {
			return nil, err
		}
		patchExpandContextDescription(readSchema, cfg.AgentProfile)
		ops = append(ops, compactOperationSchema{Name: "read", Schema: readSchema})
	}
	return composeDiscriminatedSchema(ops)
}

func compactSymbolsSchema(cfg config.Config) (*jsonschema.Schema, error) {
	ops := []compactOperationSchema{}
	if cfg.Capabilities.SearchSymbolDefinitions {
		s, err := schemaForType[SymbolSearchInput]()
		if err != nil {
			return nil, err
		}
		patchExpandContextDescription(s, cfg.AgentProfile)
		ops = append(ops, compactOperationSchema{Name: "definitions", Schema: s})
	}
	if cfg.Capabilities.SearchSymbolReferences {
		refSchema, err := schemaForType[SymbolSearchInput]()
		if err != nil {
			return nil, err
		}
		patchExpandContextDescription(refSchema, cfg.AgentProfile)
		ops = append(ops, compactOperationSchema{Name: "references", Schema: refSchema})
		implSchema, err := schemaForType[ImplementationSearchInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "implementations", Schema: implSchema})
		crossSchema, err := schemaForType[CrossProjectReferencesInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "cross_project", Schema: crossSchema})
	}
	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext {
		findSchema, err := schemaForType[FindSymbolAndReferencesInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "find", Schema: findSchema})
	}
	if cfg.Capabilities.ListSymbols {
		listSchema, err := schemaForType[ListSymbolsInput]()
		if err != nil {
			return nil, err
		}
		ops = append(ops, compactOperationSchema{Name: "list", Schema: listSchema})
	}
	return composeDiscriminatedSchema(ops)
}

func compactReadSchema() (*jsonschema.Schema, error) {
	fileSchema, err := schemaForType[ReadFileInput]()
	if err != nil {
		return nil, err
	}
	contextSchema, err := schemaForType[ReadContextInput]()
	if err != nil {
		return nil, err
	}
	return composeDiscriminatedSchema([]compactOperationSchema{
		{Name: "file", Schema: fileSchema},
		{Name: "context", Schema: contextSchema},
	})
}
