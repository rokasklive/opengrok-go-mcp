// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResolveCompactFilesList(t *testing.T) {
	r, err := Resolve(surfaceCompact, "files.list", map[string]any{"project": "platform", "path": "src"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped {
		t.Fatal("expected files.list to resolve on compact")
	}
	if r.Tool != "opengrok_projects" {
		t.Fatalf("tool = %q", r.Tool)
	}
	if r.Arguments["operation"] != "files" {
		t.Fatalf("operation = %v", r.Arguments["operation"])
	}
}

func TestResolveCompactProjectsOverview(t *testing.T) {
	r, err := Resolve(surfaceCompact, "projects.overview", map[string]any{"project": "platform"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "opengrok_projects" || r.Arguments["operation"] != "overview" {
		t.Fatalf("got tool=%q op=%v", r.Tool, r.Arguments["operation"])
	}
}

func TestResolveCompactCompoundFindSymbol(t *testing.T) {
	r, err := Resolve(surfaceCompact, "compound.find_symbol", map[string]any{
		"symbol":  "PaymentProcessor",
		"project": "platform",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "opengrok_symbols" || r.Arguments["operation"] != "find" {
		t.Fatalf("got tool=%q op=%v", r.Tool, r.Arguments["operation"])
	}
}

func TestResolveFullSymbolDefinitions(t *testing.T) {
	r, err := Resolve(surfaceFull, "search.definitions", map[string]any{
		"symbol":  "Engine",
		"project": "platform",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "search_symbol_definitions" {
		t.Fatalf("tool = %q", r.Tool)
	}
}

func TestResolveGatewayCompound(t *testing.T) {
	r, err := Resolve(surfaceGateway, "compound.find_symbol", map[string]any{
		"symbol":  "PaymentProcessor",
		"project": "platform",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "opengrok_call" {
		t.Fatalf("tool = %q", r.Tool)
	}
	if r.Arguments["operation"] != "compound.find_symbol_and_references" {
		t.Fatalf("operation = %v", r.Arguments["operation"])
	}
}

func TestResolvePathSearchSetsMode(t *testing.T) {
	r, err := Resolve(surfaceFull, "path.search", map[string]any{"query": "Engine.swift", "project": "platform"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Arguments["mode"] != "path" {
		t.Fatalf("mode = %v", r.Arguments["mode"])
	}
}

func TestCrossSurfaceEquivalenceOnSharedCases(t *testing.T) {
	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	testdataDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	cases, err := loadCases(testdataDir)
	if err != nil {
		t.Fatal(err)
	}

	full, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{ToolSurface: surfaceFull})
	if err != nil {
		t.Fatal(err)
	}
	defer full.Stop()

	compact, err := Start(ctx, moduleRoot, testdataDir, HarnessOptions{ToolSurface: surfaceCompact})
	if err != nil {
		t.Fatal(err)
	}
	defer compact.Stop()

	for _, tc := range cases {
		compactTC, ok := adaptEvalCaseForCompact(tc)
		if !ok {
			continue
		}
		if !full.hasTool(tc.Tool) || !compact.hasTool(compactTC.Tool) {
			continue
		}
		t.Run(tc.ID, func(t *testing.T) {
			fullOut, fullErr := full.Session().CallTool(ctx, callParams(tc))
			compactOut, compactErr := compact.Session().CallTool(ctx, callParams(compactTC))
			if fullErr != nil || compactErr != nil {
				t.Fatalf("call errors: full=%v compact=%v", fullErr, compactErr)
			}
			if (fullOut != nil && fullOut.IsError) || (compactOut != nil && compactOut.IsError) {
				t.Fatalf("tool errors: full=%v compact=%v", fullOut != nil && fullOut.IsError, compactOut != nil && compactOut.IsError)
			}
			if err := assertOutputsEquivalent(structured(fullOut), structured(compactOut)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func callParams(tc EvalCase) *mcp.CallToolParams {
	return &mcp.CallToolParams{Name: tc.Tool, Arguments: tc.Input}
}
