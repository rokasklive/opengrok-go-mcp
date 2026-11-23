# Agent Guide

`opengrok-go-mcp` exposes OpenGrok through MCP so agents can search indexed
code, read files, follow symbols, and cite sources. Principles live in
[`.specify/memory/constitution.md`](.specify/memory/constitution.md).

## Layout

- `cmd/opengrok-go-mcp/`: entrypoint, startup capability probing
- `internal/config/`: env, flags, defaults, validation
- `internal/mcpserver/`: tools, schemas, pagination, warnings ([file map](internal/mcpserver/README.md))
- `internal/opengrok/`: OpenGrok API and web/raw-file client
- `evals/`: stdio subprocess contract tests and token benchmark
- `docs/`: [configuration](docs/configuration.md), [tool contracts](docs/tool-contracts.md), [limitations](docs/limitations.md), [agent usage](docs/agent-usage-patterns.md)

## Rules

- Tool names, inputs, outputs, `warnings[]` codes, `error_code`, cursors,
  `citation.url`, resources, and env vars are public contract. Changing one
  needs a test and a doc update (`docs/tool-contracts.md` or
  `docs/configuration.md`).
- Write descriptions, schemas, and warnings for a cold agent.
- Keep full, compact (default), and gateway (experimental) surfaces consistent.
- OpenGrok is full-text plus ctags, not an AST or call graph. Surface
  best-effort, truncated, or page-local results with warnings; update
  `docs/limitations.md`.
- Anything that grows response size, call count, or auto-fetch needs a limit,
  a default, and a warning.
- Secrets come from env vars, never flags or logs. HTTP mode has no inbound
  auth; keep it loopback by default. Memory tools are full-surface, stdio only.
- Do not infer an OpenGrok project from the local repository name.

## Spec Kit (lean)

For public contract or behavior changes, copy the templates in
`.specify/templates/` into `specs/NNN-name/`: `spec.md` for narrow fixes, plus
`plan.md` and `tasks.md` for new capabilities or breaking changes. Skip it for
edits with no public behavior change.

## Verify

```bash
gofmt -l .
go test -race -count=1 ./...
git diff --check
```

For non-trivial agent-facing changes, give a fresh subagent a realistic task
with minimal context and fix what confuses it.
