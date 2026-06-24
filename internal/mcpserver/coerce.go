// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// scalarCoercer makes the server tolerant of MCP clients that serialize scalar
// tool arguments as JSON strings (e.g. include_links:"true" or page_size:"10"
// instead of true / 10). Scalar inputs are typed as bool/int/float (often
// behind a pointer), so the SDK infers a boolean/integer/number schema and the
// validator rejects a string before the handler runs. The coercer records each
// tool's scalar-typed fields at registration time, then a receiving middleware
// rewrites string-encoded scalars for exactly those fields before validation.
// Only fields the input type declares as a scalar are touched, so genuine
// string arguments (e.g. a query of "true" or "123") are left untouched.
type scalarCoercer struct {
	// fields maps tool name to a set of JSON field names mapped to the
	// underlying scalar kind the tool's input type declares for that field.
	fields map[string]map[string]reflect.Kind
}

// register reflects over a tool's input type and records the JSON names and
// underlying kinds of its scalar-typed (bool/int/uint/float) fields. inputType
// may be any reflect.Type; non-struct types (e.g. interfaces from handlers
// returning any) contribute no fields.
func (c *scalarCoercer) register(toolName string, inputType reflect.Type) {
	fields := scalarFieldsForType(inputType)
	if len(fields) == 0 {
		return
	}
	if c.fields == nil {
		c.fields = make(map[string]map[string]reflect.Kind)
	}
	c.fields[toolName] = fields
}

// registerUnion records the union of scalar fields across multiple input types
// for a compact tool whose flattened schema spans several operations.
func (c *scalarCoercer) registerUnion(toolName string, inputTypes ...reflect.Type) {
	merged := map[string]reflect.Kind{}
	for _, inputType := range inputTypes {
		for name, kind := range scalarFieldsForType(inputType) {
			merged[name] = kind
		}
	}
	if len(merged) == 0 {
		return
	}
	if c.fields == nil {
		c.fields = make(map[string]map[string]reflect.Kind)
	}
	c.fields[toolName] = merged
}

func scalarFieldsForType(inputType reflect.Type) map[string]reflect.Kind {
	for inputType != nil && inputType.Kind() == reflect.Pointer {
		inputType = inputType.Elem()
	}
	if inputType == nil || inputType.Kind() != reflect.Struct {
		return nil
	}

	var fields map[string]reflect.Kind
	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if !isCoercibleScalar(fieldType.Kind()) {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		if fields == nil {
			fields = make(map[string]reflect.Kind)
		}
		fields[name] = fieldType.Kind()
	}
	return fields
}

// middleware returns receiving middleware that coerces string-encoded scalars
// on incoming tool calls before the SDK validates arguments against the schema.
func (c *scalarCoercer) middleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params != nil {
				c.coerce(call.Params)
			}
			return next(ctx, method, req)
		}
	}
}

// coerce rewrites string-encoded scalars in params.Arguments for the scalar
// fields registered for params.Name. It is a no-op when the tool has no scalar
// fields, the arguments are absent, or the arguments are not a JSON object;
// malformed input is left for the validator to reject. Other fields are
// preserved byte-for-byte so non-scalar arguments are never reinterpreted.
func (c *scalarCoercer) coerce(params *mcp.CallToolParamsRaw) {
	fields := c.fields[params.Name]
	if len(fields) == 0 || len(params.Arguments) == 0 {
		return
	}

	var args map[string]json.RawMessage
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return
	}

	changed := false
	for name, kind := range fields {
		raw, ok := args[name]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue // not a JSON string; leave scalars/null as-is
		}
		encoded, ok := coerceScalarString(s, kind)
		if !ok {
			continue // unparseable for this kind; let the validator reject it
		}
		args[name] = encoded
		changed = true
	}
	if !changed {
		return
	}
	if rewritten, err := json.Marshal(args); err == nil {
		params.Arguments = rewritten
	}
}

// isCoercibleScalar reports whether a reflect.Kind is a scalar the coercer
// rewrites string forms of: booleans and the numeric kinds.
func isCoercibleScalar(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// coerceScalarString parses s as the given scalar kind and returns its JSON
// encoding. It reports ok=false when s is not a valid value for that kind, so
// the caller can leave the original string for the schema validator to reject.
func coerceScalarString(s string, kind reflect.Kind) (json.RawMessage, bool) {
	switch kind {
	case reflect.Bool:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return nil, false
		}
		return mustMarshal(v), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, false
		}
		return mustMarshal(v), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, false
		}
		return mustMarshal(v), true
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, false
		}
		return mustMarshal(v), true
	default:
		return nil, false
	}
}

// mustMarshal marshals a scalar value that is always JSON-encodable.
func mustMarshal(v any) json.RawMessage {
	encoded, _ := json.Marshal(v)
	return encoded
}

// jsonFieldName returns the JSON object key for a struct field, honoring the
// json struct tag (including its name and "-" sentinel) and falling back to the
// Go field name when no name is given.
func jsonFieldName(field reflect.StructField) string {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return field.Name
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return field.Name
	}
	return name
}

// addTool registers a tool and records its scalar-typed input fields with the
// coercer, so string-encoded scalars can be coerced before validation. It is a
// drop-in replacement for mcp.AddTool that keeps registration and coercion
// metadata in sync.
func addTool[In, Out any](server *mcp.Server, coercer *scalarCoercer, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	if coercer != nil {
		coercer.register(tool.Name, reflect.TypeFor[In]())
	}
	mcp.AddTool(server, tool, wrapToolHandler(handler))
}
