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
	}

	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"operation": {
				Type:        "string",
				Enum:        enum,
				Description: "Discriminator — enabled operations: " + strings.Join(opNames, ", "),
			},
		},
		Required: []string{"operation"},
		OneOf:    oneOf,
	}, nil
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

// schemaForCompactType infers a schema and strips optional-field descriptions so compact
// tool descriptions carry the prose; REQUIRED field descriptions stay for schema-only discovery.
func schemaForCompactType[T any]() (*jsonschema.Schema, error) {
	schema, err := schemaForType[T]()
	if err != nil {
		return nil, err
	}
	return slimSchema(schema)
}

func slimSchema(schema *jsonschema.Schema) (*jsonschema.Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("nil schema")
	}
	out, err := cloneSchema(schema)
	if err != nil {
		return nil, err
	}
	slimSchemaInPlace(out)
	return out, nil
}

func slimSchemaInPlace(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	if schema.Description != "" && !strings.HasPrefix(schema.Description, "REQUIRED") {
		schema.Description = ""
	}
	schema.Title = ""
	for _, prop := range schema.Properties {
		slimSchemaInPlace(prop)
	}
	for _, branch := range schema.OneOf {
		slimSchemaInPlace(branch)
	}
	for _, branch := range schema.AnyOf {
		slimSchemaInPlace(branch)
	}
	slimSchemaInPlace(schema.Items)
	slimSchemaInPlace(schema.AdditionalProperties)
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
