// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"path/filepath"
	"testing"
)

func TestLoadScenarios(t *testing.T) {
	dir, err := filepath.Abs("testdata/scenarios")
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := loadScenarios(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 4 {
		t.Fatalf("len(scenarios) = %d, want 4", len(scenarios))
	}
	ids := map[string]bool{}
	for _, sc := range scenarios {
		ids[sc.ID] = true
		if len(sc.Steps) == 0 {
			t.Fatalf("scenario %q has no steps", sc.ID)
		}
	}
	want := []string{
		"symbol-investigation-granular",
		"text-search-and-read",
		"file-exploration",
		"compound-symbol-investigation",
	}
	for _, id := range want {
		if !ids[id] {
			t.Fatalf("missing scenario %q", id)
		}
	}
}

func TestLoadScenariosRejectsDuplicateID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.json")
	writeFile(t, path, `[
		{"id":"a","description":"d","steps":[{"op":"search.code","args":{"query":"x"}}]},
		{"id":"a","description":"d","steps":[{"op":"search.code","args":{"query":"y"}}]}
	]`)
	if _, err := loadScenarios(dir); err == nil {
		t.Fatal("expected duplicate id error")
	}
}
