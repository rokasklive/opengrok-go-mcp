# Data Model: Minimal Setup Surface

**Feature**: `002-minimal-setup-surface` | **Date**: 2026-06-10

All state is process-scoped; no persistence. This feature adjusts existing configuration and startup resolution — no new MCP response fields.

## Configuration fields (internal/config.Config)

| Field | Type | Default (after change) | Source |
|---|---|---|---|
| `OpenGrokAPIBaseURL` | string | `""` | `OPENGROK_MCP_BASE_URL` — **only mandatory env var** |
| `OpenGrokAPIToken` | string | `""` | optional |
| `OpenGrokBasicAuthToken` | string | `""` | optional; mutually exclusive with API token |
| `ProjectScrapeEnabled` | bool | `true` | derived from disable flag + legacy override (see below) |
| `ProjectScrapeDisabled` | bool | `false` | `OPENGROK_MCP_DISABLE_PROJECT_SCRAPE` (new) |
| `Projects` | []string | `nil` | explicit list or startup discovery snapshot |
| `DefaultProject` | string | `""` | explicit or auto when exactly one project known |
| `ProjectSource` | string | `""` | `configured \| api \| scraped \| none` |
| `ProjectRequired` | bool | `true` | unchanged default |

### Scrape enablement derivation

```text
if OPENGROK_MCP_DISABLE_PROJECT_SCRAPE explicitly set:
    ProjectScrapeEnabled = !parsed_bool
else if OPENGROK_MCP_PROJECT_SCRAPE explicitly set (legacy):
    ProjectScrapeEnabled = parsed_bool
else:
    ProjectScrapeEnabled = true   # NEW DEFAULT
```

## Startup resolution state machine

Unchanged precedence ladder; only the scrape gate default changes:

```text
configured (OPENGROK_MCP_PROJECTS non-empty)
  → validate default if set; never require default
api (GET /projects/indexed non-empty)
  → skip scrape
api fail/empty AND ProjectScrapeEnabled
  → scraped (one bounded web GET)
api fail/empty AND !ProjectScrapeEnabled
  → none
```

### Post-discovery default project rules

| Projects | DefaultProject before | After validation |
|---|---|---|
| 0 | `""` or explicit | keep explicit if set; else empty — **startup OK** |
| 1 | `""` | set to sole project |
| 1 | explicit | keep if valid |
| N>1 | `""` | empty — **startup OK** |
| N>1 | explicit | must ∈ Projects or startup error |

## Capability probe outcomes

| Field | Meaning |
|---|---|
| `Capabilities.SearchCode` etc. | unchanged booleans per probe |
| `authRemediationNeeded` | ephemeral startup flag: no token configured AND unauthorized seen on probes AND zero probes succeeded |

Used only to emit FR-008 log line; not stored on `Config` long-term unless useful for diagnostics export (optional: log only).

## Probe failure classification (unchanged from 001)

`tls_mismatch | unauthorized | endpoint_disabled | feature_unsupported | transport_error`

Auth remediation applies when **all** search capabilities are off and classification aggregate is `unauthorized-only-with-no-token`.

## Validation rules summary

**Hard fail at Validate()**:
- Missing `OpenGrokAPIBaseURL`
- Missing derivable `OpenGrokWebBaseURL` when API URL lacks `/api/v1` suffix
- Both auth tokens set
- Transport/surface/page-size constraints (unchanged)

**Removed hard fail**:
- `OPENGROK_MCP_DEFAULT_PROJECT` required when multiple projects configured at parse time
- `OPENGROK_MCP_DEFAULT_PROJECT` required when discovery yields zero or multiple projects

**Hard fail at detectCapabilities()** (revised):
- All search probes failed AND failure set includes non-auth-fixable errors (TLS, transport)
- NOT when failures are exclusively unauthorized/forbidden with no token configured

## Entity relationships

```text
Minimal Config (base URL only)
    → Project Resolution (configured|api|scraped|none)
        → Default Project (auto if |projects|==1)
    → Capability Probes
        → Capability Gates (tool registration)
        → Auth Remediation Hint (conditional log)
```
