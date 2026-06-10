// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"sort"
	"time"
)

func aggregate(name string, results []EvalResult) SuiteResult {
	s := SuiteResult{
		SuiteName: name,
		Mode:      "direct-call",
		Timestamp: time.Now(),
		Total:     len(results),
		Results:   results,
		PerTool:   map[string]float64{},
	}

	var (
		scoreSum   float64
		judged     int
		latencies  []time.Duration
		perToolSum = map[string]float64{}
		perToolN   = map[string]int{}
	)

	for _, r := range results {
		if r.Skipped {
			s.Skipped++
			continue
		}
		judged++
		if r.Passed {
			s.Passed++
		} else {
			s.Failed++
		}
		scoreSum += r.Score
		latencies = append(latencies, r.Latency)
		perToolSum[r.Tool] += r.Score
		perToolN[r.Tool]++
	}

	if judged > 0 {
		s.Score = scoreSum / float64(judged)
	}
	if s.Total > 0 {
		s.CoverageK = float64(judged) / float64(s.Total)
	}
	for tool, sum := range perToolSum {
		s.PerTool[tool] = sum / float64(perToolN[tool])
	}
	s.LatencyP50 = percentile(latencies, 50)
	s.LatencyP95 = percentile(latencies, 95)
	s.LatencyP99 = percentile(latencies, 99)
	return s
}

func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), d...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	rank := (p*len(cp) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	if rank > len(cp) {
		rank = len(cp)
	}
	return cp[rank-1]
}
