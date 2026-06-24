# Contract: Kind-Filter & Catalog Output

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24
**Status**: design.

Additive MCP output contract; no input changes.

## ListSymbolsOutput (when `kind` input non-empty)

| Field | Type | Required | Meaning |
|-------|------|----------|---------|
| `kind_filter_active` | `bool` | yes | `true` |
| `kind_matches_on_page` | `int` | yes | `len(symbols)` after page-local filter |
| `total_hits_scope` | `string` | yes | constant `pre_kind_filter` |

`total_hits` unchanged — global OpenGrok definition count before kind filter.

`warnings[]` still includes `KIND_FILTER_PAGE_LOCAL` when `has_more`.

When `kind` input empty: omit the three fields above.

## ListProjectsOutput (always)

| Field | Type | Required |
|-------|------|----------|
| `catalog_source` | `string` | yes |
| `catalog_is_snapshot` | `bool` | yes (`true`) |

Values for `catalog_source`: `configured`, `api`, `scraped`, `none`.

## UNKNOWN_PROJECT error

When project not in startup allowlist, `message` MUST include:

- Snapshot semantics (list is startup-resolved)
- Suggestion to restart server after OpenGrok project changes
- Existing allowlist + `source=` detail (preserve today)

`error_code`: `UNKNOWN_PROJECT` (unchanged).

## Acceptance

- `evals/testdata/search_symbols.json` or trajectory case with kind filter.
- Unit tests in `symbols_test.go` for field presence/absence.
- `projects_test.go` for catalog fields on list.
