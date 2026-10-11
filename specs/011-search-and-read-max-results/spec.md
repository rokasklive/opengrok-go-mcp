# Feature Specification: Lossless `max_results` In Search-And-Read

**Feature Branch**: `fix/search-and-read-max-results`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: Review of v0.6.2 against a live OpenGrok instance; follow-up to 010.

## Context & Motivation

`search_and_read` (compact `opengrok_search` with `operation=read`) applied
`max_results` *after* `search()` had already minted `next_cursor` past the whole
page. Results between `max_results` and `page_size` were trimmed from the
response, but the cursor had moved beyond them, so no page could ever return
them — while the same response's `PAGE_SIZE_TRUNCATED` warning stated the
surplus was "NOT lost". This is the 010 defect again, on the compound path.

```
page_size=5, max_results=2
page 1 → QuoteCommonJob.java:5, QuoteCommonJob.java:22
page 2 → EndorsementInitiatePurchaseJob.java:24     ← skipped 3 matches:
         AmendmentRateJob.java:5, AmendmentRateJob.java:24,
         EndorsementInitiatePurchaseJob.java:5
```

## Decisions

**D1 — cap the lines inside `search()`, before the cursor is minted.**
`searchRequest` gains `maxLines`; a page is cut at `min(page_size, max_results)`
lines and `LineOffset` advances by exactly what was returned. The OpenGrok file
window (`page_size` files) is unchanged, so the cursor stays consistent even if a
caller changes `max_results` between pages.

**D2 — `PAGE_SIZE_TRUNCATED` names the limiting field.** The code is unchanged;
the message says `max_results` when that is the cap that bit, so the caller is
told which parameter to raise.

**D3 — describe `max_results` honestly.** The schema description now says
results beyond the cap are reachable through `next_cursor` and that `0` means
"up to `page_size`" (previously "read all", which was never true).

## Contract Impact

- Results past `max_results` are returned on later pages (previously dropped).
- A page still holds at most `max_results` results.
- `PAGE_SIZE_TRUNCATED` message may name `max_results`; code unchanged.
- Cursor format unchanged.

## Verification

Unit:

- Exhaustive walk with `page_size=5`, `max_results=2` over five line matches
  returns lines `[1..5]` exactly once, in order, and page 1 warns about
  `max_results`.

Live, against a real OpenGrok instance:

- The query above now pages as `[Quote:5, Quote:22]`,
  `[AmendmentRate:5, AmendmentRate:24]`, `[EndorsementInitiatePurchase:5, :24]`
  — the first five matches all reached, in order.

## Out of Scope

- Changing `page_size` semantics or the OpenGrok file-window size.
