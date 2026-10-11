# Feature Specification: Consistent Result Counts In Compound Responses

**Feature Branch**: `fix/compound-result-counts`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: Review of v0.6.2 against `docs/tool-contracts.md` § Outputs, which
says paginated responses always carry `total_hits` and `results_on_page`, the
latter "always equal to `len(results)`".

## Context & Motivation

Three compound responses broke that contract:

- `search_and_read` computed `results_on_page` before dropping results whose
  file could not be read, so it could exceed the number of results returned.
- `search_cross_project_references` (compact `opengrok_symbols`
  `operation=cross_project`) never set `results_on_page`, so it was always 0.
- `find_symbol_and_references` (compact `operation=find`) returned neither
  `total_hits` nor `results_on_page`; only `total_references`.

## Decisions

**D1 — count what is returned.** `search_and_read` reports the number of
results in the response; cross-project references report the number of
references across all project groups.

**D2 — add the standard fields to find, keep the old one.** `total_hits` and
`results_on_page` are added; `total_references` stays (same value as
`total_hits`) so existing callers keep working.

## Contract Impact

- `search_and_read` `results_on_page` can decrease when reads fail (now
  correct); a `FILE_READ_FAILED` warning still reports the failures.
- `cross_project` `results_on_page` is populated (previously always 0).
- `find` gains `total_hits` and `results_on_page` (additive).

## Verification

Unit:

- `search_and_read` with one unreadable file: `results_on_page == len(results) == 1`.
- Cross-project references across two projects: `results_on_page == 3`.
- `find`: `total_hits == total_references == 18`, `results_on_page == len(references)`.

Live, against a real OpenGrok instance:

- `find` for `AbstractStrategyJob`: `total_hits` 23, `results_on_page` 20,
  matching the 20 references returned.

## Out of Scope

- Renaming or removing `total_references`.
