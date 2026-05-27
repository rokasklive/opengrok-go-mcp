# opengrok-go-mcp

Agent-oriented MCP server for searching, navigating, and reading code through
OpenGrok.

It turns OpenGrok into a safer code-intelligence surface for LLM agents:

- capability-gated tools that only appear when the backing OpenGrok feature works
- paginated search and file reads with stable cursors
- citation URLs on code results so answers can point back to source
- warnings for broad, heuristic, truncated, or best-effort results
- automatic context expansion around search hits with explicit limits
- full, compact, and experimental gateway tool surfaces for different agent styles

If you are an AI agent reading this repository, start with [AGENTS.md](AGENTS.md)
for project constraints and agent workflow guidance.

> **Pre-1.0 note:** this MCP server is still evolving. Some tools, responses,
> and configuration paths may be broken or change before a stable 1.0 release.
> Please report issues using [docs/reporting-issues.md](docs/reporting-issues.md).

## When To Use It

Use `opengrok-go-mcp` when you want an agent to investigate a large indexed
codebase without cloning it locally. It works best for finding symbols,
reading files, tracing references, narrowing broad searches, and producing
answers with source citations.

It is intentionally honest about OpenGrok's limits. OpenGrok provides full-text
search plus ctags definitions, not a full semantic call graph or AST engine.
For structural questions, use this server to find the right files and symbols,
then verify relationships with language-aware tools when needed.

## Client Setup

### OpenCode

Add this to `opencode.json`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "opengrok": {
      "command": [
        "go",
        "run",
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.3.0"
      ],
      "enabled": true,
      "environment": {
        "OPENGROK_MCP_BASE_URL": "https://instance.opengrok.com/source/api/v1",
        "OPENGROK_MCP_BASIC_AUTH_TOKEN": "Ik5ldmVyIGdvbm5hIGdpdmUgeW91IHVwIjoiTmV2ZXIgZ29ubmEgbGV0IHlvdSBkb3duIg==",
        "OPENGROK_MCP_DEFAULT_PROJECT": "services-1.0.2-full"
      },
      "type": "local"
    }
  }
}
```

For Basic auth use only the base64 token value, without the `Basic ` prefix. Set exactly one of
`OPENGROK_MCP_API_TOKEN` or `OPENGROK_MCP_BASIC_AUTH_TOKEN`.

`OPENGROK_MCP_WEB_BASE_URL` may be omitted when `OPENGROK_MCP_BASE_URL` ends in
`/api/v1`; the server derives it by trimming that suffix.

If the OpenGrok HTTPS certificate is expired or otherwise invalid, the MCP
server cannot connect until TLS is fixed or
`OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true` is set. Use that only for trusted
internal instances, and remove it once certificates are valid.

### Local Clone Development

When working from a local checkout, keep the same environment block and replace
the published package command with a command that runs from your clone.

OpenCode:

```jsonc
"command": [
  "sh",
  "-c",
  "cd /path/to/mcp/opengrok-go-mcp && go run ./cmd/opengrok-go-mcp --read-timeout=30s --write-timeout=30s"
]
```

Claude Code:

```json
"command": "sh",
"args": [
  "-c",
  "cd /path/to/mcp/opengrok-go-mcp && go run ./cmd/opengrok-go-mcp --read-timeout=30s --write-timeout=30s"
]
```

Codex:

```toml
command = ["sh", "-c", "cd /path/to/mcp/opengrok-go-mcp && go run ./cmd/opengrok-go-mcp --read-timeout=30s --write-timeout=30s"]
```

The timeout flags are optional, but useful when a remote OpenGrok instance or
large query occasionally responds slowly.

### Claude Code

Add to `~/.claude.json` under `mcpServers`, or run `claude mcp add`:

```json
{
  "mcpServers": {
    "opengrok": {
      "command": "go",
      "args": [
        "run",
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.3.0"
      ],
      "env": {
        "OPENGROK_MCP_BASE_URL": "https://instance.opengrok.com/source/api/v1",
        "OPENGROK_MCP_BASIC_AUTH_TOKEN": "Ik5ldmVyIGdvbm5hIGdpdmUgeW91IHVwIjoiTmV2ZXIgZ29ubmEgbGV0IHlvdSBkb3duIg==",
        "OPENGROK_MCP_DEFAULT_PROJECT": "services-1.0.2-full"
      }
    }
  }
}
```

### Codex

Add to `.codex` in the project root or `~/.codex/config.toml` globally:

```toml
[[mcp_servers]]
name = "opengrok"
command = ["go", "run", "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.3.0"]

