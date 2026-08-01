# Feature Specification: Lossless, Deterministic Search Pagination

**Feature Branch**: `010-lossless-search-pagination`

**Created**: 2026-08-01

**Status**: Implemented

**Input**: Follow-up to 009. Documenting `page_size` semantics surfaced a defect
where paging silently dropped results; investigating it surfaced a second,
deeper one in result ordering.

## Context & Motivation

Two defects, the second a prerequisite for fixing the first.

### Problem 1 — paging dropped line matches permanently

OpenGrok pages by **file** (`start`/`maxresults` select documents), but the
server capped a page at `page_size` **lines** and then advanced the cursor by
`page_size` files. Every line match beyond the cap inside an already-fetched
file was discarded, and no cursor could return to it:

```
page_size=2 → OpenGrok returns 85 line matches across the 2 files fetched
            → 2 returned: concurrency.md:18, concurrency.md:22
page 2      → interfaces.md:50     ← advanced past both files
                                     83 line matches unreachable
```

A caller walking to exhaustion saw a small fraction of the matches while every
signal in the response indicated completeness.

### Problem 2 — result order was non-deterministic

`searchResponse.Results` is a `map[string][]searchHit`, and the flattener ranged
over it directly. Go randomizes map iteration, so the order of files in
`results[]` differed per call; combined with truncation, **identical queries
returned different results**:

```
5 identical calls, page_size=3:
  runs 0-3 → generics.md:9, :17, :26
  run 4    → concurrency.md:18, :22, :25
```

This also made Problem 1 unfixable: resuming at a saved line offset requires a
stable order to resume into.

## Decisions

**D1 — preserve OpenGrok's emission order, do not sort.** A custom
`UnmarshalJSON` records the key order of the `results` object and the flattener
follows it. Sorting by path would have been two lines but silently replaces
OpenGrok's relevance ranking, which `sort=relevance` documents as the default.
Responses not built through `UnmarshalJSON` (test fixtures) fall back to sorted
order so callers never see map randomization.

**D2 — resume inside the file window via a cursor line offset.** `cursor.State`
gains `LineOffset`, marking the position within the flattened line hits of the
window starting at `Offset`. While lines remain in a window the cursor holds
`Offset` steady and advances `LineOffset`; when the window is exhausted it
advances `Offset` by `page_size` files and resets `LineOffset` to 0. The field
is `omitempty` and absent in previously minted cursors, which decode to 0 — the
prior behavior — so old cursors remain valid.

**D3 — keep `PAGE_SIZE_TRUNCATED`, change what it says.** The code is public
contract and still fires in the same condition, but it no longer describes data
loss: it now states the surplus is reachable via `next_cursor`. Removing the
code would have broken clients keying on it.

**D4 — `total_pages` becomes a lower bound rather than being recomputed.** It
counts file pages while a walk may take more, and the true count is unknowable
because OpenGrok reports no global line count (`resultCount` is constant
regardless of `maxresults`). Documented as a lower bound; `has_more` and
`next_cursor` remain exact.

## Contract Impact

- `results[]` ordering is now stable across identical calls (previously random).
- Cursors gain an internal `line_offset`; format is opaque and backward
  compatible.
- `PAGE_SIZE_TRUNCATED` message changed; code unchanged.
- A stale line offset (window shrank under a reindex) returns `INVALID_CURSOR`
  rather than silently restarting, which would present as a pagination loop.
- `total_hits`, `results_on_page`, `page_size`, and `has_more` semantics are
  unchanged.

## Verification

Unit:

- Emission order preserved across 50 unmarshal/flatten cycles with
  deliberately unsorted keys.
- Deterministic sorted fallback for hand-built responses.
- Exhaustive walk over a document-window backend yields every line exactly once,
  in order (`[1..7]` from a 4-file corpus at `page_size` 2).
- Stale `LineOffset` rejected as `INVALID_CURSOR`.

Live, against a real OpenGrok instance:

- 5 identical queries returned identical results (previously divergent).
- Full walk of `func` in one project: **910 results, 910 unique, 0 duplicates,
  52 distinct files, clean termination** — matching the independently probed
  ground truth of 52 documents / 910 lines.

## Out of Scope

- Reducing the re-fetch cost of resuming inside a window (each page re-requests
  the same file window and skips `LineOffset` lines). Correctness first; the
  response cache already absorbs repeat fetches when enabled.
- Exposing a global line count. OpenGrok does not report one.
- The ≈1,800 tokens of `oneOf` duplication deferred by 009 D3.
