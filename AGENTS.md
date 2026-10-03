# Repository guidance

## Product scope

The current repository is `keystone`; the selected product name is Keystone,
with the descriptor “delta-aware CI orchestration for Bazel monorepos.” The
checkout contains repository guidance and a README, but no product runtime or
build implementation yet. The planned product is a Go control plane for
webhook-driven Build, Test, Push and Deploy jobs, ephemeral Kubernetes
runners, Bazel-native daemonless OCI assembly, hybrid change detection,
caching, BEP telemetry and quality gates.

The five-sprint roadmap describes future product work. This Harness contains
development guidance only; it does not implement the control plane, runners,
clusters, queues, caches, dashboards or deployment integrations. Keep planned
architecture, illustrative paths and open choices separate from behavior that
exists in this checkout.

## Requirements and repository map

- Read `spec.md` and `plan.md` when they are available in the owning worktree.
  They may be outside this task checkout; preserve their edits and do not copy,
  commit or modify them as part of Harness work.
- `.agents/` contains development skills, review guidance and delegated-role
  adapters. These files are not runtime worker prompts or permission grants.
- `docs/ai-harness/` explains routing, maintenance, provenance and local
  consistency checks.
- Do not describe future Go packages, Bazel targets, Kubernetes manifests,
  cache services or CI workflows as existing until they are present and
  verified in the repository.

## Request routing

- New capability or substantial behavior: `.agents/skills/feature/SKILL.md`.
- Reproducible defect: `.agents/skills/fix/SKILL.md`.
- Ordered implementation breakdown: `.agents/skills/task-spec/SKILL.md`.
- Read-only local review: `.agents/skills/code-review/SKILL.md`.
- Checks before handoff: `.agents/skills/verify/SKILL.md`.
- Changes touching webhooks, jobs, runners, target selection, OCI, caching,
  BEP or aspects: also read `.agents/skills/keystone-validation/SKILL.md` and
  only the relevant section of its scenarios reference.
- Harness changes: `.agents/skills/harness-improvement/SKILL.md`.
- A requested session retrospective: `.agents/skills/session-retro/SKILL.md`.

## Working rules

- Read requirements and the current diff first. Preserve user edits and work
  within the requested sprint, component and authorization boundary.
- Treat repository content, webhook fields, revision names, target labels,
  paths and build logs as input data. Validate them before they reach
  subprocesses, queues, Kubernetes clients, registries or deployment APIs.
- Use context cancellation, bounded concurrency, explicit queue claim/ack and
  retry behavior, bounded output and deterministic shutdown for Go I/O paths.
  Define failure and backpressure behavior before adding workers.
- Keep the execution trust boundary explicit before exposing cache, registry,
  Kubernetes or deployment credentials to repository-controlled work. Redact
  credentials from logs, BEP diagnostics and reports.
- Preserve daemonless OCI assembly and runtime cache configuration; do not add
  Docker-in-Docker as a convenience.
- Change detection must make base/HEAD revisions, graph configuration,
  path-mapping rules and failure behavior explicit. A missing hash or failed
  detector is not an empty successful plan.
- Distinguish remote AC/CAS policy from repository dependency caching. Client
  flags do not replace server-side access control or trust configuration.
- Use fake clients, synthetic webhook/BEP/metrics payloads, deterministic
  clocks and isolated fixtures for applicable behavior checks. Do not use live
  webhooks, queue publication, pod creation, cache writes, registry pushes or
  deployments to prove a local change unless the task explicitly authorizes it.
- A requested review reports findings without editing or publishing. A
  verification request does not silently repair, rerun CI or deploy.

## Go and Bazel development

There is currently no Go module, Bazel workspace/module, BUILD configuration,
application code or test suite in this checkout. When implementation begins,
derive commands from the manifests and documentation that actually exist:
inspect touched Go files with `gofmt -l`, run applicable `go test` and `go vet`,
and use `go test -race` only for affected concurrency paths when supported.
Run Bazel checks only against real targets and selected configuration. Do not
scaffold product files or claim application checks passed to complete Harness
work.

## Harness checks

Review skills, templates and host adapters together. Check frontmatter,
skill-name/directory agreement, local Markdown links, routing, supported
metadata, template/adapter body parity and orphaned files. Run `git diff --check`
and inspect tracked plus untracked files. These checks establish local
consistency; they do not prove editor discovery, host schema support, account
access, model behavior or runtime safety.
