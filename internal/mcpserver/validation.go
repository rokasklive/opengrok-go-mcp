// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type compactValidator struct {
	tools map[string]compactToolValidation
}

type compactToolValidation struct {
	enabled    []string
	operations map[string]operationValidation
}

type operationValidation struct {
	required []string
	fields   map[string]fieldValidation
}

type fieldValidation struct {
	kind       reflect.Kind
	elemKind   reflect.Kind
	slice      bool
	acceptNull bool
	expected   string
}

func (v *compactValidator) registerOperations(toolName string, operations []string, inputTypes map[string]reflect.Type) {
	if len(operations) == 0 {
		return
	}
	if v.tools == nil {
		v.tools = make(map[string]compactToolValidation)
	}
	tool := compactToolValidation{
		enabled:    append([]string(nil), operations...),
		operations: make(map[string]operationValidation, len(operations)),
	}
	for _, operation := range operations {
		inputType := inputTypes[operation]
		tool.operations[operation] = validationForType(inputType)
	}
	v.tools[toolName] = tool
}

func validationForType(inputType reflect.Type) operationValidation {
	for inputType != nil && inputType.Kind() == reflect.Pointer {
		inputType = inputType.Elem()
	}
	out := operationValidation{fields: map[string]fieldValidation{}}
	if inputType == nil || inputType.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		out.fields[name] = fieldValidationForType(field.Type)
		if fieldRequired(field) {
			out.required = append(out.required, name)
		}
	}
	return out
}

func fieldRequired(field reflect.StructField) bool {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return true
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "-" {
		return false
	}
	for _, opt := range strings.Split(opts, ",") {
		if opt == "omitempty" {
			return false
		}
	}
	return true
}

func fieldValidationForType(fieldType reflect.Type) fieldValidation {
	acceptNull := false
	for fieldType.Kind() == reflect.Pointer {
		acceptNull = true
		fieldType = fieldType.Elem()
	}
	if fieldType.Kind() == reflect.Slice {
		elem := fieldType.Elem()
		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		return fieldValidation{
			kind:       fieldType.Kind(),
			elemKind:   elem.Kind(),
			slice:      true,
			acceptNull: acceptNull,
			expected:   expectedTypeName(fieldType.Kind(), elem.Kind()),
		}
	}
	return fieldValidation{
		kind:       fieldType.Kind(),
		acceptNull: acceptNull,
		expected:   expectedTypeName(fieldType.Kind(), reflect.Invalid),
	}
}

func expectedTypeName(kind reflect.Kind, elemKind reflect.Kind) string {
	if kind == reflect.Slice {
		return "array of " + expectedTypeName(elemKind, reflect.Invalid) + "s"
	}
	switch kind {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "JSON " + kind.String()
	}
}

func (v *compactValidator) middleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params != nil {
				if body, ok := v.validate(call.Params); ok {
					return structuredToolErrorBodyResult(body)
				}
			}
			return next(ctx, method, req)
		}
	}
}

func (v *compactValidator) validate(params *mcp.CallToolParamsRaw) (ToolErrorBody, bool) {
	tool, ok := v.tools[params.Name]
	if !ok {
		return ToolErrorBody{}, false
	}

	args := map[string]json.RawMessage{}
	if len(params.Arguments) > 0 {
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return ToolErrorBody{}, false
		}
	}

	operationRaw, ok := args["operation"]
	if !ok {
		return missingRequiredFieldBody("", "operation"), true
	}
	operation, ok := rawString(operationRaw)
	if !ok {
		return invalidFieldTypeBody("operation", "string", jsonTypeName(operationRaw)), true
	}
	op, ok := tool.operations[operation]
	if !ok {
		return unknownOperationBody(params.Name, operation, tool.enabled), true
	}

	for _, field := range op.required {
		if _, ok := args[field]; !ok {
			return missingRequiredFieldBody(operation, field), true
		}
	}

	unknown := ""
	for field, raw := range args {
		if field == "operation" {
			continue
		}
		validation, ok := op.fields[field]
		if !ok {
			if unknown == "" || field < unknown {
				unknown = field
			}
			continue
		}
		if !validation.valid(raw) {
			return invalidFieldTypeBody(field, validation.expected, jsonTypeName(raw)), true
		}
	}
	if unknown != "" {
		return unknownFieldBody(operation, unknown, op.fieldNames()), true
	}

	return ToolErrorBody{}, false
}

