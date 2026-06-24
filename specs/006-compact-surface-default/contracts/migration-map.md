# Contract: Migration Map

**Feature**: 006-compact-surface-default | **Date**: 2026-06-24

Authoritative mapping for the migration note (FR-011). Covers the default change,
the prior-compact → new mapping, and the full → compact mapping. The **full surface
is unchanged**; this table is for agents/configs moving onto compact.

## Default change

| Before | After | Restore path |
|---|---|---|
| no `OPENGROK_MCP_TOOL_SURFACE` set → **full** | no var set → **compact** | set `OPENGROK_MCP_TOOL_SURFACE=full` |

`full` and `gateway` remain selectable and behave exactly as before.

## Prior compact → new compact

| Prior compact call | New compact call |
|---|---|
| `opengrok_projects` (list only) | `opengrok_projects` op=`list` |
| `opengrok_search` op=`code` | `opengrok_search` op=`code` (unchanged) |
| `opengrok_search` op=`definitions` | `opengrok_symbols` op=`definitions` *(moved)* |
| `opengrok_search` op=`references` | `opengrok_symbols` op=`references` *(moved)* |
| `opengrok_symbols` op=`list` | `opengrok_symbols` op=`list` (unchanged) |
| `opengrok_symbols` op=`implementations` | `opengrok_symbols` op=`implementations` |
| `opengrok_symbols` op=`cross_project_references` | `opengrok_symbols` op=`cross_project` *(renamed)* |
| `opengrok_read` op=`file`/`context` | `opengrok_read` op=`file`/`context` (unchanged) |
| `opengrok_compound` op=`search_and_read` | `opengrok_search` op=`read` *(tool removed; folded)* |
| `opengrok_compound` op=`find_symbol_and_references` | `opengrok_symbols` op=`find` *(tool removed; folded)* |
| `opengrok_memory` op=`…` | **removed from compact** — memory is full-only (pending sunset, FR-014) |
| envelope `{operation, payload:{…}}` | **flattened** `{operation, …fields}` *(payload wrapper removed)* |

## Full → new compact

| Full tool | New compact call |
|---|---|
| `list_projects` | `opengrok_projects` op=`list` |
| `list_files` | `opengrok_projects` op=`files` *(new in compact)* |
| `get_project_overview` | `opengrok_projects` op=`overview` *(new in compact)* |
| `search_code` | `opengrok_search` op=`code` |
| `search_and_read` | `opengrok_search` op=`read` |
| `search_symbol_definitions` | `opengrok_symbols` op=`definitions` |
| `search_symbol_references` | `opengrok_symbols` op=`references` |
| `find_symbol_and_references` | `opengrok_symbols` op=`find` |
| `search_implementations` | `opengrok_symbols` op=`implementations` |
| `search_cross_project_references` | `opengrok_symbols` op=`cross_project` |
| `list_symbols` | `opengrok_symbols` op=`list` |
| `read_file` | `opengrok_read` op=`file` |
| `get_file_context` | `opengrok_read` op=`context` |
| `memory_*` | **not in compact** — remains full-only (pending sunset, FR-014) |

## Coverage check (FR-008 / SC-003)

Every full tool above maps to a compact target **except `memory_*`**, which is the
single deliberate exception (full-only, pending sunset, FR-014). Relative to *prior
compact*, the changes are: add `opengrok_projects.files` and `.overview` (close the two
known gaps) and **remove the memory tool**. No code-intelligence capability is reachable
only on full.
