// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *Service) MemorySet(ctx context.Context, input MemorySetInput) (MemorySetOutput, error) {
	s.memoryBank.Set(input.Key, input.Value)
	return MemorySetOutput{Success: true}, nil
}

func (s *Service) MemoryGet(ctx context.Context, input MemoryGetInput) (MemoryGetOutput, error) {
	value, found := s.memoryBank.Get(input.Key)
	return MemoryGetOutput{Value: value, Found: found}, nil
}

func (s *Service) MemoryList(ctx context.Context, input MemoryListInput) (MemoryListOutput, error) {
	return MemoryListOutput{Entries: s.memoryBank.List()}, nil
}

func (s *Service) MemoryDelete(ctx context.Context, input MemoryDeleteInput) (MemoryDeleteOutput, error) {
	_, found := s.memoryBank.Get(input.Key)
	if found {
		s.memoryBank.Delete(input.Key)
	}
	return MemoryDeleteOutput{Found: found, Deleted: found}, nil
}

func (s *Service) MemoryClear(ctx context.Context, input MemoryClearInput) (MemoryClearOutput, error) {
	s.memoryBank.Clear()
	return MemoryClearOutput{Success: true}, nil
}

func (s *Service) CompactMemory(ctx context.Context, input CompactMemoryInput) (any, error) {
	switch input.Operation {
	case "set":
		var payload MemorySetInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact memory set payload: %w", err)
		}
		return s.MemorySet(ctx, payload)
	case "get":
		var payload MemoryGetInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact memory get payload: %w", err)
		}
		return s.MemoryGet(ctx, payload)
	case "list":
		return s.MemoryList(ctx, MemoryListInput{})
	case "delete":
		var payload MemoryDeleteInput
		if err := json.Unmarshal(input.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode compact memory delete payload: %w", err)
		}
		return s.MemoryDelete(ctx, payload)
	case "clear":
		return s.MemoryClear(ctx, MemoryClearInput{})
	default:
		return nil, unknownOperationError(input.Operation, compactMemoryOperations())
	}
}

func compactMemoryOperations() []string {
	return []string{"set", "get", "list", "delete", "clear"}
}
