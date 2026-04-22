# opengrok-go-mcp

OpenCode-friendly MCP server for project-scoped OpenGrok search.

## Running

For OpenCode `type: "local"` usage, run the command as the MCP process and
pass OpenGrok settings through environment variables:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "opengrok": {
      "type": "local",
      "command": [
        "go",
        "run",
        "github.com/rokasklive/opengrok-go-mcp/cmd/opengrok-go-mcp@latest"
      ],
      "enabled": true,
      "environment": {
        "OPENGROK_MCP_BASE_URL": "https://grok.example.com/source/api/v1",
        "OPENGROK_MCP_WEB_BASE_URL": "https://grok.example.com/source",
        "OPENGROK_MCP_API_TOKEN": "your-api-token"
      }
    }
  }
}
```

Use `OPENGROK_MCP_BASIC_AUTH_TOKEN` instead of `OPENGROK_MCP_API_TOKEN` for
Basic auth. The Basic token value should be pre-encoded.

To run the HTTP endpoint manually:

```bash
OPENGROK_MCP_TRANSPORT=http \
OPENGROK_MCP_BASE_URL=https://grok.example.com/source/api/v1 \
OPENGROK_MCP_WEB_BASE_URL=https://grok.example.com/source \
go run ./cmd/opengrok-go-mcp
```

The HTTP MCP endpoint is available at:

```text
http://127.0.0.1:8765/mcp
```

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--transport` | `stdio` | MCP transport: `stdio` for local command clients, or `http` for Streamable HTTP. |
| `--listen` | `127.0.0.1:8765` | Address the local MCP HTTP server listens on. |
| `--base-url` | | OpenGrok API base URL ending in `/api/v1`. |
| `--web-base-url` | | OpenGrok web UI base URL used for clickable links. |
| `--default-project` | | Default OpenGrok project when a request does not specify one. |
| `--project-required` | `true` | Require a project to be specified or resolved from the default project. |
| `--read-timeout` | `10s` | HTTP server read timeout. |
| `--write-timeout` | `10s` | HTTP server write timeout. |
| `--log-level` | `info` | Logging level. |

## Environment Variables

| Variable | Description |
| --- | --- |
| `OPENGROK_MCP_TRANSPORT` | MCP transport: `stdio` for local command clients, or `http` for Streamable HTTP. |
| `OPENGROK_MCP_LISTEN` | Address the local MCP HTTP server listens on. |
| `OPENGROK_MCP_BASE_URL` | OpenGrok API base URL ending in `/api/v1`. |
| `OPENGROK_MCP_WEB_BASE_URL` | OpenGrok web UI base URL used for clickable links. |
| `OPENGROK_MCP_DEFAULT_PROJECT` | Default OpenGrok project when a request does not specify one. |
| `OPENGROK_MCP_PROJECT_REQUIRED` | Whether a project must be specified or resolved from the default project. |
| `OPENGROK_MCP_LOG_LEVEL` | Logging level. |
| `OPENGROK_MCP_API_TOKEN` | Sends `Authorization: Bearer <token>` to OpenGrok. |
| `OPENGROK_MCP_BASIC_AUTH_TOKEN` | Sends `Authorization: Basic <token>` to OpenGrok. The token should be pre-encoded. Set exactly one OpenGrok auth token; configuring both tokens is an error. |

## Tools

- `list_projects`
- `search_code`
- `search_symbol_definitions`
- `search_symbol_references`
- `get_file_context`

## Resources

- `opengrok://projects`
- `opengrok://project/{project}`
- `opengrok://project/{project}/files/{+path}`

## Security

`opengrok-go-mcp` binds to `127.0.0.1` by default. Do not expose it externally without authentication and network controls.

Avoid passing secrets as CLI flags. Use environment variables for OpenGrok auth tokens.
