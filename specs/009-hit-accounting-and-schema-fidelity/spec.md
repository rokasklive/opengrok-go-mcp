# Feature Specification: Honest Hit Accounting and Schema Fidelity

**Feature Branch**: `009-hit-accounting-and-schema-fidelity`

**Created**: 2026-08-01

**Status**: Draft

**Input**: Regression testing against a live OpenGrok instance surfaced seven issues.
Five were fixed directly (snippet markup, `file_type` guidance, `expand_context`
being silently dropped, capability gaps misreported as parse errors) or found to be
working as designed (`total_hits_scope` on kind-filtered symbol listings). The two
remaining are public-contract changes and are specified here.

## Context & Motivation

Two defects share a root cause: **the server reports one thing and delivers another,
and the mismatch is invisible to the caller.**

### Problem 1 — `total_hits` counts documents, `results[]` counts lines

OpenGrok's search API returns `resultCount` (matching **documents**) alongside
`results`, a map of `path → []hit` where one document can carry many line-level
hits. `internal/opengrok/client.go` assigns `TotalHits: r.ResultCount` and then
flattens `results` into one `Hit` per matching **line**. The two fields in the MCP
response therefore measure different things:

```
query "func main" across all projects
  total_hits:     7      (documents)
  len(results):   9      (lines — generics.md matched at lines 26 and 78)
```

The gap is unbounded, not off-by-one: it grows with every additional match inside an
already-matching file. Downstream consequences:

- `total_pages` and `has_more` are computed from a document count while pages are
  filled with line results, so page arithmetic does not describe what arrives.
- The pagination cursor advances `offset` by `pageSize` in document space.
- `PAGE_SIZE_TRUNCATED` reports `len(hits)` (lines) while `total_hits` reports
  documents — an unfiltered query on one project produced `total_hits: 52` beside a
  warning citing `760 hits`. This reads as a bug in the filter and is not one.

An agent cannot tell whether it has seen everything, and cannot trust `total_hits`
to decide whether to paginate or narrow.

### Problem 2 — compact schemas advertise fields the operation rejects

Release 0.5.1 declared per-operation fields at the top level of compact tool schemas
so that strict clients (which ignore nested `oneOf` properties when filtering
arguments) would stop dropping required fields. That fixed the client-side drop, but
the top-level property bag is now the **union** of every operation's fields, while
each `oneOf` branch keeps `additionalProperties: false`. The schema therefore
advertises fields the selected operation refuses:

```
opengrok_search top-level properties: … max_results … page_size …
  oneOf branch operation=code: page_size    (no max_results)
  oneOf branch operation=read: max_results, page_size

{"operation":"code", …, "max_results":2}
  → UNKNOWN_FIELD: Unknown field "max_results" for operation "code".
```

`max_results` and `page_size` are genuinely different parameters — page size versus
how many results the `read` operation expands — so this is not a rename. The defect
is that the schema offers a field the call will reject, and only the error message
reveals which operation owns it.

Secondary cost, measured on the current default surface: of 25,159 bytes of
`tools/list`, 7,313 bytes (29%, ≈1,800 tokens) are property definitions
**byte-identical** between the top-level bag and the `oneOf` branches, paid on every
session.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - An agent can trust the hit count to decide what to do next (Priority: P1)

An agent issues a search, reads `total_hits`, and correctly decides whether to
paginate, narrow, or stop. The number it reads describes the same units as the
results it received, and the pagination fields describe the pages it will actually
get.

**Why this priority**: `total_hits` is load-bearing for the agent's next action, and
today it silently under-reports. Every downstream field (`total_pages`, `has_more`,
cursor offset, truncation warning) inherits the error.

**Independent Test**: A search whose result set contains at least one file with two
or more matching lines. Success = the count field(s) and `len(results)` are
reconcilable from the response alone, without knowing OpenGrok's internals.

**Acceptance Scenarios**:

1. **Given** a query matching 9 lines across 7 files, **When** the agent reads the
   response, **Then** it can determine both figures, each labeled with its unit, and
   no single field silently mixes them.
2. **Given** the same query, **When** the agent walks every page to exhaustion,
   **Then** the number of results it accumulates agrees with the advertised total,
   and `has_more` is false exactly on the last page.
3. **Given** a page truncated by `page_size`, **When** `PAGE_SIZE_TRUNCATED` fires,
   **Then** its figure is in the same unit as the count field it can be compared
   against.