[mcp_servers.env]
OPENGROK_MCP_BASE_URL = "https://instance.opengrok.com/source/api/v1"
OPENGROK_MCP_BASIC_AUTH_TOKEN = "Ik5ldmVyIGdvbm5hIGdpdmUgeW91IHVwIjoiTmV2ZXIgZ29ubmEgbGV0IHlvdSBkb3duIg=="
OPENGROK_MCP_DEFAULT_PROJECT = "services-1.0.2-full"
```

`OPENGROK_MCP_PROJECTS` is optional. When `/projects/indexed` is restricted,
set **`OPENGROK_MCP_PROJECT_SCRAPE=true`** (experimental) to discover projects
from the web UI `<select id="project">` at startup, or hand-list projects in
`OPENGROK_MCP_PROJECTS`. If the resolved list contains exactly one project,
`OPENGROK_MCP_DEFAULT_PROJECT` may be omitted. When multiple projects are
known, set `OPENGROK_MCP_DEFAULT_PROJECT`. Explicit project arguments must match
the startup-resolved allowlist when one exists.

### HTTP Mode

OpenCode should use local command mode. For manual HTTP use:

```bash
OPENGROK_MCP_TRANSPORT=http \
OPENGROK_MCP_BASE_URL=https://instance.opengrok.com/source/api/v1 \
OPENGROK_MCP_BASIC_AUTH_TOKEN=Ik5ldmVyIGdvbm5hIGdpdmUgeW91IHVwIjoiTmV2ZXIgZ29ubmEgbGV0IHlvdSBkb3duIg== \
OPENGROK_MCP_DEFAULT_PROJECT=services-1.0.2-full \
go run ./cmd/opengrok-go-mcp
```

The HTTP MCP endpoint is available at:

```text
http://127.0.0.1:8765/mcp
```

## Environment

Required:

- `OPENGROK_MCP_BASE_URL`: OpenGrok API base URL ending in `/api/v1`.
- `OPENGROK_MCP_DEFAULT_PROJECT`: project used when tool calls omit `project`. Optional when the startup-resolved project list contains exactly one project.

Common optional settings:

- `OPENGROK_MCP_API_TOKEN`: sends `Authorization: Bearer <token>`.
- `OPENGROK_MCP_BASIC_AUTH_TOKEN`: sends `Authorization: Basic <token>`.
- `OPENGROK_MCP_WEB_BASE_URL`: OpenGrok web UI base URL, used for citations and raw file fallback. Derived from `OPENGROK_MCP_BASE_URL` when omitted and the API URL ends in `/api/v1`.
- `OPENGROK_MCP_PROJECTS`: comma-separated known OpenGrok projects (skips API/scrape discovery when set).
- `OPENGROK_MCP_PROJECT_SCRAPE`: experimental; set to `true` when `/projects/indexed` returns `401`/`403` or is empty but the web UI project picker works. Default off. Then use `list_projects` (startup snapshot) before scoped searches.
- `DEBUG=1`: log OpenGrok API and web requests to stderr.
- `OPENGROK_MCP_TRANSPORT=http`: enable Streamable HTTP mode.
- `OPENGROK_MCP_LISTEN`: HTTP listen address, default `127.0.0.1:8765`.
- `OPENGROK_MCP_TOOL_SURFACE`: tool registration surface, default `full`.
  Set to `compact` to expose fewer wrapper tools (e.g. `opengrok_search`, `opengrok_read`).
  Set to `gateway` to expose only `opengrok_discover` and `opengrok_call`
  (experimental).
- `OPENGROK_MCP_MEMORY_ENABLED`: default `true`. Set to `false` to disable
  process-scoped memory tools. Memory tools are never exposed over HTTP.

Full matrix (defaults, context expansion, retries, cache, cursor signing,
context budgets, probes, TLS): see [docs/configuration.md](docs/configuration.md).

## Tool Surface

At startup, the server probes OpenGrok and exposes only tools whose backing
capabilities work.

The default `full` surface exposes fine-grained tools for:

- project and file discovery: `list_projects` (startup-resolved snapshot), `list_files`, `get_project_overview`
- code search: `search_code`
- symbol search: `search_symbol_definitions`, `search_symbol_references`, `list_symbols`
- source reads: `read_file`, `get_file_context`
- compound flows: `search_and_read`, `find_symbol_and_references`
- best-effort structural discovery: `search_implementations`, `search_cross_project_references`
- stdio-only process memory: `memory_set`, `memory_get`, `memory_list`,
  `memory_delete`, `memory_clear`

The `compact` surface groups the same operations behind fewer wrappers:
`opengrok_projects`, `opengrok_search`, `opengrok_symbols`, `opengrok_read`,
`opengrok_compound`, and `opengrok_memory`.

Both surfaces keep pagination, warnings, citations, and capability gating.
Agent-specific usage guidance lives in [AGENTS.md](AGENTS.md).

### Gateway mode (experimental)

`OPENGROK_MCP_TOOL_SURFACE=gateway` exposes only two tools:

- `opengrok_discover` — returns the list of enabled operations and their
  descriptions. Use this to learn what the server can do.
- `opengrok_call` — dispatch any operation by name with a JSON payload. The
  operation name must match an entry from `opengrok_discover`.

Gateway mode is useful when the agent benefits from a single-call discovery
contract instead of a large static tool list. It is experimental and may change
in future releases. Gateway operations use the same capability rules as the full
and compact surfaces; process-scoped memory operations are not registered over
HTTP.

File reads try `/api/v1/file/content` first, then fall back to authenticated
`/raw/{project}/{path}` under `OPENGROK_MCP_WEB_BASE_URL`.

Search and file outputs include `citation.url`. Agents should include it when
answering about a specific class or file.

## Resources

Resources are exposed only when the matching capability is enabled:

- `opengrok://projects`
- `opengrok://project/{project}`
- `opengrok://project/{project}/files/{+path}`

