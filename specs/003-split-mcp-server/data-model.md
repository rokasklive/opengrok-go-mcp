# Data Model: Split MCP Server Monolith

**Feature**: `003-split-mcp-server` | **Date**: 2026-06-10

This refactor does not introduce new MCP response fields or persisted state. The data model
describes **internal package structure** and relationships that must remain stable across the
split.

## Core types (unchanged semantics)

### Service

| Field | Type | Role |
|-------|------|------|
| `cfg` | `config.Config` | Capabilities, projects, budgets, surface mode |
| `backend` | `Backend` | OpenGrok operations (list, search, file content) |
| `links` | `links.Builder` | Display/raw URL construction |
| `memoryBank` | `*MemoryBank` | Process-scoped agent memory |

**Lifecycle**: Created by `NewService`; owned by `NewMCPServer` for process lifetime. File
layout changes must not alter field set or initialization order.

### Backend (interface)

| Method | Purpose |
|--------|---------|
| `ListProjects` | Project index |
| `ListFiles` | Directory listing |
| `Search` | OpenGrok search API |
| `FileContent` | Raw/paged file reads |
| `GetProjectOverview` | Project metadata |

Optional extension: `fileListMetadataBackend` for enriched list responses (type assertion in
`listFilesWithMetadata`).

### Error (service errors)

| Field | Type | Examples |
|-------|------|----------|
| `Code` | string | `INVALID_CURSOR`, `PROJECT_REQUIRED`, `UNKNOWN_OPERATION` |
| `Message` | string | Agent-visible text |

Must remain inspectable via `IsCode`; splitting must not change code strings.

## Tool family map

| Family | Service methods | Registration |
|--------|-----------------|--------------|
| Projects / files | `ListProjects`, `ListFiles`, `GetProjectOverview` | full + gateway ops |
| Search | `SearchCode`, symbol search methods, `search()` pipeline | full + compact + gateway |
| Symbols | `ListSymbols`, `SearchImplementations`, cross-project refs | full + compact + gateway |
| Read / context | `GetFileContext`, inline `read_file` handler | full + compact + gateway |
| Compound | `SearchAndRead`, `FindSymbolAndReferences` | full + compact compound |
| Memory | `Memory*` methods, `CompactMemory` | full + compact + gateway (gated) |
| Compact wrappers | `CompactSearch`, `CompactSymbols`, `CompactRead`, `CompactCompound` | compact surface only |
| Gateway | `GatewayDiscover`, `GatewayCall` via registry | gateway surface only |
| Resources | `projectsResource`, `projectResource`, `fileResource` | all surfaces with resources |

## Registration surface relationships

```text
NewMCPServer(cfg, backend, version)
  → NewService(cfg, backend)
  → mcp.NewServer(...)
  → switch cfg.ToolSurface:
        compact → registerCompactTools + registerResources
        gateway → registerGatewayTools + registerResources
        default   → registerFullTools + registerResources
  → coercer.middleware()
```

**Invariant**: Exactly one registration path per configured surface; no duplicate tool names.

### Gateway registry

```text
buildGatewayRegistry(service, cfg) → map[string]gatewayOperation
  gatewayOperation { Manifest, Call }
```

**Invariant**: Every manifest entry in `GatewayDiscover` must have a matching `Call` in the
map; capability flags must gate both discover and call consistently.

## Shared pipeline (search)

```text
Tool handler (search_handlers.go)
  → validate inputs / projects / cursor
  → search() (search_core.go)
      → backend.Search
      → results() + expansion (results.go)
      → warnings + pagination (warnings.go, pagination.go)
  → SearchOutput
```

**Invariant**: Single implementation path for search result shaping; compact/full/gateway must
not fork divergent copies.

## Test artifact relationships

| Source file | Test file (target) |
|-------------|-------------------|
| `projects.go` | `projects_test.go` |
| `search_handlers.go`, `search_core.go`, `results.go` | `search_test.go` |
| `symbols.go` | `symbols_test.go` |
| `filecontext.go` | `filecontext_test.go` |
| `compact.go` | `compact_test.go` |
| `register_gateway.go` | `gateway_test.go` |
| `register*.go` | `register_test.go` |
| `resources.go` | `resources_test.go` |
| Cross-surface integration | `integration_test.go` (optional thin file) |

## Validation rules (must not change)

- Project resolution: `resolveProjects`, `resolveSearchProjects`, `validateConfiguredProjects`
- Cursor: invalid cursor → `INVALID_CURSOR`
- Response mode: `full` | `compact` only
- Compact unknown operation → `UNKNOWN_OPERATION` with enabled op list
- Memory tools disabled over HTTP transport (enforced in registration, not Service)

## State transitions

None — refactor is structural only. `MemoryBank` state semantics unchanged.
