// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/rokasklive/opengrok-go-mcp/internal/config"
)

func TestComposeDiscriminatedSchemaValidatesBranches(t *testing.T) {
	codeSchema, err := schemaForType[SearchCodeInput]()
	if err != nil {
		t.Fatalf("schemaForType SearchCodeInput: %v", err)
	}
	symbolSchema, err := schemaForType[SymbolSearchInput]()
	if err != nil {
		t.Fatalf("schemaForType SymbolSearchInput: %v", err)
	}

	schema, err := composeDiscriminatedSchema([]compactOperationSchema{
		{Name: "code", Schema: codeSchema},
		{Name: "definitions", Schema: symbolSchema},
	})
	if err != nil {
		t.Fatalf("composeDiscriminatedSchema: %v", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := resolved.Validate(map[string]any{"operation": "code", "query": "x"}); err != nil {
		t.Fatalf("valid code rejected: %v", err)
	}
	if err := resolved.Validate(map[string]any{"operation": "definitions", "symbol": "Foo"}); err != nil {
		t.Fatalf("valid definitions rejected: %v", err)
	}
	if err := resolved.Validate(map[string]any{"operation": "nope"}); err == nil {
		t.Fatal("unknown operation should be rejected")
	}
	if err := resolved.Validate(map[string]any{"operation": "code"}); err == nil {
		t.Fatal("missing required query should be rejected")
	}
}

func TestCompactSearchSchemaDeclaresRequiredFieldsPerBranch(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{SearchCode: true, GetFileContext: true}

	schema, err := compactSearchSchema(cfg)
	if err != nil {
		t.Fatalf("compactSearchSchema: %v", err)
	}

	codeBranch := findOneOfBranch(schema, "code")
	if codeBranch == nil {
		t.Fatal("code branch not found")
	}
	if !slices.Contains(codeBranch.Required, "query") {
		t.Fatalf("code branch required = %#v, want query", codeBranch.Required)
	}

	readBranch := findOneOfBranch(schema, "read")
	if readBranch == nil {
		t.Fatal("read branch not found")
	}
	if !slices.Contains(readBranch.Required, "query") {
		t.Fatalf("read branch required = %#v, want query", readBranch.Required)
	}
}

func TestCompactSymbolsSchemaDeclaresSymbolRequired(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{SearchSymbolDefinitions: true}

	schema, err := compactSymbolsSchema(cfg)
	if err != nil {
		t.Fatalf("compactSymbolsSchema: %v", err)
	}
	branch := findOneOfBranch(schema, "definitions")
	if branch == nil {
		t.Fatal("definitions branch not found")
	}
	if !slices.Contains(branch.Required, "symbol") {
		t.Fatalf("definitions required = %#v, want symbol", branch.Required)
	}
}

func TestCompactSchemaPreservesRequiredFieldDescriptions(t *testing.T) {
	schema, err := compactReadSchema()
	if err != nil {
		t.Fatalf("compactReadSchema: %v", err)
	}
	contextBranch := findOneOfBranch(schema, "context")
	if contextBranch == nil {
		t.Fatal("context branch not found")
	}
	lineProp := contextBranch.Properties["line_number"]
	if lineProp == nil || lineProp.Description == "" || !strings.HasPrefix(lineProp.Description, "REQUIRED") {
		t.Fatalf("line_number should keep REQUIRED description in slim schema: %#v", lineProp)
	}
	fileBranch := findOneOfBranch(schema, "file")
	if fileBranch == nil {
		t.Fatal("file branch not found")
	}
	pathProp := fileBranch.Properties["file_path"]
	if pathProp == nil || pathProp.Description == "" {
		t.Fatalf("file_path should keep REQUIRED description: %#v", pathProp)
	}
	// Optional fields should be stripped.
	if mode := fileBranch.Properties["before"]; mode != nil && mode.Description != "" {
		t.Fatalf("optional before should have no description in slim schema")
	}
}

func TestCompactSearchSchemaPreservesQueryDescription(t *testing.T) {
	cfg := testConfig()
	cfg.Capabilities = config.Capabilities{SearchCode: true}
	schema, err := compactSearchSchema(cfg)
	if err != nil {
		t.Fatalf("compactSearchSchema: %v", err)
	}
	branch := findOneOfBranch(schema, "code")
	if branch == nil {
		t.Fatal("code branch not found")
	}
	query := branch.Properties["query"]
	if query == nil || !strings.HasPrefix(query.Description, "REQUIRED") {
		t.Fatalf("query should keep REQUIRED description: %#v", query)
	}
	mode := branch.Properties["mode"]
	if mode != nil && mode.Description != "" {
		t.Fatalf("optional mode should have no description in slim schema")
	}
}

func TestCompactReadSchemaRequiresLineNumberForContext(t *testing.T) {
	schema, err := compactReadSchema()
	if err != nil {
		t.Fatalf("compactReadSchema: %v", err)
	}
	contextBranch := findOneOfBranch(schema, "context")
	if contextBranch == nil {
		t.Fatal("context branch not found")
	}
	if !slices.Contains(contextBranch.Required, "line_number") {
		t.Fatalf("context branch required = %#v, want line_number", contextBranch.Required)
	}
	fileBranch := findOneOfBranch(schema, "file")
	if fileBranch == nil {
		t.Fatal("file branch not found")
	}
	if slices.Contains(fileBranch.Required, "line_number") {
		t.Fatalf("file branch should not require line_number: %#v", fileBranch.Required)
	}
}

func TestCompactSchemaSlimmerThanVerboseForSameType(t *testing.T) {
	verbose, err := schemaForType[SymbolSearchInput]()
	if err != nil {
		t.Fatalf("verbose schema: %v", err)
	}
	slim, err := schemaForCompactType[SymbolSearchInput]()
	if err != nil {
		t.Fatalf("slim schema: %v", err)
	}
	verboseSize := schemaJSONSize(verbose)
	slimSize := schemaJSONSize(slim)
	if slimSize >= verboseSize {
		t.Fatalf("slim schema (%d bytes) should be smaller than verbose (%d bytes)", slimSize, verboseSize)
	}
	symbol := slim.Properties["symbol"]
	if symbol == nil || !strings.HasPrefix(symbol.Description, "REQUIRED") {
		t.Fatalf("symbol REQUIRED description should be preserved: %#v", symbol)
	}
	project := slim.Properties["project"]
	if project != nil && project.Description != "" {
		t.Fatalf("optional project description should be stripped")
	}
}

func schemaJSONSize(schema *jsonschema.Schema) int {
	data, err := json.Marshal(schema)
	if err != nil {
		return 0
	}
	return len(data)
}

func findOneOfBranch(schema *jsonschema.Schema, operation string) *jsonschema.Schema {
	for _, branch := range schema.OneOf {
		if branch == nil {
			continue
		}
		opProp := branch.Properties["operation"]
		if opProp == nil || opProp.Const == nil {
			continue
		}
		if *opProp.Const == operation {
			return branch
		}
	}
	return nil
}