## Recommended Workflows

### Structural queries (subclasses, implementers, call graphs)

OpenGrok indexes full text plus ctags **definitions** — it knows where a class or
method is *defined*, but not relationships like `extends`, `implements`, or call
edges. For structural questions, combine two tools:

1. Use `search_symbol_definitions` or `list_symbols` (with `path_prefix` / `kind`)
   to find the relevant package(s) and narrow scope — OpenGrok answers *"where is it?"* well.
2. Run a local AST tool (e.g. [ast-grep](https://ast-grep.github.io)) scoped to those
   paths for precise structural matching — *"what are the relationships?"*

A full-text search for `extends BaseController` returns every textual hit (fields,
parameters, comments), not just subclasses. The two-step workflow above replaces the
"search → truncate → grep → repeat" loop.

### Reading pagination

Every paginated tool (`search_*`, `list_symbols`, `list_files`) returns top-level
pagination fields:

- `has_more` — `true` if more pages exist; fetch them by passing `next_cursor`.
- `page` / `total_pages` — your position, e.g. page 1 of 3.
- `total_hits` — the global, unfiltered match count from OpenGrok.

For `list_symbols` with a `kind` filter, `total_hits` counts definitions *before*
the kind filter (OpenGrok cannot filter by ctags kind server-side). A `warning`
makes this explicit; narrow with `path_prefix` to enumerate a kind fully.

## Recommended Configuration

Choose the setup that matches your usage:

- **Full mode (default).** The server exposes every fine-grained tool
  individually (`search_code`, `read_file`, `search_symbol_definitions`, …),
  each with detailed, per-argument descriptions. This is where the richest
  guidance reaches the agent. No additional config needed.

- **Local stdio + compact mode.** Set `OPENGROK_MCP_TOOL_SURFACE=compact` to
  expose fewer, higher-level wrapper tools (`opengrok_search`, `opengrok_read`,
  `opengrok_symbols`, `opengrok_projects`). A cleaner tool list for agents that
  prefer dispatching through a single wrapper; note that per-argument schemas
  are not surfaced for the nested `payload` in this mode.

- **HTTP mode (controlled local/internal setups only).** Set
  `OPENGROK_MCP_TRANSPORT=http`. The server binds to `127.0.0.1:8765` by
  default. Do not expose it externally without authentication and network
  controls. Prefer local stdio when possible; HTTP is useful for agents that
  only support remote MCP endpoints, or when running the server as a sidecar
  process.

## Security

Avoid passing secrets as CLI flags. Use environment variables for OpenGrok auth
tokens.

Operational caveats:

- HTTP mode does not add inbound client authentication. Keep the default
  loopback bind address or put it behind trusted network/auth controls.
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true` is only for controlled internal
  instances with broken certificates. Do not use it for public or untrusted
  hosts.
- Raw file fallback uses `OPENGROK_MCP_WEB_BASE_URL` with the same configured
  credentials. Treat that URL as part of the trusted OpenGrok boundary.
- Memory tools are process-scoped and ephemeral. They are disabled over HTTP
  because memory is not isolated by client session.
- Set `OPENGROK_MCP_CURSOR_SECRET` for shared deployments if cursor integrity
  matters.

## Development Workflow

This project uses GitHub Spec Kit for non-trivial feature planning.

For meaningful behavior changes, new MCP tools, schema changes, configuration
changes, or changes that affect agent-facing behavior, contributors should
start from the project constitution:

- `.specify/memory/constitution.md`

Feature work should generally produce:

- `specs/FEATURE/spec.md`
- `specs/FEATURE/plan.md`
- `specs/FEATURE/tasks.md`

Small bug fixes, documentation edits, dependency bumps, and mechanical
refactors do not require a full Spec Kit workflow unless they affect the public
MCP contract.

All changes must preserve the MCP contract, OpenGrok semantics, security
posture, compatibility expectations, and documentation requirements described
in the constitution.

## Known Limitations

- Large project traversal is bounded, and some search and discovery
  operations are best-effort rather than language-semantic.
- HTTP transport is intended for controlled local or internal setups and does
  not add inbound client authentication.

See [docs/limitations.md](docs/limitations.md) for the detailed current list,
behavioral impact, and mitigations.

## License

`opengrok-go-mcp` is licensed under the Apache License 2.0 (`Apache-2.0`) for
new releases starting with `v0.3.0-beta.2`.

Earlier published releases up to and including `v0.3.0-beta.1` were released
under `CC0-1.0` and remain available under those terms.
