#!/usr/bin/env python3
"""Live regression matrix for minimal-setup-surface config combinations.

Requires a reachable OpenGrok instance. Run from repo root after building the server:

    go build -o /tmp/opengrok-go-mcp ./cmd/opengrok-go-mcp
    OPENGROK_MCP_BASE_URL=https://your-host/api/v1 python3 scripts/live-config-matrix.py

Optional:
    OPENGROK_MCP_LIVE_BIN=/path/to/opengrok-go-mcp
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import threading
import time
from dataclasses import dataclass, field
from typing import Any

DEFAULT_BIN = os.environ.get("OPENGROK_MCP_LIVE_BIN", "/tmp/opengrok-go-mcp")
BASE_URL = os.environ.get("OPENGROK_MCP_BASE_URL", "")
SCRAPED_COUNT = os.environ.get("OPENGROK_MCP_LIVE_SCRAPED_COUNT", "")
STARTUP_TIMEOUT = int(os.environ.get("OPENGROK_MCP_LIVE_STARTUP_TIMEOUT", "12"))
MCP_TIMEOUT = int(os.environ.get("OPENGROK_MCP_LIVE_MCP_TIMEOUT", "30"))

ENV_KEYS = [
    "OPENGROK_MCP_BASE_URL",
    "OPENGROK_MCP_PROJECTS",
    "OPENGROK_MCP_DEFAULT_PROJECT",
    "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE",
    "OPENGROK_MCP_PROJECT_SCRAPE",
    "OPENGROK_MCP_API_TOKEN",
    "OPENGROK_MCP_BASIC_AUTH_TOKEN",
]


@dataclass
class Case:
    name: str
    env: dict[str, str]
    expect_startup_ok: bool = True
    expect_log: list[str] = field(default_factory=list)
    expect_log_absent: list[str] = field(default_factory=list)
    expect_default: str | None = None
    mcp_list_projects_count: int | None = None
    mcp_search: dict[str, Any] | None = None


def cases(base_url: str, scraped_count: int) -> list[Case]:
    scraped = f"project source=scraped count={scraped_count}"
    return [
        Case(
            "01_base_url_only",
            {"OPENGROK_MCP_BASE_URL": base_url},
            expect_log=[scraped, "web-UI project discovery enabled"],
            expect_log_absent=["web-UI project discovery disabled"],
            expect_default="",
            mcp_list_projects_count=scraped_count,
            mcp_search={"query": "NewMCPServer", "project": "opengrok-go-mcp", "page_size": 1},
        ),
        Case(
            "02_default_project_on_scraped_list",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DEFAULT_PROJECT": "opengrok-go-mcp",
            },
            expect_log=[scraped],
            expect_default="opengrok-go-mcp",
            mcp_list_projects_count=scraped_count,
            mcp_search={"query": "NewMCPServer", "page_size": 1},
        ),
        Case(
            "03_disable_scrape_no_projects",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE": "true",
            },
            expect_log=["web-UI project discovery disabled", "project source=none count=0"],
            expect_log_absent=["fetching landing page"],
            expect_default="",
            mcp_list_projects_count=0,
        ),
        Case(
            "04_disable_scrape_single_explicit_project",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE": "true",
                "OPENGROK_MCP_PROJECTS": "opengrok-go-mcp",
            },
            expect_log=["project source=configured count=1", "derived from single project list"],
            expect_log_absent=["fetching landing page"],
            expect_default="opengrok-go-mcp",
            mcp_list_projects_count=1,
            mcp_search={"query": "NewMCPServer", "page_size": 1},
        ),
        Case(
            "05_explicit_projects_skip_scrape",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_PROJECTS": "opengrok-go-mcp,retrieval-engineering",
            },
            expect_log=["project source=configured count=2"],
            expect_log_absent=["fetching landing page", "web-UI project discovery enabled"],
            mcp_list_projects_count=2,
            mcp_search={"query": "NewMCPServer", "project": "opengrok-go-mcp", "page_size": 1},
        ),
        Case(
            "06_two_projects_with_default_search_uses_default",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_PROJECTS": "opengrok-go-mcp,retrieval-engineering",
                "OPENGROK_MCP_DEFAULT_PROJECT": "retrieval-engineering",
            },
            expect_log=["project source=configured count=2"],
            expect_default="retrieval-engineering",
            mcp_list_projects_count=2,
            mcp_search={"query": "retrieval", "page_size": 1},
        ),
        Case(
            "07_legacy_scrape_false",
            {"OPENGROK_MCP_BASE_URL": base_url, "OPENGROK_MCP_PROJECT_SCRAPE": "false"},
            expect_log=["web-UI project discovery disabled", "project source=none count=0"],
            expect_log_absent=["fetching landing page"],
        ),
        Case(
            "08_disable_wins_over_legacy_scrape_true",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE": "true",
                "OPENGROK_MCP_PROJECT_SCRAPE": "true",
            },
            expect_log=["web-UI project discovery disabled", "project source=none count=0"],
            expect_log_absent=["fetching landing page"],
        ),
        Case(
            "09_invalid_default_on_scraped_list_fails_startup",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DEFAULT_PROJECT": "no-such-project",
            },
            expect_startup_ok=False,
            expect_log=["not in the resolved OpenGrok project allowlist"],
        ),
        Case(
            "10_single_explicit_project_overrides_invalid_default",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_PROJECTS": "opengrok-go-mcp",
                "OPENGROK_MCP_DEFAULT_PROJECT": "wrong-project",
            },
            expect_log=["project source=configured count=1"],
            expect_default="opengrok-go-mcp",
            mcp_search={"query": "NewMCPServer", "page_size": 1},
        ),
        Case(
            "11_invalid_default_on_multi_explicit_list_fails_startup",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_PROJECTS": "opengrok-go-mcp,retrieval-engineering",
                "OPENGROK_MCP_DEFAULT_PROJECT": "wrong-project",
            },
            expect_startup_ok=False,
            expect_log=["not in the resolved OpenGrok project allowlist"],
        ),
        Case(
            "12_empty_disable_env_still_scrapes",
            {"OPENGROK_MCP_BASE_URL": base_url, "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE": ""},
            expect_log=[scraped],
            expect_log_absent=["web-UI project discovery disabled"],
        ),
        Case(
            "13_trailing_slash_base_url",
            {"OPENGROK_MCP_BASE_URL": base_url.rstrip("/") + "/"},
            expect_log=[scraped],
        ),
        Case(
            "14_scrape_diagnostics_default_on",
            {"OPENGROK_MCP_BASE_URL": base_url},
            expect_log=["project scrape=default-on"],
        ),
        Case(
            "15_disable_scrape_diagnostics",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DISABLE_PROJECT_SCRAPE": "true",
            },
            expect_log=["project scrape=disabled"],
        ),
        Case(
            "16_multi_project_no_default_startup_ok",
            {"OPENGROK_MCP_BASE_URL": base_url},
            expect_log=["default project="],
            expect_default="",
        ),
        Case(
            "17_bare_api_token_fails_startup",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_API_TOKEN": "secret-without-scheme",
            },
            expect_startup_ok=False,
            expect_log_absent=["secret-without-scheme"],
        ),
        Case(
            "18_legacy_basic_auth_env_fails_startup",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_BASIC_AUTH_TOKEN": "dXNlcjpwYXNz",
            },
            expect_startup_ok=False,
            expect_log=["OPENGROK_MCP_BASIC_AUTH_TOKEN"],
            expect_log_absent=["dXNlcjpwYXNz"],
        ),
        Case(
            "19_bearer_token_format_starts",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_API_TOKEN": "Bearer unused-test-token",
            },
            expect_log=[scraped],
            expect_log_absent=["unused-test-token"],
        ),
        Case(
            "20_default_project_on_scraped_list",
            {
                "OPENGROK_MCP_BASE_URL": base_url,
                "OPENGROK_MCP_DEFAULT_PROJECT": "opengrok-go-mcp",
            },
            expect_default="opengrok-go-mcp",
            mcp_search={"query": "NewMCPServer", "page_size": 1},
        ),
    ]


def scrub_env(base: dict[str, str], case_env: dict[str, str]) -> dict[str, str]:
    out = base.copy()
    for key in ENV_KEYS:
        out.pop(key, None)
    out.update(case_env)
    return out


def run_startup(bin_path: str, env: dict[str, str]) -> tuple[int, str]:
    try:
        proc = subprocess.run(
            [bin_path],
            env=env,
            capture_output=True,
            text=True,
            timeout=STARTUP_TIMEOUT,
            input="",
        )
        return proc.returncode, proc.stderr
    except subprocess.TimeoutExpired as exc:
        return 0, exc.stderr or ""


def parse_default(stderr: str) -> str:
    match = re.search(r"startup config: default project=([^\s(]+)", stderr)
    return match.group(1) if match else ""


def mcp_session(bin_path: str, env: dict[str, str], calls: list[dict[str, Any]]) -> dict[int, dict[str, Any]]:
    proc = subprocess.Popen(
        [bin_path],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=env,
        text=True,
        bufsize=1,
    )
    responses: dict[int, dict[str, Any]] = {}

    def read_stdout() -> None:
        assert proc.stdout is not None
        for line in proc.stdout:
            line = line.strip()
            if not line:
                continue
            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                continue
            if "id" in msg:
                responses[int(msg["id"])] = msg

    threading.Thread(target=read_stdout, daemon=True).start()

    def send(obj: dict[str, Any]) -> None:
        assert proc.stdin is not None
        proc.stdin.write(json.dumps(obj) + "\n")
        proc.stdin.flush()

    send(
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "live-config-matrix", "version": "1"},
            },
        }
    )
    send({"jsonrpc": "2.0", "method": "notifications/initialized", "params": {}})

    next_id = 2
    for call in calls:
        send({"jsonrpc": "2.0", "id": next_id, "method": "tools/call", "params": call})
        next_id += 1

    deadline = time.time() + MCP_TIMEOUT
    while time.time() < deadline and len(responses) < next_id - 1:
        time.sleep(0.05)

    proc.terminate()
    try:
        proc.wait(timeout=3)
    except subprocess.TimeoutExpired:
        proc.kill()
    return responses


def tool_json(resp: dict[str, Any]) -> dict[str, Any]:
    content = resp.get("result", {}).get("content", [])
    if not content:
        return {}
    return json.loads(content[0].get("text", "{}"))


def detect_scraped_count(bin_path: str, base_env: dict[str, str], base_url: str) -> int:
    if SCRAPED_COUNT.strip().isdigit():
        return int(SCRAPED_COUNT.strip())
    env = scrub_env(base_env, {"OPENGROK_MCP_BASE_URL": base_url})
    _, stderr = run_startup(bin_path, env)
    match = re.search(r"project source=scraped count=(\d+)", stderr)
    if not match:
        print("Could not detect scraped project count; set OPENGROK_MCP_LIVE_SCRAPED_COUNT", file=sys.stderr)
        sys.exit(2)
    return int(match.group(1))


def main() -> int:
    base_url = BASE_URL.strip()
    if not base_url:
        print("Set OPENGROK_MCP_BASE_URL to your OpenGrok API base (e.g. https://host/api/v1)", file=sys.stderr)
        return 2
    if not os.path.isfile(DEFAULT_BIN):
        print(f"Binary not found: {DEFAULT_BIN}", file=sys.stderr)
        print("Build first: go build -o /tmp/opengrok-go-mcp ./cmd/opengrok-go-mcp", file=sys.stderr)
        return 2

    base_env = os.environ.copy()
    scraped_count = detect_scraped_count(DEFAULT_BIN, base_env, base_url)
    matrix = cases(base_url, scraped_count)
    failed = 0

    print(f"Live config matrix: {len(matrix)} cases")
    print(f"  base URL: {base_url}")
    print(f"  binary:   {DEFAULT_BIN}")
    print(f"  scraped:  {scraped_count} projects (auto-detected)")

    for case in matrix:
        env = scrub_env(base_env, case.env)
        issues: list[str] = []
        notes: list[str] = []

        rc, stderr = run_startup(DEFAULT_BIN, env)
        if case.expect_startup_ok and rc != 0:
            issues.append(f"startup exit {rc}, want 0")
        if not case.expect_startup_ok and rc == 0:
            issues.append("startup succeeded but expected failure")

        for needle in case.expect_log:
            if needle not in stderr:
                issues.append(f"missing log: {needle!r}")
        for needle in case.expect_log_absent:
            if needle in stderr:
                issues.append(f"unexpected log: {needle!r}")

        if case.expect_default is not None:
            got = parse_default(stderr)
            if got != case.expect_default:
                issues.append(f"default project={got!r}, want {case.expect_default!r}")

        if case.expect_startup_ok and (case.mcp_list_projects_count is not None or case.mcp_search):
            calls: list[dict[str, Any]] = []
            if case.mcp_list_projects_count is not None:
                calls.append({"name": "list_projects", "arguments": {}})
            if case.mcp_search:
                calls.append({"name": "search_code", "arguments": case.mcp_search})

            resps = mcp_session(DEFAULT_BIN, env, calls)
            idx = 2
            if case.mcp_list_projects_count is not None:
                body = tool_json(resps.get(idx, {}))
                count = len(body.get("projects", []))
                if count != case.mcp_list_projects_count:
                    issues.append(f"list_projects count={count}, want {case.mcp_list_projects_count}")
                else:
                    notes.append(f"list_projects={count}")
                idx += 1
            if case.mcp_search:
                resp = resps.get(idx, {})
                if resp.get("result", {}).get("isError"):
                    issues.append("search_code returned MCP error")
                else:
                    body = tool_json(resp)
                    hits = len(body.get("results", []))
                    if hits == 0:
                        issues.append("search_code returned no results")
                    else:
                        notes.append(f"search hits={hits} project={body.get('project')!r}")

        status = "PASS" if not issues else "FAIL"
        mark = "✓" if status == "PASS" else "✗"
        line = f"{mark} {case.name}: {status}"
        if notes:
            line += " (" + ", ".join(notes) + ")"
        print(line)
        for issue in issues:
            print(f"    - {issue}")
        if status == "FAIL":
            failed += 1

    passed = len(matrix) - failed
    print(f"\nSummary: {passed}/{len(matrix)} passed")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
