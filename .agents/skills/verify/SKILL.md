---
name: verify
description: Verify a requested Keystone application or Harness change and report exact evidence, unavailable checks and remaining risk.
---

# Verification

Inspect unstaged, staged and relevant untracked changes first; use the actual
requested scope. Identify commands from repository documentation and manifests.
Do not claim application tests passed when no Go module, Bazel target or test
suite exists.

## Application changes

Once a Go module exists, typical checks are `gofmt -l` on touched Go files,
applicable `go test` and `go vet` from its root, and `go test -race` for
affected concurrency paths when supported. Run Bazel checks only against real
targets and selected configuration. Read [CI validation](../keystone-validation/SKILL.md)
for risk-specific scenarios. Use fake Kubernetes/queue/cache/registry clients
and synthetic BEP or webhook inputs unless live access is explicitly authorized.

Report each command, exit/result and contract covered. Formatting or build
commands that modify files are implementation work, not a read-only shortcut.

## Harness changes

- Validate skill frontmatter, supported metadata, name/directory agreement and
  discriminating triggers.
- Resolve relative Markdown links from each file and repository-relative paths
  from the repository root. Do not mistake future product documents for files
  required in this checkout.
- Compare each canonical role body and shared metadata with every supported
  host adapter; check host-only metadata and orphaned files.
- Review routing, authority boundaries, context size, clean/no-change behavior,
  missing context and unavailable-check reporting.
- Run `git diff --check` and inspect the final diff, including untracked files.

These checks establish local consistency, not editor discovery, host schema
support, account access, model behavior or runtime safety. Do not install
tools/hooks, rerun CI, push, publish, deploy or access external systems
implicitly. End with passed checks, failures/unavailable checks and remaining
risks.
