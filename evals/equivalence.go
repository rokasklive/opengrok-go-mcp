// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// assertOutputsEquivalent compares load-bearing output fields across surfaces.
func assertOutputsEquivalent(full, compact map[string]any) error {
	if full == nil || compact == nil {
		return fmt.Errorf("missing structured output (full=%v compact=%v)", full != nil, compact != nil)
	}
	for _, key := range []string{"total_hits", "total_projects", "total_references", "total_lines", "next_cursor", "warning"} {
		fv, fok := full[key]
		cv, cok := compact[key]
		if fok != cok {
			return fmt.Errorf("field %q presence mismatch (full=%v compact=%v)", key, fok, cok)
		}
		if fok && !reflect.DeepEqual(fv, cv) {
			return fmt.Errorf("field %q differs: full=%#v compact=%#v", key, fv, cv)
		}
	}
	if err := compareResults(full["results"], compact["results"]); err != nil {
		return err
	}
	return nil
}

func compareResults(fullResults, compactResults any) error {
	fItems, fok := fullResults.([]any)
	cItems, cok := compactResults.([]any)
	if fok != cok {
		return fmt.Errorf("results presence mismatch")
	}
	if !fok {
		return nil
	}
	if len(fItems) != len(cItems) {
		return fmt.Errorf("results length differs: full=%d compact=%d", len(fItems), len(cItems))
	}
	fKeys := resultKeys(fItems)
	cKeys := resultKeys(cItems)
	if !reflect.DeepEqual(fKeys, cKeys) {
		return fmt.Errorf("results differ: full=%#v compact=%#v", fKeys, cKeys)
	}
	return nil
}

func resultKeys(items []any) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		path, _ := m["file_path"].(string)
		line, _ := m["line_number"].(float64)
		cite := ""
		if citation, ok := m["citation"].(map[string]any); ok {
			if url, ok := citation["url"].(string); ok {
				cite = citationPath(url)
			}
		}
		keys = append(keys, fmt.Sprintf("%s:%.0f:%s", path, line, cite))
	}
	slices.Sort(keys)
	return keys
}

func citationPath(url string) string {
	if idx := strings.Index(url, "/source/"); idx >= 0 {
		return url[idx:]
	}
	return url
}
