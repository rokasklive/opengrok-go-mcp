// SPDX-License-Identifier: Apache-2.0

package mcpserver

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	registerClaimCheck("scalar-coercion", "TestScalarCoercionStringEncodedBeforeValidation")
}

func TestScalarCoercerCoerce(t *testing.T) {
	coercer := &scalarCoercer{}
	coercer.register("search_code", reflect.TypeFor[SearchCodeInput]())

	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "string true on boolean field is coerced",
			in:   `{"include_links":"true"}`,
			want: `{"include_links":true}`,
		},
		{
			name: "string false on boolean field is coerced",
			in:   `{"include_snippets":"false"}`,
			want: `{"include_snippets":false}`,
		},
		{
			name: "real boolean on boolean field is preserved",
			in:   `{"include_links":true}`,
			want: `{"include_links":true}`,
		},
		{
			name: "unparseable string on boolean field is left for the validator",
			in:   `{"include_links":"maybe"}`,
			want: `{"include_links":"maybe"}`,
		},
		{
			name: "string integer on integer field is coerced",
			in:   `{"page_size":"10"}`,
			want: `{"page_size":10}`,
		},
		{
			name: "real integer on integer field is preserved",
			in:   `{"page_size":10}`,
			want: `{"page_size":10}`,
		},
		{
			name: "fractional string on integer field is left for the validator",
			in:   `{"page_size":"10.5"}`,
			want: `{"page_size":"10.5"}`,
		},
		{
			name: "string value on a non-numeric field is never coerced",
			in:   `{"query":"123"}`,
			want: `{"query":"123"}`,
		},
		{
			name: "string value on a non-boolean field is never coerced",
			in:   `{"query":"true"}`,
			want: `{"query":"true"}`,
		},
		{
			name: "explicit null on a scalar field is preserved",
			in:   `{"include_links":null}`,
			want: `{"include_links":null}`,
		},
		{
			name: "other fields are preserved alongside a coercion",
			in:   `{"query":"Engine","mode":"full_text","include_links":"false","page_size":"25"}`,
			want: `{"query":"Engine","mode":"full_text","include_links":false,"page_size":25}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := &mcp.CallToolParamsRaw{Name: "search_code", Arguments: json.RawMessage(tc.in)}
			coercer.coerce(params)
			if !equalJSON(t, params.Arguments, tc.want) {
				t.Fatalf("coerce(%s) = %s, want equivalent to %s", tc.in, params.Arguments, tc.want)
			}
		})
	}
}

func TestScalarCoercerCoercesFloatField(t *testing.T) {
	type floatInput struct {
		Ratio float64 `json:"ratio"`
	}
	coercer := &scalarCoercer{}
	coercer.register("ratio_tool", reflect.TypeFor[floatInput]())

	params := &mcp.CallToolParamsRaw{Name: "ratio_tool", Arguments: json.RawMessage(`{"ratio":"0.5"}`)}
	coercer.coerce(params)
	if !equalJSON(t, params.Arguments, `{"ratio":0.5}`) {
		t.Fatalf("coerce did not coerce float field: %s", params.Arguments)
	}
}

func TestScalarCoercerIgnoresUnregisteredTool(t *testing.T) {
	coercer := &scalarCoercer{}
	coercer.register("search_code", reflect.TypeFor[SearchCodeInput]())

	params := &mcp.CallToolParamsRaw{Name: "memory_set", Arguments: json.RawMessage(`{"include_links":"true"}`)}
	coercer.coerce(params)
	if !equalJSON(t, params.Arguments, `{"include_links":"true"}`) {
		t.Fatalf("coerce mutated arguments for an unregistered tool: %s", params.Arguments)
	}
}

func TestScalarCoercionStringEncodedBeforeValidation(t *testing.T) {
	clientSession, backend := compactTestServer(t, allCapabilities())
	backend.fileContent = "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\neleven\ntwelve\n"

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "opengrok_read",
		Arguments: map[string]any{
			"operation":   "context",
			"file_path":   "src/Engine.go",
			"line_number": 12,
			"before":      "10",
			"after":       "0",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(opengrok_read context) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(opengrok_read context) IsError = true, text = %s", toolResultText(result))
	}

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var output FileContextOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatalf("unmarshal FileContextOutput: %v", err)
	}
	if output.StartLine != 2 {
		t.Fatalf("StartLine = %d, want 2 from coerced before=10", output.StartLine)
	}
}

// equalJSON reports whether got and want are equal JSON values, ignoring object
// key order and formatting.
func equalJSON(t *testing.T, got json.RawMessage, want string) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("unmarshal got %s: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("unmarshal want %s: %v", want, err)
	}
	return reflect.DeepEqual(g, w)
}
