# Feature Specification: Window-Level `max_hits_per_file`

**Feature Branch**: `fix/max-hits-per-file-window`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: Review of v0.6.2; interaction between `max_hits_per_file` and the
line-offset paging introduced in 010.

## Context & Motivation

Since 010, a page is a slice of the line matches of a file window, resumed at
`LineOffset`. `max_hits_per_file` was applied *after* that slice was cut, so it
only saw one page at a time. A line-dense file therefore resurfaced on every
page of its window, one capped line each:

```
page_size=10, max_hits_per_file=1, top file has 50 matching lines
pages 1-5 → one result each, all from that same file
```

The `PAGE_SIZE_TRUNCATED` count also included lines the cap was about to
discard, overstating what the next page would bring.

## Decisions

**D1 — apply the cap to the window, before paging.** The filter runs on the
window's hits (`[]opengrok.Hit`) before the page is cut, so a file contributes
at most `max_hits_per_file` lines to the whole walk of that window, and the
truncation count reflects lines that will actually be returned.

**D2 — the cursor carries the cap.** `LineOffset` now indexes the *filtered*
window, so the filter must stay fixed for the whole walk. `cursor.State` gains
`max_hits_per_file` (`omitempty`), and — like `page_size` — the cursor's value
wins over the request. A caller that omits `max_hits_per_file` on later pages
(common: only `cursor` plus the query is resent) keeps a consistent position.
Cursors minted before this change decode to `0` (no cap).

## Contract Impact

- Under `max_hits_per_file`, a file appears at most that many times per file
  window across pages (previously once per page).
- Cursor gains an opaque `max_hits_per_file`; format stays backward compatible.
- `PAGE_SIZE_TRUNCATED` counts only lines that remain after the cap.
- `gofmt` realigns `cursor.State` (it was not gofmt-clean upstream).

## Verification

Unit:

- Window of 6 lines in A plus one each in B and C, `page_size=1`,
  `max_hits_per_file=1`, later pages sending only the cursor: results are
  `A, B, C` exactly once, and page 1 reports 2 remaining lines (not 7).
- Cursor round-trips `max_hits_per_file`; negative values are rejected.

Live, against a real OpenGrok instance:

- `page_size=3`, `max_hits_per_file=1`, cursor-only follow-ups: 12 results over
  4 pages from 12 distinct files.

## Out of Scope

- A global (cross-window) per-file cap; OpenGrok pages by file windows and the
  server keeps no walk state beyond the cursor.
