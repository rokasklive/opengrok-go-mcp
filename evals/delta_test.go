// SPDX-License-Identifier: Apache-2.0

package evals

import "testing"

func TestDeltaEstTokensStr(t *testing.T) {
	if got := deltaEstTokensStr(14000, 14000, true); got != "14k (Δ ±0)" {
		t.Fatalf("got %q", got)
	}
	if got := deltaEstTokensStr(15000, 14000, true); got != "15k (Δ +1.0k)" {
		t.Fatalf("got %q", got)
	}
	if got := deltaEstTokensStr(13000, 14000, true); got != "13k (Δ -1.0k)" {
		t.Fatalf("got %q", got)
	}
	if got := deltaEstTokensStr(14000, 0, false); got != "14k" {
		t.Fatalf("got %q", got)
	}
}

func TestDeltaEstRangeStr(t *testing.T) {
	got := deltaEstRangeStr(14000, 15000, 14000, 15000, true)
	if got != "14k–15k (Δ ±0)" {
		t.Fatalf("got %q", got)
	}
	got = deltaEstRangeStr(13000, 14000, 14000, 15000, true)
	if got != "13k–14k (was 14k–15k)" {
		t.Fatalf("got %q", got)
	}
}

func TestDeltaScoreStr(t *testing.T) {
	if got := deltaScoreStr(1.0, 0.9, true); got != "100% (Δ +10.0%)" {
		t.Fatalf("got %q", got)
	}
}