4. **Given** an existing client reading `total_hits`, **When** it upgrades, **Then**
   the unit change is called out as breaking in `CHANGELOG.md` and
   `docs/tool-contracts.md` with a migration note (per D1).
5. **Given** `page_size` bounds documents rather than results (per D2), **When** an
   agent reads the schema, **Then** that is stated, so a page returning more results
   than `page_size` is expected rather than a suspected bug.

---

### User Story 2 - The schema never offers a field the call will reject (Priority: P2)

An agent reads a compact tool schema, picks a field it sees, and the call succeeds —
or the field is visibly scoped to an operation it did not choose. It does not learn
about operation ownership by being rejected.

**Why this priority**: Real but lower-severity than US1 — the error is now
self-correcting since the suggestion names the valid fields (shipped separately).
It costs a wasted round trip rather than a wrong answer.

**Independent Test**: For each compact tool, take every field in the top-level
property bag, issue a call for each operation passing that field, and compare the
outcome against what the schema advertised.

**Acceptance Scenarios**:

1. **Given** a compact tool schema, **When** an agent passes any field the schema
   presents as available for the operation it selected, **Then** the call is not
   rejected with `UNKNOWN_FIELD`.
2. **Given** a field valid for only some operations, **When** the agent reads the
   schema, **Then** the owning operation(s) are discoverable before calling.
3. **Given** a strict client that filters arguments using only top-level properties,
   **When** it calls any operation, **Then** required fields still survive filtering
   — the 0.5.1 regression must not return (covered by its existing test).
4. **Given** the default surface, **When** `tools/list` is measured, **Then** the
   annotation cost is recorded by the token benchmark and stays within a stated
   budget. Removing the ≈1,800 tokens of branch duplication is deferred (D3).

## Out of Scope

- Renaming `max_results` or `page_size`, or merging them. They are distinct
  parameters and the names are public contract.
- `total_hits` on kind-filtered symbol listings. `total_hits_scope:
  "pre_kind_filter"`, `kind_matches_on_page`, and `KIND_FILTER_PAGE_LOCAL` already
  disambiguate it; the naming is a preference, and renaming breaks a documented
  field to fix it.
- Server-side ctags kind filtering. OpenGrok does not support it.
- The five issues already fixed in the same regression pass.

## Decisions

Settled 2026-08-01 by the maintainer.

**D1 — `total_hits` switches to line units.** It becomes the count of matching
lines, agreeing with `len(results)`. This is a **breaking change** to a public
field: clients reading it today get documents and will get a larger number after
upgrade. Requires a migration note in `CHANGELOG.md` and `docs/tool-contracts.md`.

**D2 — pagination stays document-based.** OpenGrok pages by document
(`startDocument`/`endDocument`); line-based paging would require server-side
accumulation and a cursor format change. `page_size` therefore bounds **documents**,
not returned results, and the schema must say so.

**D3 — annotate the compact schemas, do not restructure them.** Each top-level
property states which operations accept it. The `oneOf` branches stay as they are,
so the 0.5.1 strict-client fix is untouched and the ≈1,800-token duplication saving
is deferred.

### Consequence of D1 + D2

With `total_hits` in lines and paging in documents, `total_pages` can no longer be
derived from `total_hits`. The document count must be retained and exposed as its
own field (working name `total_files`), and `total_pages` / `has_more` computed
from it. Both units are then present and labeled, satisfying US1 scenario 1:

```
total_hits:  9   matching lines      — agrees with len(results)
total_files: 7   matching documents  — drives total_pages / has_more / cursor
```

`PAGE_SIZE_TRUNCATED` already reports lines, so under D1 it becomes consistent with
`total_hits` without further change.

## Success Criteria

- **SC-001**: For any search response, a caller can reconcile the advertised total
  against `len(results)` using only fields present in that response.
- **SC-002**: Walking a result set to exhaustion yields a result count that agrees
  with the advertised total; no page reports `has_more: true` when none remain.
- **SC-003**: No warning compares a figure against a count field in a different unit.
- **SC-004**: For every compact tool, every (operation, advertised field) pair either
  succeeds or is not advertised for that operation. Enforced by a generated test that
  walks the published schema, so it cannot drift.
- **SC-005**: The 0.5.1 strict-client behavior is preserved.
- **SC-006**: The `tools/list` byte delta from D3's annotations is measured by the
  token benchmark and stays within a budget agreed before implementation.
