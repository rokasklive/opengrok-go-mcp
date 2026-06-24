// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "testing"

func TestWarningSetFields(t *testing.T) {
	w := newWarningSet()
	w.add(warnPageSizeTruncated, "truncated to 1")
	w.add(warnHighHitCount, "many hits")

	fields := w.fields()
	if len(fields.Warnings) != 2 {
		t.Fatalf("len(Warnings) = %d, want 2", len(fields.Warnings))
	}
	if fields.Warnings[0].Code != warnPageSizeTruncated {
		t.Fatalf("Warnings[0].Code = %q, want %s", fields.Warnings[0].Code, warnPageSizeTruncated)
	}
	if fields.Warning == nil || *fields.Warning != "truncated to 1 many hits" {
		t.Fatalf("Warning = %v, want joined legacy string", fields.Warning)
	}
}

func TestWarningSetMergeStructured(t *testing.T) {
	w := newWarningSet()
	w.merge(WarningFields{
		Warnings: []Warning{{Code: warnAutoQuotedQuery, Message: "quoted"}},
	})
	fields := w.fields()
	if len(fields.Warnings) != 1 || fields.Warnings[0].Code != warnAutoQuotedQuery {
		t.Fatalf("merged warnings = %+v, want AUTO_QUOTED_QUERY", fields.Warnings)
	}
}
