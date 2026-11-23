// SPDX-License-Identifier: Apache-2.0

package evals

import "fmt"

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

func formatEstTokens(est int) string {
	if est >= 10000 {
		return fmt.Sprintf("%.0fk", float64(est)/1000)
	}
	if est >= 1000 {
		return fmt.Sprintf("%.1fk", float64(est)/1000)
	}
	return fmt.Sprintf("%d", est)
}
