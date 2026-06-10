// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "testing"

func TestNormalizeCodeQuery(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		tokenized      bool
		wantNormalized string
		wantAutoQuoted bool
	}{
		{"bare multi-word is quoted", "extends PaymentProcessor", false, `"extends PaymentProcessor"`, true},
		{"single word unchanged", "PaymentProcessor", false, "PaymentProcessor", false},
		{"already quoted unchanged", `"extends PaymentProcessor"`, false, `"extends PaymentProcessor"`, false},
		{"plus operator unchanged", "+Payment -test", false, "+Payment -test", false},
		{"minus operator unchanged", "foo -bar", false, "foo -bar", false},
		{"field syntax unchanged", "defs:Foo bar", false, "defs:Foo bar", false},
		{"wildcard star unchanged", "Payment* Service", false, "Payment* Service", false},
		{"wildcard question unchanged", "Payment? Service", false, "Payment? Service", false},
		{"AND boolean unchanged", "Foo AND Bar", false, "Foo AND Bar", false},
		{"OR boolean unchanged", "Foo OR Bar", false, "Foo OR Bar", false},
		{"NOT boolean unchanged", "Foo NOT Bar", false, "Foo NOT Bar", false},
		{"word containing AND still quoted", "Android build", false, `"Android build"`, true},
		{"tokenized opt-out unchanged", "extends PaymentProcessor", true, "extends PaymentProcessor", false},
		{"empty unchanged", "", false, "", false},
		{"whitespace trimmed and not quoted", "  PaymentProcessor  ", false, "PaymentProcessor", false},
		{"leading/trailing space around phrase is quoted trimmed", "  extends PaymentProcessor  ", false, `"extends PaymentProcessor"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNorm, gotAuto := normalizeCodeQuery(tt.query, tt.tokenized)
			if gotNorm != tt.wantNormalized {
				t.Errorf("normalized = %q, want %q", gotNorm, tt.wantNormalized)
			}
			if gotAuto != tt.wantAutoQuoted {
				t.Errorf("autoQuoted = %v, want %v", gotAuto, tt.wantAutoQuoted)
			}
		})
	}
}

func TestIsMultiWord(t *testing.T) {
	if isMultiWord("one") {
		t.Error("single word reported multi-word")
	}
	if !isMultiWord("one two") {
		t.Error("two words not reported multi-word")
	}
	if isMultiWord("   ") {
		t.Error("whitespace-only reported multi-word")
	}
}

func TestQueryHasDateField(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"date:[20230101 TO 20261231]", true},
		{"foo date:20230101", true},
		{"Engine", false},
		{"candidate:Foo", false},
		{"update:bar", false},
		{"invalidate:x", false},
		{"", false},
		{"defs:Foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			if got := queryHasDateField(tt.query); got != tt.want {
				t.Errorf("queryHasDateField(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestAppendPathExcludes(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		pathExclude string
		want        string
	}{
		{"empty exclude unchanged", `"foo bar"`, "", `"foo bar"`},
		{"single token", `"foo bar"`, "legacy", `"foo bar" -path:legacy`},
		{"multiple tokens", `"foo bar"`, "service test", `"foo bar" -path:service -path:test`},
		{"extra whitespace ignored", "Engine", "  a   b ", "Engine -path:a -path:b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := appendPathExcludes(tt.query, tt.pathExclude); got != tt.want {
				t.Errorf("appendPathExcludes(%q,%q) = %q, want %q", tt.query, tt.pathExclude, got, tt.want)
			}
		})
	}
}
