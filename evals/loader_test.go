// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCasesValidationDuplicateID(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.json"), `[{"id":"dup","tool":"search_code","description":"d","input":{},"expected":{"tool_called":"search_code","arguments":{},"result_checks":[{"type":"no_error"}]}}]`)
	writeFile(t, filepath.Join(dir, "b.json"), `[{"id":"dup","tool":"search_code","description":"d","input":{},"expected":{"tool_called":"search_code","arguments":{},"result_checks":[{"type":"no_error"}]}}]`)

	_, err := loadCases(dir)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestLoadCasesValidationMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.json"), `not json`)

	_, err := loadCases(dir)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadCasesValidationEmptyTool(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.json"), `[{"id":"x","tool":"","description":"d","input":{},"expected":{"tool_called":"","arguments":{},"result_checks":[{"type":"no_error"}]}}]`)

	_, err := loadCases(dir)
	if err == nil {
		t.Fatal("expected empty tool error")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
