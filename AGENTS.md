# Agent Guide

## Project Purpose

`opengrok-go-mcp` exposes OpenGrok through MCP so agents can search indexed
code, read source files, follow symbols, and answer with source citations.

<!-- SPECKIT START -->
**Active feature plan**: [specs/008-grounded-tool-transparency/plan.md](specs/008-grounded-tool-transparency/plan.md) — grounded, test-backed tool transparency: honest descriptions (text+ctags not AST; supported/unsupported Lucene syntax via `help.jsp` ground truth) driven by a machine-enforced claim⇔test registry, de-opaqued validation errors (structured `ToolErrorBody` + `suggestion`, not raw `oneOf`), `OPENGROK_MCP_DIAGNOSTICS` gating (default off), and removal of schema slimming (no surface hides field docs).
<!-- SPECKIT END -->

## Repository Map

- `cmd/opengrok-go-mcp/`: server entrypoint and startup capability probing
- `internal/config/`: environment, flags, defaults, validation
- `internal/mcpserver/`: MCP tools, schemas, pagination, warnings, memory (see `internal/mcpserver/README.md` for file layout)
- `evals/`: stdio subprocess eval harness (contract checks + token economy benchmark)
- `internal/opengrok/`: OpenGrok API and web/raw-file client
- `docs/`: limitations, agent usage patterns, design notes
- `.specify/`: Spec Kit constitution, templates, scripts, extension hooks

## Core MCP Contract Rules

- Treat tool names, input fields, output fields, warnings (`warnings[]` codes and
  legacy `warning` string), errors (`error_code`), pagination, cursor
  semantics, citations, resources, and environment variables as public contract.
- Write tool names, descriptions, schemas, warnings, defaults, and examples for
  a cold agent seeing the server for the first time.
- Keep behavior consistent across full, compact, and gateway surfaces unless a
  spec explicitly says otherwise.
- Preserve `citation.url` for source-backed answers.
- New or changed response-size, tool-call, or auto-fetch behavior needs limits,
  defaults, and warnings.
- For the detailed field-by-field contract (inputs, outputs, errors, warnings,
  pagination, citations, truncation, capability gates, compatibility,
  experimental fields), see `docs/tool-contracts.md`.

## OpenGrok Semantics And Limitations

- OpenGrok is full-text search plus ctags definitions, not an AST/call-graph
  engine.
- `search_implementations`, cross-project attribution, kind filtering, sorting,
  and project overview data can be best-effort, heuristic, or page-local.
- Surface uncertainty with warnings; do not claim exhaustive semantic knowledge
  unless the implementation proves it.
- For structural certainty, use OpenGrok to find candidate files and then verify
  with language-aware tooling.

## Tool Surfaces

- Default: `compact` surface with four consolidated tools (`opengrok_projects`,
  `opengrok_search`, `opengrok_symbols`, `opengrok_read`). Set
  `OPENGROK_MCP_TOOL_SURFACE=full` for fine-grained tools such as `search_code`,
  `read_file`, `get_file_context`, `list_symbols`, and symbol search tools.
- `gateway` surface is experimental; use `opengrok_discover` before
  `opengrok_call`.
- Tools are capability-gated at startup. If a tool is missing, assume the server
  could not verify the backing OpenGrok capability.

## Compatibility Rules

- Keep existing public defaults stable unless a spec and migration note justify
  a change.
- Experimental features must be labeled in tool descriptions, docs, and config
  names.
- Do not silently alter stable tool behavior from an experimental path.

## Security Rules

- Secrets belong in environment variables, not CLI flags or logs.
- HTTP mode has no built-in inbound client auth; keep loopback/default trusted
  network assumptions unless a spec changes that.
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY` is only for controlled internal hosts.
- Memory tools are **full-surface only** (not registered on compact). Memory is
  process-scoped, ephemeral, and disabled over HTTP.
- For vulnerability disclosure, see `SECURITY.md` (private reporting via GitHub
  Security Advisories).

## Spec Kit Workflow

Use GitHub Spec Kit for meaningful behavior changes: new MCP tools, schema
changes, config/env changes, transport/security changes, OpenGrok query
behavior, compatibility changes, or anything affecting agent reliability/token
usage.

Start from `.specify/memory/constitution.md`. Feature work generally produces:

- `specs/FEATURE/spec.md`
- `specs/FEATURE/plan.md`
- `specs/FEATURE/tasks.md`

Small docs fixes, typo fixes, formatting-only changes, dependency metadata
updates, and trivial test-only cleanups may skip the full workflow.

## Testing Commands

- Format Go changes: `gofmt -w <files>`
- Targeted package test: `go test ./internal/<package>/`
- Eval harness (stdio subprocess): `go test ./evals/ -count=1`
- Token economy benchmark: `go test ./evals/ -run TestTokenBenchmark -count=1`
- Full verification: `go test ./...`
- Documentation whitespace check: `git diff --check`
- For non-trivial agent-facing changes, dispatch a fresh lightweight or mid-tier
  subagent where available, or use a fresh-session simulation otherwise. Give it
  a realistic task and minimal context, then capture first-use findings on
  descriptions, schemas, warnings, defaults, and examples.

## Documentation Update Rules

- Update `README.md` for human-facing setup, configuration, or behavior changes.
- Update `docs/configuration.md` when environment variables or defaults change.
- Update `docs/limitations.md` when behavior is best-effort, heuristic,
  truncated, page-local, or security-sensitive.
- Update `docs/agent-usage-patterns.md` when agent workflow guidance changes.
- Keep examples concise and avoid duplicating long tool schemas in multiple
  places.
- Update `docs/tool-contracts.md` when MCP input/output/error/warning/pagination
  contract semantics change.
- Update `docs/agent-ux.md` when guidance on writing descriptions/warnings
  changes; `docs/review-checklist.md` when the review rubric changes.
- Update `docs/release-process.md` and `CHANGELOG.md` for releases; `SECURITY.md`
  for disclosure-policy changes.
- See `docs/README.md` for the documentation source-of-truth map.

## Common Mistakes To Avoid

- Do not infer an OpenGrok project from the local repository name.
- Do not paginate broad result sets blindly; narrow with path, kind, file type,
  or query.
- Do not claim implementation/call-graph certainty from text matches alone.
- Do not bypass warnings in responses.
- Do not introduce new public fields without tests and docs.
- Do not assume future agents know project context that is absent from tool
  descriptions or schemas.

## Review Checklist

See `docs/review-checklist.md` for the full review rubric (also usable as a
review-agent prompt).

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, invoke the `skill` tool with `skill: "graphify"` before doing anything else.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
