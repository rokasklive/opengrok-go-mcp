# Feature Specification: Escape Raw-Fallback Paths And Reject `..` Segments

**Feature Branch**: `fix/file-path-escaping`

**Created**: 2026-10-06

**Status**: Implemented

**Input**: Review of v0.6.2 `FileContent` against OpenGrok path handling.

## Context & Motivation

When `/api/v1/file/content` fails, `FileContent` falls back to the web `/raw`
endpoint. `rawFilePath` concatenated project and path into the URL without
escaping, so the fallback broke for legal file names:

```
docs/what?.md     → server sees path /raw/proj/docs/what, query ".md"
src/C#/a.cs       → truncated to /raw/proj/src/C (fragment never sent)
a/100%done.txt    → request fails to build ("invalid URL escape")
```

Citation links (`internal/links`) already escape each segment, so `raw_url`
citations worked while the fetch itself did not.

Neither the client nor the MCP layer rejected `..` segments, so a `file_path`
like `../other-project/file` was sent as-is and left to server-side
normalisation — outside the project the caller (and the project allowlist)
named.

## Decisions

**D1 — escape every segment of the raw path**, matching `links.File`.

**D2 — reject `..` segments in the client, before any request.** No indexed
OpenGrok path needs a `..` segment, and checking in `FileContent` covers every
caller (read, context, search-and-read, find, resources). Names that merely
contain dots (`..hidden`, `a..b.txt`) are allowed.

**D3 — surface a dedicated, structured error.** `opengrok.ErrInvalidFilePath`
maps to a new `INVALID_FILE_PATH` error code with a `suggestion` ("pass
file_path exactly as a search result returned it"), rather than a generic
failure or a misleading `FILE_NOT_FOUND`.

## Contract Impact

- New error code `INVALID_FILE_PATH` (documented in `tool-contracts.md`).
- Paths with `..` segments now fail fast instead of reaching OpenGrok.
- Raw-fallback reads now work for names containing `#`, `?`, `%`, spaces.

## Verification

Unit:

- `..` paths (`../x`, `src/../../x`, `src/..`) are rejected without any HTTP
  request; dotted names are accepted.
- Raw fallback requests the exact decoded path for `what?.md`, `C#/a.cs`,
  `100%done.txt`, `dir with space/x.txt`, with no stray query string.
- `get_file_context` maps the error to `INVALID_FILE_PATH` with a suggestion.

Live, against a real OpenGrok instance:

- `../genesis-lts-24.0/pom.xml` is refused with the new message and suggestion.

## Out of Scope

- Validating `path` for `list_files` and other directory-oriented inputs.
