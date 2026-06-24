// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"fmt"
	"strings"
)

const (
	surfaceFull    = "full"
	surfaceCompact = "compact"
	surfaceGateway = "gateway"
)

var allSurfaces = []string{surfaceFull, surfaceCompact, surfaceGateway}

var knownCanonicalOps = map[string]bool{
	"search.definitions":   true,
	"search.references":    true,
	"search.code":          true,
	"path.search":          true,
	"read.file":            true,
	"files.list":           true,
	"compound.find_symbol": true,
	"projects.overview":    true,
}

// ResolveResult maps a canonical op to an MCP tool call for a surface.
type ResolveResult struct {
	Tool       string
	Arguments  map[string]any
	Skipped    bool
	SkipReason string
}

func isKnownOp(op string) bool {
	return knownCanonicalOps[op]
}

// Resolve maps canonical operation + args to a surface-specific tool call.
func Resolve(surface, op string, args map[string]any) (ResolveResult, error) {
	if !isKnownOp(op) {
		return ResolveResult{}, fmt.Errorf("unknown canonical op %q", op)
	}
	surface = strings.ToLower(strings.TrimSpace(surface))
	switch surface {
	case surfaceFull:
		return resolveFull(op, cloneArgs(args))
	case surfaceCompact:
		return resolveCompact(op, cloneArgs(args))
	case surfaceGateway:
		return resolveGateway(op, cloneArgs(args))
	default:
		return ResolveResult{}, fmt.Errorf("unknown surface %q", surface)
	}
}

func cloneArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		out[k] = v
	}
	return out
}

func flattenCompactCall(operation string, args map[string]any) map[string]any {
	out := map[string]any{"operation": operation}
	for k, v := range args {
		out[k] = v
	}
	return out
}

func resolveFull(op string, args map[string]any) (ResolveResult, error) {
	switch op {
	case "search.definitions":
		return ResolveResult{Tool: "search_symbol_definitions", Arguments: args}, nil
	case "search.references":
		return ResolveResult{Tool: "search_symbol_references", Arguments: args}, nil
	case "search.code":
		return ResolveResult{Tool: "search_code", Arguments: args}, nil
	case "path.search":
		args["mode"] = "path"
		return ResolveResult{Tool: "search_code", Arguments: args}, nil
	case "read.file":
		return ResolveResult{Tool: "read_file", Arguments: args}, nil
	case "files.list":
		return ResolveResult{Tool: "list_files", Arguments: args}, nil
	case "compound.find_symbol":
		return ResolveResult{Tool: "find_symbol_and_references", Arguments: args}, nil
	case "projects.overview":
		return ResolveResult{Tool: "get_project_overview", Arguments: args}, nil
	default:
		return ResolveResult{}, fmt.Errorf("unmapped op %q for full", op)
	}
}

func resolveCompact(op string, args map[string]any) (ResolveResult, error) {
	switch op {
	case "search.definitions":
		return ResolveResult{
			Tool:      "opengrok_symbols",
			Arguments: flattenCompactCall("definitions", args),
		}, nil
	case "search.references":
		return ResolveResult{
			Tool:      "opengrok_symbols",
			Arguments: flattenCompactCall("references", args),
		}, nil
	case "search.code":
		return ResolveResult{
			Tool:      "opengrok_search",
			Arguments: flattenCompactCall("code", args),
		}, nil
	case "path.search":
		args["mode"] = "path"
		return ResolveResult{
			Tool:      "opengrok_search",
			Arguments: flattenCompactCall("code", args),
		}, nil
	case "read.file":
		return ResolveResult{
			Tool:      "opengrok_read",
			Arguments: flattenCompactCall("file", args),
		}, nil
	case "files.list":
		return ResolveResult{
			Tool:      "opengrok_projects",
			Arguments: flattenCompactCall("files", args),
		}, nil
	case "compound.find_symbol":
		return ResolveResult{
			Tool:      "opengrok_symbols",
			Arguments: flattenCompactCall("find", args),
		}, nil
	case "projects.overview":
		return ResolveResult{
			Tool:      "opengrok_projects",
			Arguments: flattenCompactCall("overview", args),
		}, nil
	default:
		return ResolveResult{}, fmt.Errorf("unmapped op %q for compact", op)
	}
}

func resolveGateway(op string, args map[string]any) (ResolveResult, error) {
	if op == "path.search" {
		args["mode"] = "path"
	}
	gwOp := gatewayOperationName(op)
	if gwOp == "" {
		return ResolveResult{}, fmt.Errorf("unmapped op %q for gateway", op)
	}
	return ResolveResult{
		Tool: "opengrok_call",
		Arguments: map[string]any{
			"operation": gwOp,
			"payload":   args,
		},
	}, nil
}

func gatewayOperationName(op string) string {
	switch op {
	case "search.definitions":
		return "search.definitions"
	case "search.references":
		return "search.references"
	case "search.code":
		return "search.code"
	case "path.search":
		return "search.code"
	case "read.file":
		return "file.read"
	case "files.list":
		return "files.list"
	case "compound.find_symbol":
		return "compound.find_symbol_and_references"
	case "projects.overview":
		return "projects.overview"
	default:
		return ""
	}
}

// adaptEvalCaseForCompact maps a full-surface direct eval case onto compact.
func adaptEvalCaseForCompact(tc EvalCase) (EvalCase, bool) {
	tool, operation, ok := fullToolToCompactOperation(tc.Tool)
	if !ok {
		return EvalCase{}, false
	}
	adapted := tc
	adapted.Tool = tool
	adapted.Input = flattenCompactCall(operation, tc.Input)
	adapted.Expected.ToolCalled = tool
	if len(tc.Expected.Arguments) > 0 {
		adapted.Expected.Arguments = flattenCompactCall(operation, tc.Expected.Arguments)
	}
	return adapted, true
}

func fullToolToCompactOperation(fullTool string) (compactTool, operation string, ok bool) {
	switch fullTool {
	case "list_projects":
		return "opengrok_projects", "list", true
	case "list_files":
		return "opengrok_projects", "files", true
	case "get_project_overview":
		return "opengrok_projects", "overview", true
	case "search_code":
		return "opengrok_search", "code", true
	case "search_and_read":
		return "opengrok_search", "read", true
	case "search_symbol_definitions":
		return "opengrok_symbols", "definitions", true
	case "search_symbol_references":
		return "opengrok_symbols", "references", true
	case "find_symbol_and_references":
		return "opengrok_symbols", "find", true
	case "search_implementations":
		return "opengrok_symbols", "implementations", true
	case "search_cross_project_references":
		return "opengrok_symbols", "cross_project", true
	case "list_symbols":
		return "opengrok_symbols", "list", true
	case "read_file":
		return "opengrok_read", "file", true
	case "get_file_context":
		return "opengrok_read", "context", true
	default:
		return "", "", false
	}
}
