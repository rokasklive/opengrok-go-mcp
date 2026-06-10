# Research: Split MCP Server Monolith

**Feature**: `003-split-mcp-server` | **Date**: 2026-06-10

## R1 — Package boundary: single package vs subpackages

**Decision**: Keep one `internal/mcpserver` package; split into multiple source files by concern.
Do not introduce `mcpserver/search`, `mcpserver/register`, etc.

**Rationale**:
- Existing pattern already uses sibling files (`pagination.go`, `warnings.go`, `coerce.go`).
- `Service` methods, unexported helpers, and registration closures share many types in `types.go`;
  subpackages would either export internals or duplicate types.
- `cmd/opengrok-go-mcp` only imports `NewMCPServer`, `Backend`, and `NewCachingBackend` — no
  caller API expansion needed.
- Subpackages increase risk of import cycles between search ↔ results ↔ registration.

**Alternatives considered**:
- `internal/mcpserver/handlers/search` subpackage — rejected; forces exported handler structs or
  awkward `internal` test-only imports.
- `internal/mcptools` new top-level package — rejected; splits MCP wiring from service logic
  without clear boundary benefit.

---

## R2 — File grouping (tool families)

**Decision**: One primary file per tool family plus shared infrastructure files:

| File | Responsibility |
|------|----------------|
| `service.go` | `Service`, `NewService`, `Backend`, `Error`, `IsCode` |
| `register.go` | `NewMCPServer`, surface switch, middleware wiring |
| `register_full.go` | `registerFullTools` |
| `register_compact.go` | `registerCompactTools`, `compactInputSchema` |
| `register_gateway.go` | `buildGatewayRegistry`, `registerGatewayTools`, `gatewayOperation` |
| `projects.go` | `ListProjects`, `ListFiles`, `GetProjectOverview`, project resolution helpers |
| `search_handlers.go` | Public search tool entrypoints (`SearchCode`, symbol searches, implementations) |
| `search_core.go` | `search()`, `searchRequest`, result assembly, sort/max-hits, empty output |
| `results.go` | `results()`, expansion, `maybeExpandResults`, `compactResults` |
| `symbols.go` | `ListSymbols`, symbol-specific validation |
| `filecontext.go` | `GetFileContext`, windowed/paged context, line extraction |
| `compound.go` | `SearchAndRead`, `FindSymbolAndReferences` |
| `compact.go` | Compact wrapper handlers and operation lists |
| `memory_handlers.go` | Direct memory tool methods + `CompactMemory` |
| `resources.go` | MCP resources and URI parsers |
| `helpers.go` | Shared validation (`validateResponseMode`, cursor, citation, budget, page size) |

Existing files stay: `types.go`, `pagination.go`, `warnings.go`, `coerce.go`, `query.go`,
`memory_bank.go`, `cache_backend.go`.

**Rationale**: Matches spec tool families; keeps registration thin; puts shared search pipeline in
`search_core.go` + `results.go` to avoid duplication across search entrypoints.

**Alternatives considered**:
- One file per MCP tool name — rejected; duplicates shared search pipeline and bloats file count.
- Keep all handlers in `server.go` until one mega-move — rejected; blocks incremental review.

---

## R3 — Incremental delivery order

**Decision**: Move code in dependency-safe slices; each slice is a mergeable PR with green
`go test ./...`:

1. **Slice A** — Extract `service.go` + `register*.go` + `resources.go` (registration and
   wiring only; handlers still called on `Service` in remaining `server.go`).
2. **Slice B** — Extract `helpers.go` + `projects.go` (low coupling to search).
3. **Slice C** — Extract `search_core.go` + `results.go` + `search_handlers.go`.
4. **Slice D** — Extract `symbols.go`, `filecontext.go`, `compound.go`.
5. **Slice E** — Extract `compact.go` + `memory_handlers.go`.
6. **Slice F** — Delete or shrink `server.go` to near-empty; rename if only stale re-exports
   remain.
7. **Slice G** — Split `server_test.go` into `projects_test.go`, `search_test.go`,
   `register_test.go`, `compact_test.go`, `gateway_test.go`, `resources_test.go` (and keep a
   thin integration file if needed).

**Rationale**: Registration extraction is mostly moves with compile-time verification; search
core is the highest coupling hub and moves after helpers exist in dedicated files; test split
last avoids chasing failures across moved code and tests simultaneously.

**Alternatives considered**:
- Big-bang single PR — rejected; violates FR-011 and reviewability goals.
- Test split first — rejected; 2700-line `server_test.go` references handlers still in `server.go`.

---

## R4 — Behavioral equivalence verification

**Decision**: No intentional changes to `server_test.go` expectations during handler moves.
Add optional **snapshot guard** only if a slice introduces risk:

- Before Slice C, capture tool output JSON for a fixed `fakeBackend` fixture (one test per
  surface) into testdata; compare after move.
- Primary gate remains `go test ./internal/mcpserver/` and `go test ./...`.

**Rationale**: Existing tests already encode MCP contract behavior; refactor should be pure moves.
Golden files add maintenance cost — use only if a slice mixes logic changes.

**Alternatives considered**:
- Require golden files for every tool — rejected; overkill for move-only refactor.
- Manual agent comparison only — rejected; not repeatable in CI.

---

## R5 — MCP SDK registration patterns

**Decision**: Keep `addTool` wrapper in `coerce.go`; registration functions receive
`*mcp.Server`, `*scalarCoercer`, `*Service`, `config.Config`. Handlers remain typed
`mcp.ToolHandlerFor[In, Out]` closures or methods — no custom router abstraction.

**Rationale**: Aligns with `github.com/modelcontextprotocol/go-sdk` patterns; `addTool` already
unifies coercion + registration; avoids new DI frameworks (spec out of scope).

**Alternatives considered**:
- Reflective tool registry — rejected; harder to test and non-idiomatic for Go MCP servers.
- Move registration to `cmd` — rejected; breaks package encapsulation.

---

## R6 — Documentation for contributors

**Decision**: Add short `internal/mcpserver/README.md` (contributor-only) mapping tool families
to files and test files. Do not change agent-facing `docs/tool-contracts.md`.

**Rationale**: Supports SC-002 (locate module in one minute); AGENTS.md repository map can
reference the layout briefly.

**Alternatives considered**:
- Document only in plan/tasks — rejected; plan is feature-scoped; README persists for maintainers.
