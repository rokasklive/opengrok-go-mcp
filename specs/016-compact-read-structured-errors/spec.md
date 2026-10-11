# Feature Specification: Structured Errors From `opengrok_read`

**Feature Branch**: `fix/compact-read-error-code`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: Live testing of v0.6.2; `docs/tool-contracts.md` § Errors promises
`error_code` in `StructuredContent` for failed tool calls.

## Context & Motivation

Failed calls to compact `opengrok_read` returned the right error *text*, but
`StructuredContent` held an empty `FileContextOutput` instead of the error body
— no `error_code`, no `suggestion`:

```
opengrok_read {"operation":"file","file_path":"no/such/File.java"}
→ isError: true
  content: "Resource not found at /raw/... Verify project and file_path ..."
  structuredContent: {"content":"","file_path":"","project":"", ...}   ← no error_code
```

The other three compact tools were unaffected. Cause: their handlers declare
output type `any`, while `opengrok_read` declared `FileContextOutput`. For a
concrete output type, go-sdk v1.7.0 marshals the handler's (zero) output into
`StructuredContent` whenever the handler returns no Go error — which is exactly
how `wrapToolHandler` returns a structured tool error — overwriting the body.

## Decisions

**D1 — declare `any`, like the other compact tools.** The handler returns the
same `FileContextOutput` value on success, so successful responses are
unchanged; on error, go-sdk skips the nil output and the error body survives.

**D2 — accept losing the advertised `outputSchema`.** With output type `any`,
`opengrok_read` no longer advertises an output schema — matching the other
compact tools, none of which do. Keeping a schema would also make clients that
validate `structuredContent` against it reject error bodies.

## Contract Impact

- `opengrok_read` errors carry `error_code` (and `suggestion` where present) in
  `StructuredContent`, as documented.
- `opengrok_read` no longer advertises `outputSchema`; success payloads are
  unchanged.

## Verification

Unit, through an in-memory MCP client session:

- Every compact tool returns `error_code: INVALID_CURSOR` for a bad cursor
  (`opengrok_read` failed this before the change).
- `opengrok_read` returns `error_code: FILE_NOT_FOUND` for a 404.
- A successful `opengrok_read` still returns a structured file window.

Live, against a real OpenGrok instance:

- A missing file returns `error_code: FILE_NOT_FOUND`; a successful read is
  unchanged; `tools/list` shows no `outputSchema` for `opengrok_read`.

## Out of Scope

- Full-surface tools: they declare concrete output types with output schemas
  and lose the error body the same way. Fixing them means either dropping their
  output schemas or making error results schema-compatible — a larger contract
  decision.
