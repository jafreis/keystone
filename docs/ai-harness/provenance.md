# Harness provenance

This Harness was adapted from the ClueMesh development Harness inspected at
commit c20eb519b493103ef44766daa1d933263402f590. The source was used as a
structural reference for repository routing, review contracts, thin delegated
roles, manually synchronized host adapters and usage/provenance documentation.

## Retained and adapted material

- Root guidance, review categories and seven common workflow shapes were
  rewritten around Keystone's CI scope. The source's SRE-specific contracts,
  provider names, package claims and MVP decisions were not carried over.
- The two thin role templates and their Augment and GitHub Copilot adapters are
  retained. Their shared bodies and metadata are synchronized; Augment keeps
  only its role colors and all hosts use default model selection.
- The keystone-validation skill and scenario reference were authored for the
  five-sprint webhook, runner, change-selection, cache, BEP and quality-gate
  contracts. They are development and review guidance, not runtime code.
- Harness-specific generators, linters, CI automation, extra host trees and
  autonomous workflows remain deferred.

## Contributor hook adoption

Keystone now includes a root `.pre-commit-config.yaml` and
`.commitlintrc.yaml`. They were designed after comparing the local bff
configuration with Kandev's public configuration, then narrowed to Keystone's
actual root Go module. Public hygiene hooks and commitlint use immutable
upstream revisions; native `gofmt`, `go vet`, and `go test` run from the module
root. No Bazel, pnpm, frontend, generator, private endpoint or infrastructure
dependency was imported from either reference.

The hooks are contributor feedback installed per clone. They can be bypassed
and do not provide server-side enforcement; a future CI policy is a separate
decision.

## Product alignment

The product drafts were read from the owning keystone worktree. They are
untracked and absent from this task checkout; they were not copied, committed
or modified. This Harness records their current scope without presenting the
planned Go control plane, Kubernetes runners, Bazel targets, caches, dashboards
or deployment resources as implemented.

Guidance covers the five planned sprints while preserving open choices such as
state store, broker, dashboard, tool versions, registry/cache trust, retry and
failure semantics. The selected product name is Keystone and the preferred
lowercase slug is keystone; repository rename work remains outside this
change.

## Excluded material

No product source, runtime worker, Kubernetes manifest, Bazel workspace,
deployment resource, product CI workflow, copied product draft, credential,
private host, personal path, live provider, registry or cache integration was
imported. The repository-level contributor hooks are the deliberate tooling
addition described above. No Docker-in-Docker behavior, namespace, service
account or backend choice was inferred from the source Harness.

## Rights and maintenance

This record documents provenance and does not grant a license or resolve
dependency obligations. Keep canonical instructions in one place, review
template and adapter changes together, and update this record when source,
product scope, host support or exclusions change. Preserve these boundaries
unless the user explicitly expands the task.
