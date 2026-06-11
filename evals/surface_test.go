// SPDX-License-Identifier: Apache-2.0

package evals

import "testing"

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

func TestResolveCompactSkipsFilesList(t *testing.T) {
	r, err := Resolve(surfaceCompact, "files.list", map[string]any{"project": "platform", "path": "src"})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Skipped {
		t.Fatal("expected skipped")
	}
	if r.SkipReason == "" {
		t.Fatal("expected skip reason")
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
