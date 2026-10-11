# Feature Specification: Cap Line-Centred Context Windows (And Read Each File Once)

**Feature Branch**: `fix/context-window-cap`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: Review of v0.6.2 response-size limits.

## Context & Motivation

A paged file read (`read_file`, compact `opengrok_read` `operation=file`) returns
at most 500 lines per call. A line-centred window had no limit at all:
`get_file_context` (compact `operation=context`) with `after: 1000000` returned
the whole file — up to the 32 MiB response cap — with `truncated: false`, and
`search_and_read` `lines_before`/`lines_after` behaved the same way per result.
One call could pull an entire generated file into an agent's context.

The same `search_and_read` loop also fetched the file once per matching line:
a page with several matches in one file downloaded it repeatedly (10 results in
5 files → 10 requests). Both changes rewrite the same lines, so they ship
together.

## Decisions

**D1 — one cap for every line-centred window: 500 lines.** The same size as a
page of `operation=file` (`filePageSize`), so neither read path can return more
than the other. Applied in a single helper used by `get_file_context`,
`search_and_read`, find-symbol definitions, and auto-expansion.

**D2 — keep the smaller side whole.** When `before + after` exceeds the cap, the
smaller span is kept in full and the larger one gets the remainder (or 249/250
when both are large). A caller asking for "line 1000, 30 before, everything
after" still gets its 30 lines of lead-in.

**D3 — signal the cap.** A capped `get_file_context` window sets
`truncated: true` with a `hint`; `search_and_read` adds a new
`CONTEXT_WINDOW_CAPPED` warning. Windows that cover a short file entirely are
not flagged — nothing was withheld.

**D4 — preserve existing anchoring.** `get_file_context` keeps anchoring on the
requested `line_number` (a line past EOF still yields the last line only, as
pinned by `TestGetFileContextLineBeyondEOFAnchorsToClampedLine`); search-and-read
keeps anchoring on the clamped line. Negative spans count as zero.

**D5 — read each file once per `search_and_read` call.** File content is
memoized by project and path for the duration of the call; a failed read is
remembered too, so `FILE_READ_FAILED` still counts per result.

## Contract Impact

- Line-centred windows return at most 500 lines (previously unbounded).
- `get_file_context` `truncated`/`hint` can now be set in windowed mode.
- New warning code `CONTEXT_WINDOW_CAPPED` (documented in `tool-contracts.md`).
- Default windows (budget tiers) are far below the cap and are unaffected.
- `search_and_read` issues one file request per distinct file (no response
  change).

## Verification

Unit:

- `windowBounds` table: both sides huge, one side small, short file, anchor
  past EOF, negative spans.
- `get_file_context` on a 2,000-line file with `after: 1000000` returns 500
  lines (970–1469) flagged truncated with a hint; a short file is untruncated.
- `search_and_read` with `lines_after: 1000000` caps the window and warns.
- Three matches in one file plus one in another → 2 file fetches.

Live, against a real OpenGrok instance:

- 27,118-line `yarn.lock`, `line_number: 1000`, `before: 30`, `after: 1000000`
  → lines 970–1469, `truncated: true`, hint present; a 5/10 window unchanged.
- `search_and_read` with 10 results in 5 files → 5 `/file/content` requests
  (was 10).

## Out of Scope

- Making the cap configurable.
- Changing the meaning of `before`/`after` = 0 (budget default).
