# Eval Speed and Release Verification

An eval invocation builds one server binary and reuses it across independent
subprocesses. Race-enabled evals instrument the child and surface subprocess
failures. Preserve the existing eval scenarios and public MCP contract.

Tagged verification runs the pinned scanner and full test suite on the
release toolchain and requires all four raw eval reports. Packaging verifies
six archives and their SBOMs. The staged macOS arm64 and Windows amd64 archives
must pass native MCP smoke tests before those same assets are published.
