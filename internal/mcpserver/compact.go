// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func (s *Service) CompactCompound(ctx context.Context, input CompactCompoundInput) (any, error) {
	switch input.Operation {
	case "search_and_read":
		if !s.cfg.Capabilities.SearchCode || !s.cfg.Capabilities.GetFileContext {
			return nil, unknownOperationError(input.Operation, compactCompoundOperations(s.cfg))
		}
		var payload SearchAndReadInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact compound search_and_read payload: %w", err)
		}
		return s.SearchAndRead(ctx, payload)
	case "find_symbol_and_references":
		if !s.cfg.Capabilities.SearchSymbolDefinitions || !s.cfg.Capabilities.SearchSymbolReferences || !s.cfg.Capabilities.GetFileContext {
			return nil, unknownOperationError(input.Operation, compactCompoundOperations(s.cfg))
		}
		var payload FindSymbolAndReferencesInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact compound find_symbol_and_references payload: %w", err)
		}
		return s.FindSymbolAndReferences(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactCompoundOperations(s.cfg))
	}
}

func compactCompoundOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.SearchCode {
		operations = append(operations, "search_and_read")
	}
	if cfg.Capabilities.SearchSymbolDefinitions && cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "find_symbol_and_references")
	}
	return operations
}

func (s *Service) CompactSearch(ctx context.Context, input CompactSearchInput) (SearchOutput, error) {
	switch input.Operation {
	case "code":
		if !s.cfg.Capabilities.SearchCode {
			return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		var payload SearchCodeInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return SearchOutput{}, fmt.Errorf("decode compact search code payload: %w", err)
		}
		return s.SearchCode(ctx, payload)
	case "definitions":
		if !s.cfg.Capabilities.SearchSymbolDefinitions {
			return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		var payload SymbolSearchInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return SearchOutput{}, fmt.Errorf("decode compact search definitions payload: %w", err)
		}
		return s.SearchSymbolDefinitions(ctx, payload)
	case "references":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
		}
		var payload SymbolSearchInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return SearchOutput{}, fmt.Errorf("decode compact search references payload: %w", err)
		}
		return s.SearchSymbolReferences(ctx, payload)
	default:
		return SearchOutput{}, unknownOperationError(input.Operation, compactSearchOperations(s.cfg))
	}
}

func (s *Service) CompactSymbols(ctx context.Context, input CompactSymbolsInput) (any, error) {
	switch input.Operation {
	case "list":
		if !s.cfg.Capabilities.ListSymbols {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		var payload ListSymbolsInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact symbols list payload: %w", err)
		}
		return s.ListSymbols(ctx, payload)
	case "implementations":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		var payload ImplementationSearchInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact symbols implementations payload: %w", err)
		}
		return s.SearchImplementations(ctx, payload)
	case "cross_project_references":
		if !s.cfg.Capabilities.SearchSymbolReferences {
			return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
		}
		var payload CrossProjectReferencesInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact symbols cross_project_references payload: %w", err)
		}
		return s.SearchCrossProjectReferences(ctx, payload)
	default:
		return nil, unknownOperationError(input.Operation, compactSymbolsOperations(s.cfg))
	}
}

func (s *Service) CompactRead(ctx context.Context, input CompactReadInput) (FileContextOutput, error) {
	if input.Operation != "file" && input.Operation != "context" {
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}
	if !s.cfg.Capabilities.GetFileContext {
		return FileContextOutput{}, unknownOperationError(input.Operation, compactReadOperations(s.cfg))
	}

	var payload FileContextInput
	if err := json.Unmarshal(input.Payload, &payload); err != nil {
		return FileContextOutput{}, fmt.Errorf("decode compact read %s payload: %w", input.Operation, err)
	}
	return s.GetFileContext(ctx, payload)
}

func compactSearchOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.SearchCode {
		operations = append(operations, "code")
	}
	if cfg.Capabilities.SearchSymbolDefinitions {
		operations = append(operations, "definitions")
	}
	if cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "references")
	}
	return operations
}

func compactSymbolsOperations(cfg config.Config) []string {
	operations := []string{}
	if cfg.Capabilities.ListSymbols {
		operations = append(operations, "list")
	}
	if cfg.Capabilities.SearchSymbolReferences {
		operations = append(operations, "implementations")
		operations = append(operations, "cross_project_references")
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
