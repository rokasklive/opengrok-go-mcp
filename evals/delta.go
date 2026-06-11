// SPDX-License-Identifier: Apache-2.0

package evals

import "fmt"

func deltaScoreStr(cur, prev float64, had bool) string {
	if !had {
		return fmt.Sprintf("%.0f%%", cur*100)
	}
	d := (cur - prev) * 100
	switch {
	case d > 0.05:
		return fmt.Sprintf("%.0f%% (Δ +%.1f%%)", cur*100, d)
	case d < -0.05:
		return fmt.Sprintf("%.0f%% (Δ %.1f%%)", cur*100, d)
	default:
		return fmt.Sprintf("%.0f%% (Δ ±0)", cur*100)
	}
}

func deltaEstTokensStr(cur, prev int, had bool) string {
	if !had {
		return formatEstTokens(cur)
	}
	val := formatEstTokens(cur)
	d := cur - prev
	switch {
	case d == 0:
		return val + " (Δ ±0)"
	case d > 0:
		return fmt.Sprintf("%s (Δ +%s)", val, formatEstDelta(d))
	default:
		return fmt.Sprintf("%s (Δ %s)", val, formatEstDelta(d))
	}
}

func deltaEstRangeStr(minCur, maxCur, minPrev, maxPrev int, had bool) string {
	curRange := formatEstRange(minCur, maxCur)
	if !had {
		return curRange
	}
	if minCur == minPrev && maxCur == maxPrev {
		return curRange + " (Δ ±0)"
	}
	prevRange := formatEstRange(minPrev, maxPrev)
	return fmt.Sprintf("%s (was %s)", curRange, prevRange)
}

func formatEstRange(min, max int) string {
	if min == max {
		return formatEstTokens(min)
	}
	return fmt.Sprintf("%s–%s", formatEstTokens(min), formatEstTokens(max))
}

func formatEstDelta(d int) string {
	if d == 0 {
		return "0"
	}
	sign := ""
	abs := d
	if d < 0 {
		sign = "-"
		abs = -d
	}
	if abs >= 1000 {
		return sign + formatEstTokens(abs)
	}
	return fmt.Sprintf("%d", d)
}
