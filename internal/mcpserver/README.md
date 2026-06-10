# internal/mcpserver

MCP tool registration, handlers, and response shaping for OpenGrok. Agent-facing tool
contracts live here; see `docs/tool-contracts.md` for the public MCP surface.

## File map

| File | Responsibility |
|------|----------------|
| `service.go` | `Service`, `Backend`, errors, `NewService` |
| `register.go` | `NewMCPServer`, surface switch |
| `register_full.go` | Full-surface tool registration |
| `register_compact.go` | Compact wrapper registration |
| `register_gateway.go` | Gateway registry and registration |
| `resources.go` | MCP resources and URI parsing |
| `projects.go` | `list_projects`, `list_files`, project overview |
| `search_handlers.go` | Search tool entrypoints |
| `search_core.go` | Shared `search()` pipeline |
| `results.go` | Result assembly, expansion, sort/max-hits |
| `symbols.go` | `list_symbols` and symbol validation |
| `filecontext.go` | `read_file` / `get_file_context` |
| `compound.go` | `search_and_read`, `find_symbol_and_references` |
| `compact.go` | Compact wrapper handlers |
| `memory_handlers.go` | Memory tool methods |
| `helpers.go` | Pagination, project resolution, citations, cursors |
| `types.go` | Input/output types |
| `coerce.go` | Scalar coercion middleware, `addTool` |
| `pagination.go`, `warnings.go`, `query.go` | Shared contract helpers |
| `memory_bank.go`, `cache_backend.go` | Memory and caching |

## Tests

| File | Focus |
|------|--------|
| `testutil_test.go` | `fakeBackend`, MCP test helpers |
| `register_test.go` | Tool registration and schemas |
| `gateway_test.go` | Gateway surface |
| `projects_test.go` | Project/file listing |
| `search_test.go` | Search, expansion, compound symbol flows |
| `symbols_test.go` | Symbol listing |
| `filecontext_test.go` | File read and context |
| `compact_test.go` | Compact wrappers |
| `resources_test.go` | MCP resources |

Run focused tests:

```bash
go test ./internal/mcpserver/ -run Search -count=1
go test ./internal/mcpserver/ -run 'ListProjects|ListFiles' -count=1
```
