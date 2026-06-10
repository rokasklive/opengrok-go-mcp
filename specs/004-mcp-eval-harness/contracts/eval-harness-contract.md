# Eval Harness Contract: MCP Eval Harness

**Feature**: `004-mcp-eval-harness` | **Date**: 2026-06-10

This document defines the **maintainer-facing contract** for the eval harness. It does not
change the MCP tool contract exposed to agents.

## Entrypoints

| Command | Behavior |
|---------|----------|
| `go test ./evals/ -run TestEvalSuite -count=1` | Full hermetic suite; fails on any judged case failure |
| `go test ./evals/ -count=1` | Includes TestEvalSuite via package tests |
| `go test ./...` | Includes evals when package is part of module |

## Environment (hermetic default)

Set by harness `startBackend`, not by case JSON:

| Variable | Example | Purpose |
|----------|---------|---------|
| `OPENGROK_MCP_BASE_URL` | `http://127.0.0.1:PORT/api/v1` | Fake OpenGrok REST |
| `OPENGROK_MCP_WEB_BASE_URL` | `http://127.0.0.1:PORT/source` | Web/raw link derivation |
| `OPENGROK_MCP_PROJECTS` | `platform,infra` | Skip scrape; fixed allowlist |
| `OPENGROK_MCP_DEFAULT_PROJECT` | `platform` | Default project for tools |
| `OPENGROK_MCP_PROBE_FILE` | `platform/src/Engine.swift` | File probe for get_file_context gate |
| `OPENGROK_MCP_CURSOR_SECRET` | test-only secret | Cursor signing for pagination cases |
| `OPENGROK_MCP_TRANSPORT` | `stdio` (default) | Subprocess transport |

Optional maintainer overrides (not default CI):

| Variable | Purpose |
|----------|---------|
| `OPENGROK_MCP_BASE_URL` | Live OpenGrok API base |
| `OPENGROK_MCP_API_TOKEN` | Auth for live instance |
| `OPENGROK_MCP_LIVE_EVAL=1` | Skip httptest; use env URL (if implemented) |

## Testdata contract

- Location: `evals/testdata/*.json` — array of `EvalCase` objects.
- OpenGrok fixtures: `evals/testdata/opengrok/*.json` + `evals/testdata/manifest.json`.
- Schema: see [testdata-format.md](../../../.agents/skills/mcp-eval-harness/references/testdata-format.md) and [data-model.md](../data-model.md).

## Skip vs fail

| Condition | Outcome |
|-----------|---------|
| Tool not in `ListTools` after startup | **Skipped** (Coverage@K) |
| `CallTool` transport error | **Failed** |
| `IsError` true on result | **Failed** (unless check suite expects error — v1 uses `no_error`) |
| Result check fails | **Failed** |
| Latency > budget | **Failed** (latency check) |

## Report contract

After suite completion, harness writes:

- `evals/report.md` — summary table per tool, Coverage@K, latency percentiles, failed case IDs
- `evals/report.json` — full `SuiteResult` struct

When `evals/report.baseline.json` exists, markdown includes Δ pass rate per tool.

Reports must **not** contain auth tokens or full file contents from failures (truncate snippets).

## Process hygiene

- Zero `opengrok-go-mcp` processes after `go test ./evals/` completes (`pgrep` empty).
- Subprocess started once per `TestMain`; session closed in cleanup.

## Relationship to other test layers

| Layer | Proves | This harness |
|-------|--------|--------------|
| `internal/mcpserver/*_test.go` | Handler logic | Does not replace |
| `connectMCPServer` in-memory | SDK registration | Does not replace |
| `cmd/.../main_test.go` | Startup gating | Complements (real subprocess) |
| **evals/** | Binary + stdio + env + corpus | **This feature** |

## v1 scope boundaries

- Surface: `full` only
- Mode: `direct-call` only
- Tools in seed corpus: `list_projects`, `search_code`, `read_file`/`get_file_context`, symbol tool
- No LLM tool-selection metrics in headline report
