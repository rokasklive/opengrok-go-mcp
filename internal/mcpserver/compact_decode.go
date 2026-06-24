// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"encoding/json"
	"fmt"
)

// compactCallInput captures a flattened compact tool call: operation plus all
// operation-specific fields at the top level (no payload wrapper).
type compactCallInput struct {
	Operation string
	raw       json.RawMessage
}

func (c *compactCallInput) UnmarshalJSON(data []byte) error {
	var probe struct {
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	c.Operation = probe.Operation
	c.raw = append(json.RawMessage(nil), data...)
	return nil
}

func decodeCompactPayload[T any](input compactCallInput) (T, error) {
	var out T
	if len(input.raw) == 0 {
		return out, fmt.Errorf("missing compact call arguments")
	}
	if err := json.Unmarshal(input.raw, &out); err != nil {
		return out, fmt.Errorf("decode compact %q arguments: %w", input.Operation, err)
	}
	return out, nil
}
