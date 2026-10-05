<!-- SPDX-License-Identifier: MPL-2.0 -->

# AI Harness

The Harness supports development of Keystone, the selected product name for
delta-aware CI orchestration for Bazel monorepos. It is a repository guidance
layer, not the production control plane, worker agent, queue, cache service,
dashboard or deployment system. A development skill cannot grant runtime
permissions.

## Current product context

The current checkout contains this Harness, a root Go module with domain
contracts and tests, and contributor pre-commit configuration. It does not yet
contain the product runtime. The product drafts describe a Go webhook control
plane, atomic Build/Test/Push/Deploy jobs, ephemeral Kubernetes runners,
Bazel-native daemonless OCI assembly, hybrid graph/path change detection,
remote and repository caching, BEP telemetry and Bazel Aspect quality gates
across five planned sprints.

Read spec.md and plan.md from their owning worktree when available. They may be
untracked and outside this checkout; preserve their edits and do not copy,
commit or modify them as part of Harness work. PostgreSQL versus Redis, NATS
versus Redis Streams, dashboard technology, tool versions, trust policies and
some failure semantics remain product decisions until explicitly selected.

## Canonical layout and routing

- [AGENTS.md](../../AGENTS.md) owns shared scope, security, authorization and
  conditional toolchain rules.
- .agents/skills/ contains the canonical feature, fix, task-spec, code-review,
  verify, keystone-validation, harness-improvement and session-retro workflows.
- .agents/skills/keystone-validation/references/scenarios.md contains the
  sprint-specific CI contract scenarios. Read only the affected sections.
- .agents/REVIEW_GUIDELINES.md owns finding categories, verdicts and evidence
  requirements.
- .agents/agent-templates/ contains canonical delegated-role bodies.
- .agents/agents/ and .github/agents/ contain manually synchronized Augment
  and GitHub Copilot adapters.

Use feature for a new capability, fix for a reproducible defect, task-spec for
an ordered breakdown, code-review for a read-only report and verify for checks.
For webhooks, jobs, runners, target selection, OCI, cache, BEP or aspect work,
load keystone-validation with the affected scenario sections. Use
harness-improvement for instruction changes and session-retro only when the
user requests a retrospective.

## Host adapters

The two delegated roles are code-review and verify. Each template body and
shared name/description must match both adapter copies. Augment adapters retain
blue/green role colors; all adapters use host-default models and add no tool
permissions. Update the adapters manually in the same change as a template.

Actual Augment or GitHub Copilot discovery, host schemas, account access and
model behavior are unverified. These files do not claim that every host
discovers repository skills or recognizes identical metadata. No additional
host tree or per-skill UI metadata is needed for the current scope.

## Local verification

Run checks from the repository root. They are portable consistency checks, not
proof of editor discovery or product runtime behavior.

~~~sh
git diff --check
rg --files .agents docs/ai-harness | sort
pre-commit run --all-files --hook-stage pre-commit
~~~

Use an available YAML parser to check that every file named
.agents/skills/*/SKILL.md has nonempty name and description frontmatter, that
the name matches its directory, and that all eight expected skill directories
exist. Compare the Markdown body and shared name/description of each canonical
role with both adapters; allow only Augment's blue/green color metadata to
differ. Check for orphaned role files and resolve every actual relative link.
Search the committed Harness for unfinished placeholders, source-project names,
provider-specific paths, private endpoints and personal configuration; allow
source attribution only in provenance.

The root module currently declares Go 1.27.1 and contains the domain test
suite. Derive checks from the manifests that exist: inspect touched Go files
with `gofmt -l`, run `go test -mod=readonly ./...` and
`go vet -mod=readonly ./...`, use `go test -race` for affected concurrency
paths when supported, and run Bazel only against real targets and
configuration. Run commitlint separately with
`pre-commit run commitlint --hook-stage commit-msg --commit-msg-filename
<disposable-message-file>`; the file-stage all-files run does not validate a
message. Report unavailable commands instead of inventing success. The
pre-commit hooks cover repository hygiene and native Go checks, while the
frontmatter, link, routing, and adapter-parity review above remains Harness
consistency guidance. Do not install generators or create product scaffolding
just to validate the Harness.

## Authority and maintenance

Keep shared constraints in AGENTS.md, workflow decisions in skills, conditional
CI detail in the validation reference and delegated role text in templates.
Review template/adapter parity, frontmatter, routing, links and authority
boundaries together. Local implementation may edit repository files within the
active task. Review and verification are read-only by default; live webhooks,
queue publication, pod creation, cache writes, registry pushes, deployments
and external comments require explicit task scope.

Record source and scope changes in [provenance](provenance.md). A skill or
adapter change does not establish a product integration, editor compatibility
or runtime safety claim.
