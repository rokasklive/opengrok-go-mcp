# Migration: compact as default

As of feature **006-compact-surface-default**, the shipped default tool surface
is **`compact`**. The **`full`** surface is unchanged and remains selectable.

## Restore the previous default

```bash
export OPENGROK_MCP_TOOL_SURFACE=full
```

## What changed in compact

| Before (prior compact) | After (new compact) |
|---|---|
| `{operation, payload:{…}}` envelope | Flattened `{operation, …fields}` |
| `opengrok_search` `definitions` / `references` | `opengrok_symbols` `definitions` / `references` |
| `opengrok_compound` | `opengrok_search` `read` or `opengrok_symbols` `find` |
| `opengrok_memory` | **Removed from compact** — use full surface memory tools |
| No `list_files` / overview in compact | `opengrok_projects` `files` / `overview` |
| `cross_project_references` | `cross_project` |

## Full surface mapping

See [specs/006-compact-surface-default/contracts/migration-map.md](../specs/006-compact-surface-default/contracts/migration-map.md)
for the complete full → compact table.

## Memory

Process-scoped memory remains on the **full** surface only (stdio). Compact
omits memory intentionally pending a separate sunset decision.
