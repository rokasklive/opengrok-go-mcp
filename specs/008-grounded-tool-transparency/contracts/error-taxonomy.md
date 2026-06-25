# Contract: Structured-Error Taxonomy

Replaces the single opaque `-32602 oneOf: did not validate` (which today collapses four
distinct causes). Every failed call returns a `ToolErrorBody`
(`{error_code, message, suggestion, details?}`) with `IsError=true` — never a bare transport
error for the four validation classes. `suggestion` is a **new** field on the body.

## Validation classes (caught in pre-validation middleware — `detection_point=pre-validation-middleware`)

| error_code | cause_class | names | suggestion (pattern) | test_ref |
|------------|-------------|-------|----------------------|----------|
| `UNKNOWN_OPERATION` | operation not valid / not enabled for this tool | the operation + this tool | "operation `{op}` is not valid for `{tool}`; enabled operations: {list}{; did you mean `{other_tool}`?}" | `TestErrUnknownOperation` |
| `MISSING_REQUIRED_FIELD` | required field for the operation absent | the field + operation | "operation `{op}` requires `{field}`" | `TestErrMissingRequired` |
| `INVALID_FIELD_TYPE` | field present but wrong type (post-coercion) | the field + expected type | "`{field}` must be {type}; got {got}" | `TestErrInvalidType` |
| `UNKNOWN_FIELD` | field not in the operation's schema | the offending field | "`{field}` is not a recognized parameter for `{op}`; did you mean `{closest}`?" | `TestErrUnknownField` |

The middleware checks in this order (operation → required → type → unknown) and returns the
**first** matching class, so the agent gets one specific cause, not a union.

## Upstream / query classes

| error_code | cause_class | names | suggestion (pattern) | detection_point | test_ref |
|------------|-------------|-------|----------------------|-----------------|----------|
| `QUERY_PARSER_FAILED` | OpenGrok rejected the query (HTTP 400 / parse error) | the query | "OpenGrok could not parse `{query}`. Wrap regex in `/…/`; quote phrases; `*`/`?` are wildcards, not regex; see opengrok://capabilities" | upstream | `TestErrQueryParser` |
| `FILE_NOT_FOUND` *(existing)* | 404 from upstream | the path | (existing message) | upstream | existing |
| `UPSTREAM_HTTP_ERROR` *(existing)* | other non-2xx | path + status | (existing) | upstream | existing |
| `UNAUTHORIZED` *(existing 401/403 mapping)* | auth/permission | path + status | "check API credentials and project permissions" | upstream | existing |

## Response-state legibility (FR-011)

| state | shape signal | must NOT be confused with |
|-------|--------------|---------------------------|
| `success` | `IsError=false`, results present | — |
| `empty` | `IsError=false`, `total_hits=0`, `results=[]` | `error` (a zero-result search is success-with-no-hits) |
| `truncated` | success + `truncated=true`/`next_cursor` | `empty` |
| `warning` | success + `warnings[]`/legacy `warning` | `error` |
| `unauthorized` | `IsError=true`, `UNAUTHORIZED` | generic `UPSTREAM_HTTP_ERROR` |
| `error` | `IsError=true` + a code above | `empty` |

**Contract invariants**
- The four validation classes MUST surface as a `ToolErrorBody`, not a raw `-32602`
  (un-shadows the existing `unknownOperationError`; R1).
- The `oneOf` discriminated schema is retained; the middleware intercepts before the SDK
  validator so the typed schema (006) is preserved while errors become specific.
- `QUERY_PARSER_FAILED` is distinct from `UPSTREAM_HTTP_ERROR` (the recon gap — Lucene 400s
  previously fell through to the generic mapping with no corrective hint).
- Each error class is covered by a contract test that asserts code + that the offending
  operation/field/query is named + that `suggestion` is present (P-III).
