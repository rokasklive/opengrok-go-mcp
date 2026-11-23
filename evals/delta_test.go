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
