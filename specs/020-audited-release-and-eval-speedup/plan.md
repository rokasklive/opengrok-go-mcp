# Plan

Reuse the existing eval harness and local fixtures. Move binary ownership to
the eval invocation and capture shutdown failures. Use the harness with an
unpacked archive binary to check startup, version, and compact tools.

Keep the pinned scanner and verified Go patch in the tagged verification job.
Build and verify assets once, transfer them to native smoke tests, and publish
the same artifacts with write permission limited to the publish job.
Validate workflow syntax, the Go race suite, and a no-publication dry run.