func (op operationValidation) fieldNames() []string {
	names := make([]string, 0, len(op.fields))
	for name := range op.fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func rawString(raw json.RawMessage) (string, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func (v fieldValidation) valid(raw json.RawMessage) bool {
	decoded, ok := decodeJSONValue(raw)
	if !ok {
		return true
	}
	if decoded == nil {
		return v.acceptNull
	}
	if v.slice {
		values, ok := decoded.([]any)
		if !ok {
			return false
		}
		for _, value := range values {
			if !kindMatches(value, v.elemKind) {
				return false
			}
		}
		return true
	}
	return kindMatches(decoded, v.kind)
}

func decodeJSONValue(raw json.RawMessage) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	return value, true
}

func kindMatches(value any, kind reflect.Kind) bool {
	switch kind {
	case reflect.String:
		_, ok := value.(string)
		return ok
	case reflect.Bool:
		_, ok := value.(bool)
		return ok
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := strconv.ParseInt(n.String(), 10, 64)
		return err == nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := strconv.ParseUint(n.String(), 10, 64)
		return err == nil
	case reflect.Float32, reflect.Float64:
		n, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := strconv.ParseFloat(n.String(), 64)
		return err == nil
	default:
		return true
	}
}

func jsonTypeName(raw json.RawMessage) string {
	value, ok := decodeJSONValue(raw)
	if !ok {
		return "invalid JSON"
	}
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func missingRequiredFieldBody(operation string, field string) ToolErrorBody {
	message := fmt.Sprintf("Missing required field %q.", field)
	suggestion := fmt.Sprintf("Provide %s.", field)
	if operation != "" {
		message = fmt.Sprintf("Operation %q is missing required field %q.", operation, field)
		suggestion = fmt.Sprintf("operation %s requires %s.", operation, field)
	}
	return ToolErrorBody{
		ErrorCode:  codeMissingRequiredField,
		Message:    message,
		Suggestion: suggestion,
		Details: map[string]any{
			"operation": operation,
			"field":     field,
		},
	}
}

func invalidFieldTypeBody(field string, expected string, got string) ToolErrorBody {
	return ToolErrorBody{
		ErrorCode:  codeInvalidFieldType,
		Message:    fmt.Sprintf("Invalid type for field %q: got %s, want %s.", field, got, expected),
		Suggestion: fmt.Sprintf("%s must be %s; got %s.", field, expected, got),
		Details: map[string]any{
			"field":    field,
			"expected": expected,
			"got":      got,
		},
	}
}

func unknownFieldBody(operation string, field string, knownFields []string) ToolErrorBody {
	return ToolErrorBody{
		ErrorCode: codeUnknownField,
		Message: fmt.Sprintf(
			"Unknown field %q for operation %q.",
			field,
			operation,
		),
		Suggestion: fmt.Sprintf(
			"%s is not a recognized parameter for %s; use one of: %s.",
			field,
			operation,
			strings.Join(knownFields, ", "),
		),
		Details: map[string]any{
			"operation":    operation,
			"field":        field,
			"known_fields": knownFields,
		},
	}
}

func unknownOperationBody(tool string, operation string, enabled []string) ToolErrorBody {
	return ToolErrorBody{
		ErrorCode: codeUnknownOperation,
		Message: fmt.Sprintf(
			"Operation %q is not valid for %s.",
			operation,
			tool,
		),
		Suggestion: fmt.Sprintf(
			"operation %q is not valid for %s; enabled operations: %s.",
			operation,
			tool,
			strings.Join(enabled, ", "),
		),
		Details: map[string]any{
			"tool":               tool,
			"operation":          operation,
			"enabled_operations": enabled,
		},
	}
}
