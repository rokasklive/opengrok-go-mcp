// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"encoding/json"
	"fmt"
)

// inputSchemaForType infers a JSON schema for T and patches expand_context for the active profile.
func inputSchemaForType[T any](profile string) any {
	schema, err := schemaForType[T]()
	if err != nil {
		panic(fmt.Sprintf("inputSchemaForType: %v", err))
	}
	patchExpandContextDescription(schema, profile)
	data, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("marshal input schema: %v", err))
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		panic(fmt.Sprintf("unmarshal input schema: %v", err))
	}
	return out
}
