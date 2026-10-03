---
name: keystone-validation
description: Validate Keystone's webhook, runner, change-selection, cache, BEP and quality-gate contracts for an affected implementation or review.
---

# CI contract validation

Use this alongside the requested feature, fix, task-spec, code-review or
verify workflow. Select only the affected sprint and component, then read the
matching section of [validation scenarios](references/scenarios.md). The
scenarios are design and test criteria for future product work; they do not
claim that the packages, integrations or commands already exist.

Before proposing checks, establish:

- which sprint and implementation stage is in scope;
- repository identity, revisions, target/configuration inputs and expected
  plan/output contract;
- failure, cancellation, retry, resource and credential behavior;
- whether an example command or path is illustrative, selected by a manifest,
  or an unresolved product choice;
- which actions are local/read-only and which would create external state.

For hybrid selection, require explicit base/HEAD and graph configuration and
preserve infrastructure mappings, dependencies, deletions, renames and stable
deduplication. A missing base/hash or detector failure cannot silently become a
successful empty plan. For runner and OCI work, require structured argv,
bounded cancellation-aware execution, daemonless assembly, gate ordering and
redacted diagnostics. For cache work, separate AC/CAS from repository caching
and client flags from server-side trust policy. For BEP, metrics and aspects,
test partial evidence and actual failure propagation rather than inferring
success from absent telemetry or reports.

Use synthetic webhook, queue, pod, subprocess, graph, path, cache, BEP and
metrics fixtures with fake clients and deterministic clocks. Report the
contract, scenarios checked, evidence/results, unavailable tools and remaining
open decisions. Do not invoke live webhooks, queues, clusters, registries,
caches, deployments or paid services through this skill.
