// SPDX-License-Identifier: Apache-2.0

package mcpserver

import "strings"

// Warning is a structured, machine-readable contract signal (L2).
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WarningFields carries structured warnings and a legacy joined warning string.
type WarningFields struct {
	Warning  *string   `json:"warning,omitempty"`
	Warnings []Warning `json:"warnings,omitempty"`
}

const (
	warnPageSizeTruncated   = "PAGE_SIZE_TRUNCATED"
	warnAutoQuotedQuery     = "AUTO_QUOTED_QUERY"
	warnDateIgnored         = "DATE_IGNORED_OUTSIDE_HISTORY"
	warnHighHitCount        = "HIGH_HIT_COUNT"
	warnSortUnsupported     = "SORT_UNSUPPORTED"
	warnKindFilterPageLocal = "KIND_FILTER_PAGE_LOCAL"
	warnLargeSymbolList     = "LARGE_SYMBOL_LIST"
	warnFileListTruncated   = "FILE_LIST_TRUNCATED"
	warnExpansionIncomplete = "EXPANSION_INCOMPLETE"
	warnFileReadFailed      = "FILE_READ_FAILED"
	warnNoDefinitionFound   = "NO_DEFINITION_FOUND"
	warnBestEffortImpl      = "BEST_EFFORT_IMPLEMENTATION"
)

type warningSet struct {
	items []Warning
}

func newWarningSet() *warningSet {
	return &warningSet{}
}

func (w *warningSet) add(code, message string) {
	if message == "" {
		return
	}
	w.items = append(w.items, Warning{Code: code, Message: message})
}

func (w *warningSet) merge(other WarningFields) {
	if len(other.Warnings) > 0 {
		w.items = append(w.items, other.Warnings...)
		return
	}
	if other.Warning != nil && *other.Warning != "" {
		w.add("LEGACY_WARNING", *other.Warning)
	}
}

func (w *warningSet) fields() WarningFields {
	if len(w.items) == 0 {
		return WarningFields{}
	}
	return WarningFields{
		Warning:  joinWarningMessages(w.items),
		Warnings: append([]Warning(nil), w.items...),
	}
}

func joinWarningMessages(warnings []Warning) *string {
	if len(warnings) == 0 {
		return nil
	}
	parts := make([]string, len(warnings))
	for i, w := range warnings {
		parts[i] = w.Message
	}
	joined := strings.Join(parts, " ")
	return &joined
}

// appendWarning joins msg onto an existing warning string (legacy helper).
func appendWarning(existing *string, msg string) *string {
	if msg == "" {
		return existing
	}
	if existing == nil || *existing == "" {
		return &msg
	}
	combined := *existing + " " + msg
	return &combined
}
