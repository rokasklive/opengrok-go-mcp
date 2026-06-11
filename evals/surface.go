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
	default:
		return ResolveResult{}, fmt.Errorf("unmapped op %q for full", op)
	}
}

func resolveCompact(op string, args map[string]any) (ResolveResult, error) {
	switch op {
	case "search.definitions":
		return ResolveResult{
			Tool:      "opengrok_search",
			Arguments: map[string]any{"operation": "definitions", "payload": args},
		}, nil
	case "search.references":
		return ResolveResult{
			Tool:      "opengrok_search",
			Arguments: map[string]any{"operation": "references", "payload": args},
		}, nil
	case "search.code":
		return ResolveResult{
			Tool:      "opengrok_search",
			Arguments: map[string]any{"operation": "code", "payload": args},
		}, nil
	case "path.search":
		args["mode"] = "path"
		return ResolveResult{
			Tool:      "opengrok_search",
			Arguments: map[string]any{"operation": "code", "payload": args},
		}, nil
	case "read.file":
		return ResolveResult{
			Tool:      "opengrok_read",
			Arguments: map[string]any{"operation": "file", "payload": args},
		}, nil
	case "files.list":
		return ResolveResult{
			Skipped:    true,
			SkipReason: "compact surface has no list_files equivalent",
		}, nil
	case "compound.find_symbol":
		return ResolveResult{
			Tool:      "opengrok_compound",
			Arguments: map[string]any{"operation": "find_symbol_and_references", "payload": args},
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
	default:
		return ""
	}
}
