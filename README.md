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

> **Pre-1.0 note:** this MCP server is still evolving. Some tools, responses,
> and configuration paths may be broken or change before a stable 1.0 release.
> Please report issues on [GitHub](https://github.com/rokasklive/opengrok-go-mcp/issues).

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

Copy a block below, replace the URL, and restart the client. The `go run` and
`go install` commands require Go 1.26.0 or later. Release binaries are built
with the Go patch pinned in [`.go-version`](.go-version).

See [docs/configuration.md](docs/configuration.md) for every environment variable.

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
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.7.0"
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
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.7.0"
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
command = ["go", "run", "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@v0.7.0"]

[mcp_servers.env]
OPENGROK_MCP_BASE_URL = "https://your-opengrok-host/source/api/v1"
```

</details>

Other stdio clients (Cursor, VS Code, …) use the same `command` / `args` / `env` shape as Claude Code.

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

Hermetic stdio evals in [`evals/`](evals/) run the real binary against a fake
OpenGrok backend on every PR: MCP contract checks plus a token-economy
benchmark per surface.

```bash
go test ./evals/ -count=1
```

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md).

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
