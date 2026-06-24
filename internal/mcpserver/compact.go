// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func (s *Service) CompactProjects(ctx context.Context, input compactCallInput) (any, error) {
	switch input.Operation {
	case "list":
		if !s.cfg.Capabilities.ListProjects {
			return nil, unknownOperationError(input.Operation, compactProjectsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[ListProjectsInput](input)
		if err != nil {
			return nil, err
		}
		return s.ListProjects(ctx, payload)
	case "files":
		if !s.cfg.Capabilities.ListFiles {
			return nil, unknownOperationError(input.Operation, compactProjectsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[ListFilesInput](input)
		if err != nil {
			return nil, err
		}
		return s.ListFiles(ctx, payload)
	case "overview":
		if !s.cfg.Capabilities.ListFiles {
			return nil, unknownOperationError(input.Operation, compactProjectsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[ProjectOverviewInput](input)
		if err != nil {
			return nil, err
		}
		return s.GetProjectOverview(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactProjectsOperations(s.cfg))
	}
}

func (s *Service) CompactSearch(ctx context.Context, input compactCallInput) (any, error) {
	switch input.Operation {
	case "code":
		if !s.cfg.Capabilities.SearchCode {
			return nil, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[SearchCodeInput](input)
		if err != nil {
			return nil, err
		}
		return s.SearchCode(ctx, payload)
	case "read":
		if !s.cfg.Capabilities.SearchCode || !s.cfg.Capabilities.GetFileContext {
			return nil, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[SearchAndReadInput](input)
		if err != nil {
			return nil, err
		}
		return s.SearchAndRead(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
	}
}

func (s *Service) CompactSymbols(ctx context.Context, input compactCallInput) (any, error) {
	switch input.Operation {
	case "definitions":
		if !s.cfg.Capabilities.SearchSymbolDefinitions {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[SymbolSearchInput](input)
		if err != nil {
			return nil, err
		}
		return s.SearchSymbolDefinitions(ctx, payload)
	case "references":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[SymbolSearchInput](input)
		if err != nil {
			return nil, err
		}
		return s.SearchSymbolReferences(ctx, payload)
	case "find":
		if !s.cfg.Capabilities.SearchSymbolDefinitions || !s.cfg.Capabilities.SearchSymbolReferences || !s.cfg.Capabilities.GetFileContext {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[FindSymbolAndReferencesInput](input)
		if err != nil {
			return nil, err
		}
		return s.FindSymbolAndReferences(ctx, payload)
	case "implementations":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[ImplementationSearchInput](input)
		if err != nil {
			return nil, err
		}
		return s.SearchImplementations(ctx, payload)
	case "cross_project":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[CrossProjectReferencesInput](input)
		if err != nil {
			return nil, err
		}
		return s.SearchCrossProjectReferences(ctx, payload)
	case "list":
		if !s.cfg.Capabilities.ListSymbols {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		payload, err := decodeCompactPayload[ListSymbolsInput](input)
		if err != nil {
			return nil, err
		}
		return s.ListSymbols(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
	}
}

func (s *Service) CompactRead(ctx context.Context, input compactCallInput) (FileContextOutput, error) {
	if input.Operation != "file" && input.Operation != "context" {
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}
	if !s.cfg.Capabilities.GetFileContext {
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}

	switch input.Operation {
	case "file":
		payload, err := decodeCompactPayload[ReadFileInput](input)
		if err != nil {
			return FileContextOutput{}, fmt.Errorf("decode compact read %s arguments: %w", input.Operation, err)
		}
		return s.GetFileContext(ctx, fileContextFromReadFile(payload))
	case "context":
		payload, err := decodeCompactPayload[ReadContextInput](input)
		if err != nil {
			return FileContextOutput{}, fmt.Errorf("decode compact read %s arguments: %w", input.Operation, err)
		}
		return s.GetFileContext(ctx, fileContextFromReadContext(payload))
	default:
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}
}

func fileContextFromReadFile(in ReadFileInput) FileContextInput {
	return FileContextInput{
		Project:            in.Project,
		FilePath:           in.FilePath,
		Cursor:             in.Cursor,
		IncludeAnnotations: in.IncludeAnnotations,
		IncludeLinks:       in.IncludeLinks,
		ContextBudget:      in.ContextBudget,
	}
}

func fileContextFromReadContext(in ReadContextInput) FileContextInput {
	return FileContextInput{
		Project:            in.Project,
		FilePath:           in.FilePath,
		LineNumber:         in.LineNumber,
		Before:             in.Before,
		After:              in.After,
		IncludeAnnotations: in.IncludeAnnotations,
		IncludeLinks:       in.IncludeLinks,
		ContextBudget:      in.ContextBudget,
	}
}

func compactProjectsOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.ListProjects {
		operations = append(operations, "list")
	}
	if cfg.Capabilities.ListFiles {
		operations = append(operations, "files", "overview")
	}
	return operations
}

func compactSearchOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.SearchCode {
		operations = append(operations, "code")
	}
	if cfg.Capabilities.SearchCode && cfg.Capabilities.GetFileContext {
		operations = append(operations, "read")
	}
	return operations
}

func compactSymbolsOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.SearchSymbolDefinitions {
		operations = append(operations, "definitions")
	}
	if cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "references", "implementations", "cross_project")
	}
	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences && cfg.Capabilities.GetFileContext {
		operations = append(operations, "find")
	}
	if cfg.Capabilities.ListSymbols {
		operations = append(operations, "list")
	}
	return operations
}

func compactReadOperations(cfg config.Config) []string {
	if !cfg.Capabilities.GetFileContext {
		return []string{}
	}
	return []string{"file", "context"}
}

func unknownOperationError(operation string, enabled []string) error {
	return &Error{
		Code:    codeUnknownOperation,
		Message: fmt.Sprintf("Unknown operation %q; enabled operations: %s.", operation, strings.Join(enabled, ", ")),
	}
}
