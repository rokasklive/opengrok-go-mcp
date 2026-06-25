<div align="center">

<img src="assets/groky.png" alt="opengrok-go-mcp mascot" width="400" />

# opengrok-go-mcp

**Source-code intelligence for agents, powered by OpenGrok and Go.**

*I have seen things you people wouldn’t grep.* — Senior Architect, probably.

<br/>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=for-the-badge&logo=apache&logoColor=white)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)](go.mod)
[![CI](https://img.shields.io/github/actions/workflow/status/rokasklive/opengrok-go-mcp/ci.yml?style=for-the-badge&label=CI)](.github/workflows/ci.yml)
[![MCP](https://img.shields.io/badge/MCP-Server-1f6feb?style=for-the-badge)](docs/tool-contracts.md)
[![Evals](https://img.shields.io/badge/Evals-Tracked-success?style=for-the-badge)](evals/README.md)

</div>

---

Agent-oriented MCP server for searching, navigating, and reading code through
OpenGrok.

It turns OpenGrok into a safer code-intelligence surface for LLM agents:

- capability-gated tools that only appear when the backing OpenGrok feature works
- paginated search and file reads with stable cursors
- citation URLs on code results so answers can point back to source
- warnings for broad, heuristic, truncated, or best-effort results (`warnings[]`
  codes plus a legacy `warning` string)
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

**Required:** `OPENGROK_MCP_BASE_URL` — OpenGrok API base URL ending in `/api/v1`. On
startup the server discovers projects, probes capabilities, and registers only tools that
work. A typical reverse-proxied instance needs nothing else.

Copy a block below, replace the URL, restart the client.

The server defaults to `OPENGROK_MCP_AGENT_PROFILE=economy` (lean payloads, no auto
context expansion). Set `OPENGROK_MCP_AGENT_PROFILE=rich` when you want expanded
search context by default. Internal response diagnostics are off by default; set
`OPENGROK_MCP_DIAGNOSTICS=true` only when debugging pagination/search counters.
See [docs/configuration.md](docs/configuration.md) for the full environment
reference.

<details>
<summary><strong>Released binary</strong> (no Go install)</summary>

Download the archive for your OS/arch from [GitHub Releases](https://github.com/rokasklive/opengrok-go-mcp/releases), verify `checksums.txt`, and point your MCP client at the unpacked `opengrok-go-mcp` binary. Example (Claude Code):

```json
{
  "mcpServers": {
    "opengrok": {
      "command": "/path/to/opengrok-go-mcp",
      "env": {
        "OPENGROK_MCP_BASE_URL": "https://your-opengrok-host/source/api/v1"
      }
    }
  }
}
```

</details>

<details open>
<summary><strong>Claude Code</strong> (<code>go run</code>)</summary>

Add to `~/.claude.json` under `mcpServers`, or run `claude mcp add`:

```json
{
  "mcpServers": {
    "opengrok": {
      "command": "go",
      "args": [
        "run",
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.5.0"
      ],
      "env": {
        "OPENGROK_MCP_BASE_URL": "https://your-opengrok-host/source/api/v1"
      }
    }
  }
}
```

</details>

<details>
<summary><strong>OpenCode</strong> (<code>go run</code>)</summary>

Add to `opencode.json`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "opengrok": {
      "type": "local",
      "enabled": true,
      "command": [
        "go",
        "run",
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.5.0"
      ],
      "environment": {
        "OPENGROK_MCP_BASE_URL": "https://your-opengrok-host/source/api/v1"
      }
    }
  }
}
```

</details>

<details>
<summary><strong>Codex</strong></summary>

Add to `.codex/config.toml` in the project root or `~/.codex/config.toml`:

```toml
[[mcp_servers]]
name = "opengrok"
command = ["go", "run", "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.5.0"]

[mcp_servers.env]
OPENGROK_MCP_BASE_URL = "https://your-opengrok-host/source/api/v1"
```

</details>

<details>
<summary><strong>Other clients</strong> (Cursor, VS Code MCP, …)</summary>

Most stdio MCP clients use the same shape as Claude Code — `command`, `args`, and an
`env` map (name may vary: `environment`, `env`):

```json
{
  "mcpServers": {
    "opengrok": {
      "command": "go",
      "args": [
        "run",
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.5.0"
      ],
      "env": {
        "OPENGROK_MCP_BASE_URL": "https://your-opengrok-host/source/api/v1"
      }
    }
  }
}
```

Cursor: project `.cursor/mcp.json` or **Cursor Settings → MCP**. VS Code: MCP extension
config with the same server entry.

</details>

<details>
<summary><strong>Local clone</strong> (replace <code>go run</code> package path)</summary>

Use the same `env` / `environment` block as above. Example command for Claude Code:

```json
"command": "sh",
"args": [
  "-c",
  "cd /path/to/opengrok-go-mcp && go run ./cmd/opengrok-go-mcp --read-timeout=30s --write-timeout=30s"
]
```

Optional HTTP mode from a clone:

```bash
OPENGROK_MCP_TRANSPORT=http \
OPENGROK_MCP_BASE_URL=https://your-opengrok-host/source/api/v1 \
go run ./cmd/opengrok-go-mcp
```

MCP endpoint: `http://127.0.0.1:8765/mcp`

</details>

### Common environment variables

| Variable | Required | When to set |
|---|---|---|
| `OPENGROK_MCP_BASE_URL` | **yes** | OpenGrok API URL ending in `/api/v1` |
| `OPENGROK_MCP_API_TOKEN` | no | Auth required, or startup logs 401/403 on probes. Full `Authorization` value: `Bearer <token>` or `Basic <credentials>`. Never logged. |
| `OPENGROK_MCP_DEFAULT_PROJECT` | no | Multiple projects and you want one implicit on calls that omit `project`. Auto-set when exactly one project is discovered. |

If search probes return 401/403 without a token, the server still starts and logs remediation;
search tools stay gated until `OPENGROK_MCP_API_TOKEN` is set.

<details>
<summary><strong>All environment variables</strong></summary>

| Variable | Default | Purpose |
|---|---|---|
| `OPENGROK_MCP_WEB_BASE_URL` | derived | Web UI base for citations and raw-file fallback |
| `OPENGROK_MCP_PROJECTS` | — | Comma-separated allowlist; skips API/scrape discovery |
| `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` | `false` | Skip web-UI scrape when `/projects/indexed` fails |
| `OPENGROK_MCP_PROJECT_REQUIRED` | `true` | Require `project` on tool calls |
| `OPENGROK_MCP_PROBE_FILE` | — | `project/path` for file-read capability probe |
| `OPENGROK_MCP_TRANSPORT` | `stdio` | `http` for Streamable HTTP (`127.0.0.1:8765/mcp`) |
| `OPENGROK_MCP_LISTEN` | `127.0.0.1:8765` | HTTP listen address |
| `OPENGROK_MCP_TOOL_SURFACE` | `compact` | `full` (fine-grained tools) or `gateway` (experimental) |
| `OPENGROK_MCP_AGENT_PROFILE` | `economy` | `rich` for expanded search context and per-result links by default. Per-call `expand_context` / `response_mode` / `include_links` still override |
| `OPENGROK_MCP_MEMORY_ENABLED` | `true` | Process-scoped memory tools on the **full** surface only (stdio). Disabled over HTTP regardless of this setting |
| `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY` | `false` | Trusted internal hosts with broken TLS only |
| `OPENGROK_MCP_CURSOR_SECRET` | — | HMAC secret for signed pagination cursors |
| `OPENGROK_MCP_AUTO_EXPAND_CONTEXT` | `true` | Auto-expand context around search hits |
| `OPENGROK_MCP_CONTEXT_BEFORE` / `AFTER` | `5` / `10` | Expansion window lines |
| `OPENGROK_MCP_MAX_EXPANDED_RESULTS` | `10` | Max results to expand |
| `OPENGROK_MCP_MAX_EXPANDED_FILES` | `5` | Max files fetched during expansion |
| `OPENGROK_MCP_CONTEXT_FETCH_CONCURRENCY` | `3` | Parallel fetches during expansion |
| `OPENGROK_MCP_RETRY_MAX_ATTEMPTS` | `2` | Retries for transient OpenGrok errors |
| `OPENGROK_MCP_RETRY_BASE_DELAY` | `200ms` | Retry backoff base |
| `OPENGROK_MCP_CACHE_ENABLED` | `false` | In-process response cache |
| `OPENGROK_MCP_CACHE_TTL` | `5m` | Cache entry lifetime |
| `OPENGROK_MCP_CACHE_MAX_SIZE` | `1000` | Max cache entries |
| `DEBUG` | `false` | `1` logs OpenGrok HTTP requests to stderr |

Context budget overrides: `OPENGROK_MCP_BUDGET_{MINIMAL|DEFAULT|MAXIMAL}_{BEFORE|AFTER|RESULTS|FILES}`.

Deprecated: `OPENGROK_MCP_PROJECT_SCRAPE` (use `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE`).
Removed: `OPENGROK_MCP_BASIC_AUTH_TOKEN` (use `OPENGROK_MCP_API_TOKEN="Basic …"`).

Full reference: [docs/configuration.md](docs/configuration.md).

</details>

## Security

Avoid passing secrets as CLI flags. Use `OPENGROK_MCP_API_TOKEN` for OpenGrok auth;
the server never logs token values.

Operational caveats:

- HTTP mode does not add inbound client authentication. Keep the default
  loopback bind address or put it behind trusted network/auth controls.
- `OPENGROK_MCP_INSECURE_SKIP_TLS_VERIFY=true` is only for controlled internal
  instances with broken certificates. Do not use it for public or untrusted
  hosts.
- Raw file fallback uses `OPENGROK_MCP_WEB_BASE_URL` with the same configured
  credentials. Treat that URL as part of the trusted OpenGrok boundary.
- Memory tools are **full-surface only** (stdio). They are process-scoped,
  ephemeral, and disabled over HTTP because memory is not isolated by client
  session.
- Set `OPENGROK_MCP_CURSOR_SECRET` for shared deployments if cursor integrity
  matters.

## Evaluation

Hermetic stdio evals in [`evals/`](evals/) — real MCP binary, fake OpenGrok backend, no live instance.
CI runs them on every PR ([`ci.yml`](.github/workflows/ci.yml)). README summaries and **Δ** columns compare against committed baselines in [`evals/baselines/`](evals/baselines/). Refresh locally with [`scripts/update-eval-results.sh`](scripts/update-eval-results.sh); the optional pre-push hook ([`scripts/install-githooks.sh`](scripts/install-githooks.sh)) runs tests and updates those files before you push.

```bash
go test ./evals/ -count=1                        # contract + token benchmark
go test ./evals/ -run TestEvalSuite -count=1      # MCP contract only
go test ./evals/ -run TestTokenBenchmark -count=1 # token economy only
```

**How to read the tables below**

- **Contract eval** — hermetic MCP calls against a fake OpenGrok backend; checks outputs,
  errors, and pagination fields. **Δ** is change vs the committed baseline in
  `evals/baselines/`. **coverage@K** is the fraction of eval cases exercised.
- **Token benchmark** — same harness, but measures UTF-8 bytes crossing the MCP wire
  (tool schemas, requests, responses). **Est. tokens** = bytes ÷ 4 (rough heuristic,
  not a specific model tokenizer).
- **Surface** — `full` (fine-grained tools), `compact` (4 consolidated tools, default),
  or `gateway` (experimental discover + call).
- **ListTools** — one-time cost when the client loads the tool list and schemas at
  session start; usually the largest line item on `full`.
- **Warm total** — `ListTools` plus all tool calls in a scenario (request + response
  bytes). For **gateway**, **warm** excludes the one-time `opengrok_discover` call;
  **cold** includes it (first session only). On **full** and **compact**, cold = warm.
- **Min–max** — range across the four replay scenarios (symbol lookup, file browse,
  multi-step symbol search, search-and-read). Per-scenario breakdown is in the
  collapsed table.
- **Δ** on token rows — change in estimated tokens vs the last committed baseline
  (`evals/baselines/token_report.json`).

<!-- EVAL-RESULTS START -->

### Contract eval

Last run: **2026-06-25** · direct-call · [harness docs →](evals/README.md)

**10/10 passed** · 100% (Δ ±0) · 100% coverage@K — see [How to read the tables](#evaluation) for Δ and coverage@K.

<details>
<summary>Per-tool scores</summary>

| Tool | Score | Cases |
|---|---|---|
| get_file_context | 100% (Δ ±0) | 1 |
| list_projects | 100% (Δ ±0) | 1 |
| list_symbols | 100% (Δ ±0) | 1 |
| read_file | 100% (Δ ±0) | 1 |
| search_code | 100% (Δ ±0) | 4 |
| search_symbol_definitions | 100% (Δ ±0) | 1 |
| search_symbol_references | 100% (Δ ±0) | 1 |

</details>

### Token economy benchmark

Last run: **2026-06-25** · deterministic-replay · est. tokens = bytes÷4 (heuristic, not model-exact)

**ListTools** dominates session cost on the full surface (18 tools). Compact (4) and gateway (2) register far fewer schemas.

| Surface | ListTools (est. tokens) | Warm total min–max (est. tokens) |
|---|---|---|
| full | 14k (Δ +724) | 15k–16k (was 14k–15k) |
| compact | 6.8k (Δ +3.3k) | 7.7k–10k (was 4.4k–6.8k) |
| gateway | 261 (Δ ±0) | 1.2k–2.3k (was 1.2k–2.1k) |

_Warm = ListTools + scenario tool traffic. Gateway warm omits one-time `discover`; full/compact cold = warm. Compact **file-exploration** skips `files.list` (no compact op)._

<details>
<summary>Per-scenario warm totals (est. tokens; ListTools + calls)</summary>

| Scenario | full | compact | gateway |
|---|---|---|---|
| Compound symbol | 16k (Δ +911) | 8.3k (Δ +3.5k) | 1.8k (Δ +187) |
| File exploration | 15k (Δ +779) | 7.7k (Δ +3.3k) | 1.2k (Δ +56) |
| Symbol investigation (3 calls) | 16k (Δ +927) | 8.7k (Δ +3.5k) | 2.3k (Δ +203) |
| Search + read | 15k (Δ +834) | 7.9k (Δ +3.4k) | 1.4k (Δ +109) |

</details>

_Δ vs baseline from 2026-06-24._

<!-- EVAL-RESULTS END -->

## Development

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
