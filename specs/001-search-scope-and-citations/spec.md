# Search scope and source citations

**Created** 2026-11-23

**Status** Implemented

## User scenarios and testing

### P1 Preserve explicitly requested scope

An agent that supplies a project or project list gets a scoped search or a
structured error. The server rejects an unknown project when the configured
project catalog is non-empty, even when project selection is optional.

Test through full, compact, and gateway. An invalid name or a mixed valid and
invalid list must produce `UNKNOWN_PROJECT` before contacting OpenGrok. Also
check default project selection and intentional unscoped searches.

### P1 Cite current evidence accurately

When the fetched file ends before the indexed line, preserve the indexed hit
and replace its line links with file links. Omit the source window and emit
`STALE_SEARCH_HIT` so the agent can locate the current line independently.

Test compound text search, compound symbol lookup, and search expansion
through all three surfaces. Valid hits retain their source windows and line
links. Ambiguously attributed hits retain attribution metadata and have no
source window or URL.

## Constitution alignment

- **MCP contract** preserves existing tool inputs and cursor behavior. Adds
  two warning codes and propagates attribution fields to compound results.
- **OpenGrok semantics** distinguishes index hits from current file reads.
  A line within bounds provides no revision or content consistency guarantee.
- **Security** keeps invalid explicit project selections from widening scope.
  The project catalog identifies known projects.
- **Documentation** covers behavior in the README, configuration, tool
  contracts, and limitations.
- **Experimental behavior** uses the same rules on the experimental gateway.
- **Resource bounds** reuse existing fetch limits. Warnings are aggregated by
  type for each response.

## Functional requirements

- **FR-001** Return `UNKNOWN_PROJECT` for explicitly unknown projects before
  issuing any search request, regardless of the optional project setting.
- **FR-002** Preserve valid explicit scopes, default project selection, and
  intentional unscoped search behavior.
- **FR-003** For a fetched line past EOF, keep the indexed line and any
  returned snippet. Omit the source window and its line links. Include a
  file-level citation and the structured and legacy warning.
- **FR-004** Preserve uncertainty metadata and omit file fetches and links
  for hits whose project attribution is uncertain. Emit
  `ATTRIBUTION_UNCERTAIN` on the shared search responses.
- **FR-005** Preserve file-level hits with no indexed line and direct file
  read semantics.

## Success criteria

- All three surfaces return the same error for invalid explicit scopes.
- All three surfaces return usable source windows for valid hits and
  consistent recovery signals for stale or uncertain evidence.
- The race suite and MCP contract evals pass.

## Assumptions

OpenGrok may serve indexed hits and current file content from different
revisions. The service can check the bounds of a fetched file using its
existing file API.
