# Behavior Equivalence Contract: Split MCP Server Monolith

**Feature**: `003-split-mcp-server` | **Date**: 2026-06-10

This document defines the **non-regression contract** for the refactor. MCP tool input/output
schemas, operator configuration, and agent workflows are unchanged. Any deviation is a defect.

## Scope of equivalence

| Area | Must remain identical |
|------|----------------------|
| Tool names | All registered tools per surface and capability gate |
| Tool descriptions | Exact strings in registration (no drive-by edits) |
| Input schemas | JSON Schema from struct tags + compact `InputSchema` maps |
| Output shapes | All response fields, optional vs required, warning arrays |
| Error codes | `Error.Code` values and `IsCode` behavior |
| Pagination | `next_cursor`, page sizes, `total_*` fields |
| Citations | `citation.url` construction via `links.Builder` |
| Warnings | Text, ordering where tests assert order, warn thresholds |
| Capability gating | Tool omitted when capability false at startup |
| Gateway manifest | Operation ids, descriptions, input hints |
| Resources | URI patterns and JSON payloads |
| Memory | stdio enabled; HTTP disabled for memory tools |
| Scalar coercion | String-encoded booleans via `scalarCoercer` middleware |

## Surfaces

Equivalence must hold for each configured `ToolSurface`:

- `full` (default)
- `compact`
- `gateway`

Cross-surface: compact and gateway operations must invoke the same `Service` methods as full
tools for equivalent operations.

## Verification methods

### CI gate (required)

```bash
go test ./...
```

All existing tests pass without expectation changes except:

- Test file moves (`server_test.go` → split files) where test names may change but assertions
  must not weaken.

### Package gate (required per slice)

```bash
go test ./internal/mcpserver/
```

### Optional golden guard (slice C+)

When splitting search pipeline, add tests that:

1. Fix `fakeBackend` responses.
2. Call representative tools on each surface.
3. Compare JSON output to `testdata/*_golden.json` captured pre-split.

Failure indicates behavioral drift, not acceptable refactor noise.

### Agent UX gate (recommended before merge)

Fresh-session task (no code access): *"List projects, search for a symbol, read the definition
file with citation."* Outcomes must match pre-refactor baseline on the same backend fixture.

## Explicit non-goals (not defects)

- File names, line numbers, or internal function names
- Test function names after test file split
- Contributor README for package layout
- Import order or `gofmt` formatting-only diffs

## Exported package API

`cmd/opengrok-go-mcp` must continue to use:

- `mcpserver.NewMCPServer(cfg, backend, version)`
- `mcpserver.Backend` interface
- `mcpserver.NewCachingBackend(...)` when caching enabled

No new required parameters or constructor side effects.

## Regression response

If equivalence fails:

1. Treat as blocking for merge.
2. Either revert the slice or fix the drift before proceeding to the next slice.
3. Do not "update tests to match" without proving the old behavior was incorrect.
