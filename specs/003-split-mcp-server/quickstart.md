# Quickstart: Split MCP Server Monolith (Contributors)

**Feature**: `003-split-mcp-server` | **Date**: 2026-06-10

## Who this is for

Maintainers implementing or reviewing the `internal/mcpserver` de-monolith refactor. Agent
operators and MCP clients see **no** setup or behavior changes.

## Before you start

```bash
git checkout 003-split-mcp-server
go test ./internal/mcpserver/   # baseline must be green
```

Current state: ~2700 lines in `server.go` and `server_test.go`; smaller files already exist for
pagination, warnings, coercion, query, memory bank, and cache backend.

## Target layout (after refactor)

```text
internal/mcpserver/
├── service.go              # Service, Backend, errors
├── register.go             # NewMCPServer
├── register_full.go
├── register_compact.go
├── register_gateway.go
├── projects.go
├── search_handlers.go
├── search_core.go
├── results.go
├── symbols.go
├── filecontext.go
├── compound.go
├── compact.go
├── memory_handlers.go
├── resources.go
├── helpers.go
├── types.go                # existing
├── pagination.go           # existing
├── warnings.go             # existing
├── coerce.go               # existing
├── query.go                # existing
├── memory_bank.go          # existing
├── cache_backend.go        # existing
├── README.md               # contributor map (new)
└── *_test.go               # split by concern
```

## Where to change what

| Task | Go to |
|------|--------|
| Add/fix project or file listing | `projects.go` + `projects_test.go` |
| Search modes, Lucene query handling | `search_handlers.go`, `search_core.go`, `query.go` |
| Result expansion, citations in results | `results.go`, `links` package |
| Symbol listing / implementations | `symbols.go` |
| `read_file` / `get_file_context` | `filecontext.go` |
| `search_and_read`, `find_symbol_and_references` | `compound.go` |
| Compact wrapper ops | `compact.go` |
| Gateway discover/call | `register_gateway.go` |
| New full-surface tool registration | `register_full.go` |
| MCP resources | `resources.go` |
| Shared pagination/warnings | `pagination.go`, `warnings.go` (do not duplicate) |

## Slice workflow

1. Pick the next slice from [plan.md](./plan.md) (A → G).
2. **Move only** — no behavior edits in the same commit as moves when possible.
3. Run `go test ./internal/mcpserver/` after each file move.
4. Run `go test ./...` before opening PR.
5. If tests fail, fix structure (missing symbols, wrong file) — not assertions.

## Verify equivalence

```bash
go test ./internal/mcpserver/ -count=1
go test ./...
```

Optional after search slice:

```bash
go test ./internal/mcpserver/ -run Golden -count=1
```

(if golden tests were added — see [behavior-equivalence-contract.md](./contracts/behavior-equivalence-contract.md)).

## What not to do

- Split into subpackages without a plan update and cycle analysis.
- Change tool descriptions or schemas "while you're here".
- Duplicate `search()` or `results()` logic into compact handlers.
- Leave duplicate registration paths in `server.go` after moving to `register_*.go`.

## After merge

Update `internal/mcpserver/README.md` if file map changes. Agent-facing docs unchanged unless
a bugfix ships alongside the refactor (out of scope for this feature).
