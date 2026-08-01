// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

// compactOperationSchema pairs an operation name with its typed input schema branch.
type compactOperationSchema struct {
	Name   string
	Schema *jsonschema.Schema
}

// composeDiscriminatedSchema builds a top-level schema discriminated by operation,
// with one oneOf branch per enabled operation. Each branch pins operation via const
// and merges the operation input type's properties and required fields.
func composeDiscriminatedSchema(operations []compactOperationSchema) (*jsonschema.Schema, error) {
	if len(operations) == 0 {
		return nil, fmt.Errorf("composeDiscriminatedSchema: no operations")
	}

	enum := make([]any, 0, len(operations))
	opNames := make([]string, 0, len(operations))
	oneOf := make([]*jsonschema.Schema, 0, len(operations))
	// properties holds every field the server can read for ANY operation,
	// declared at the top level so strict MCP clients (which validate and strip
	// arguments against root-level properties, ignoring oneOf branches) keep
	// per-operation fields like query/symbol/file_path instead of dropping them.
	// The oneOf branches below remain the precise per-operation validation.
	properties := map[string]*jsonschema.Schema{
		"operation": {
			Type: "string",
			// Enum/Description filled in after the loop once opNames is known.
		},
	}
	for _, op := range operations {
		if op.Name == "" {
			return nil, fmt.Errorf("composeDiscriminatedSchema: empty operation name")
		}
		if op.Schema == nil {
			return nil, fmt.Errorf("composeDiscriminatedSchema: nil schema for operation %q", op.Name)
		}
		enum = append(enum, op.Name)
		opNames = append(opNames, op.Name)
		branch, err := operationBranch(op.Name, op.Schema)
		if err != nil {
			return nil, err
		}
		oneOf = append(oneOf, branch)
		// Surface each branch's fields at the top level (optional, first wins).
		// Skip "operation" so the root keeps the enum discriminator below, not a
		// per-branch const.
		// Clone each prop: the SDK requires the published schema to form a tree,
		// so top-level properties may not share pointers with oneOf branches.
		for name, prop := range branch.Properties {
			if name == "operation" {
				continue
			}
			if _, exists := properties[name]; exists {
				continue
			}
			cloned, err := cloneSchema(prop)
			if err != nil {
				return nil, fmt.Errorf("clone property %q for operation %q: %w", name, op.Name, err)
			}
			properties[name] = cloned
		}
	}

	properties["operation"].Enum = enum
	properties["operation"].Description = "Discriminator — enabled operations: " + strings.Join(opNames, ", ")
	annotateOperationScope(properties, oneOf, opNames)

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"operation"},
		OneOf:      oneOf,
	}, nil
}

// annotateOperationScope marks every top-level property that only some
// operations accept. The top-level bag is the union of all operations' fields
// (so strict clients stop stripping them), but each oneOf branch enforces
// additionalProperties:false — so without this an agent reads a field from the
// root, sends it, and gets UNKNOWN_FIELD. Fields every operation accepts are
// left alone; annotating those would be noise on every schema.
func annotateOperationScope(
	properties map[string]*jsonschema.Schema,
	branches []*jsonschema.Schema,
	opNames []string,
) {
	owners := make(map[string][]string, len(properties))
	for i, branch := range branches {
		for name := range branch.Properties {
			if name == "operation" {
				continue
			}
			owners[name] = append(owners[name], opNames[i])
		}
	}

	for name, prop := range properties {
		if name == "operation" {
			continue
		}
		accepted := owners[name]
		if len(accepted) == 0 || len(accepted) == len(opNames) {
			continue
		}
		scope := "operation=" + strings.Join(accepted, " or ") + " only"
		if prop.Description == "" {
			prop.Description = scope
			continue
		}
		prop.Description = prop.Description + " (" + scope + ")"
	}
}

func operationBranch(operation string, inputSchema *jsonschema.Schema) (*jsonschema.Schema, error) {
	inputSchema, err := cloneSchema(inputSchema)
	if err != nil {
		return nil, fmt.Errorf("clone schema for operation %q: %w", operation, err)
	}
	opConst := any(operation)
	properties := map[string]*jsonschema.Schema{
		"operation": {Const: &opConst},
	}
	for name, prop := range inputSchema.Properties {
		properties[name] = prop
	}

	required := []string{"operation"}
	for _, field := range inputSchema.Required {
		if field == "operation" {
			continue
		}
		if !slices.Contains(required, field) {
			required = append(required, field)
		}
	}

	branch := &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
	if inputSchema.AdditionalProperties != nil {
		branch.AdditionalProperties = inputSchema.AdditionalProperties
	}
	return branch, nil
}

func cloneSchema(schema *jsonschema.Schema) (*jsonschema.Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("nil schema")
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var out jsonschema.Schema
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// schemaForType infers a JSON schema for a Go input type.
func schemaForType[T any]() (*jsonschema.Schema, error) {
	return jsonschema.For[T](nil)
}

func expandContextDescription(profile string) string {
	if config.IsEconomyProfile(profile) {
		return "Optional. Defaults to off under the economy profile. Set true to include extra lines of file context around each match."
	}
	return "Optional. Defaults to on under the rich profile. Set false to skip automatic file context expansion around each match."
}

func patchExpandContextDescription(schema *jsonschema.Schema, profile string) {
	if schema == nil {
		return
	}
	desc := expandContextDescription(profile)
	if prop := schema.Properties["expand_context"]; prop != nil {
		prop.Description = desc
	}
	for _, branch := range schema.OneOf {
		patchExpandContextDescription(branch, profile)
	}
}
