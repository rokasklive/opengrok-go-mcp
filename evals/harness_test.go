// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"context"
	"path/filepath"
	"testing"
)

func TestHarnessSurfaceListToolsDiffer(t *testing.T) {
	ctx := context.Background()
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	testdataDir, err := filepath.Abs("testdata")
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

	if len(full.ListedTools()) == 0 || len(compact.ListedTools()) == 0 {
		t.Fatal("expected tools on both surfaces")
	}
	if full.ListedTools()[0].Name == compact.ListedTools()[0].Name {
		t.Fatalf("expected different first tool names, got %q and %q",
			full.ListedTools()[0].Name, compact.ListedTools()[0].Name)
	}
}
