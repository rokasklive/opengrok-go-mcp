# Data Model: Agent Ergonomics Hardening

**Feature**: 007-agent-ergonomics-hardening | **Date**: 2026-06-24

Entities introduced or extended by this feature. Field names match planned JSON
tags for MCP outputs unless noted.

---

## AgentProfile (configuration)

Operator-selected bundle; stored on `config.Config`.

| Field | Type | Values | Notes |
|-------|------|--------|-------|
| `AgentProfile` | `string` | `""`, `economy`, `rich` | `""` → `economy` |

**Resolved defaults** (used when per-call fields omitted):

| Resolved setting | Rich | Economy |
|------------------|------|---------|
| `AutoExpandContext` | `true` | `false` |
| `ResponseModeDefault` | `full` | `compact` |
| `IncludeLinksDefault` | `true` | `false` |

Env: `OPENGROK_MCP_AGENT_PROFILE`.

---

## CapabilityReport (startup snapshot → manifest)

Populated once at startup in `cmd/opengrok-go-mcp/main.go`; read-only for process
lifetime. Not exposed on every tool call — only via manifest resource.

| Field | Type | Description |
|-------|------|-------------|
| `ToolSurface` | `string` | Active surface |
| `AgentProfile` | `string` | Active profile |
| `Tools` | `[]ToolCapability` | Registered tools + operations |
| `Gated` | `[]GatedCapability` | Probed but disabled |
| `ProjectCatalog` | `ProjectCatalogMeta` | Discovery snapshot |

### ToolCapability

| Field | Type |
|-------|------|
| `Name` | `string` |
| `Operations` | `[]string` |
| `Summary` | `string` | One line from compact blurbs |

### GatedCapability

| Field | Type |
|-------|------|
| `Capability` | `string` | e.g. `SearchSymbolReferences` |
| `ReasonCode` | `string` | e.g. `PROBE_UNAUTHORIZED`, `PROBE_UNSUPPORTED` |
| `Remediation` | `string` | Actionable; env var names only |

### ProjectCatalogMeta

| Field | Type |
|-------|------|
| `Source` | `string` | `configured`, `api`, `scraped`, `none` |
| `IsSnapshot` | `bool` | Always `true` in v1 |
| `ProjectCount` | `int` |
| `DefaultProject` | `string` |

---

## CapabilityManifest (MCP resource body)

URI: `opengrok://capabilities`. Mirrors `CapabilityReport` as JSON for agents.

---

## ListProjectsOutput (extended)

Additive fields on existing type:

| Field | Type | Always present |
|-------|------|----------------|
| `catalog_source` | `string` | yes |
| `catalog_is_snapshot` | `bool` | yes (`true`) |

Existing: `projects`, `total_projects`, `next_cursor`.

---

## ListSymbolsOutput (extended)

When `kind` filter active on input:

| Field | Type |
|-------|------|
| `kind_filter_active` | `bool` (`true`) |
| `kind_matches_on_page` | `int` |
| `total_hits_scope` | `string` (`pre_kind_filter`) |

When kind filter absent: omit `kind_filter_active` and `total_hits_scope`; do not
emit misleading zeros.

Existing: `symbols`, embedded `Pagination`, `WarningFields`.

---

## TrajectoryCase (eval harness)

File: `evals/testdata/trajectory/<id>.json`

| Field | Type | Required |
|-------|------|----------|
| `id` | `string` | yes |
| `description` | `string` | yes |
| `surface` | `string` | yes (`compact` default) |
| `env` | `object` | optional profile overrides |
| `steps` | `[]TrajectoryStep` | yes |
| `graders` | `[]TrajectoryGrader` | yes |

### TrajectoryStep

| Field | Type |
|-------|------|
| `tool` | `string` |
| `input` | `object` |
| `expect_no_error` | `bool` |

### TrajectoryGrader

| Field | Type | Grader kinds |
|-------|------|--------------|
| `type` | `string` | `tool_sequence`, `warning_code`, `citation_present`, `field_present`, `field_eq`, `description_cuj` |
| `step_index` | `int` | optional |
| `field` | `string` | dotted path |
| `value` | `any` | for `field_eq`, `warning_code` |
| `tools` | `[]string` | for `tool_sequence` |
| `task` | `string` | for `description_cuj` |

---

## ListToolsBaseline (eval policy)

Stored in `evals/baselines/token_report.json`:

| Field | Type |
|-------|------|
| `compact_list_tools_ceiling_bytes` | `int` |

---

## Relationships

```text
Config.AgentProfile ──resolves──► Service default helpers ──► tool responses
Config.CapabilityReport ──serializes──► opengrok://capabilities
Config.ProjectSource ──copies──► ListProjectsOutput.catalog_source
ListSymbolsInput.Kind ──triggers──► ListSymbolsOutput kind metadata fields
TrajectoryCase ──replays──► Harness steps ──graded──► TrajectoryGrader
```

---

## Validation rules

- Invalid `OPENGROK_MCP_AGENT_PROFILE` → startup error (fail fast), same pattern as
  invalid `OPENGROK_MCP_TOOL_SURFACE`.
- Manifest MUST NOT include secret values or auth header contents.
- `kind_matches_on_page` MUST equal `len(symbols)` when kind filter active.
- `total_hits_scope` MUST only appear when `kind_filter_active` is true.
