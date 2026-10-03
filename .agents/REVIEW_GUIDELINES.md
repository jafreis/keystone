# Review guidelines

Review the requested application or Harness change against the actual
repository state and delivery stage. The checkout currently has no product
implementation or build manifest; do not invent package conventions, targets
or test results.

## Findings

Use these categories:

- `BUG`: incorrect behavior, unsafe permission use, data loss or a security
  issue.
- `DEADCODE`: unused or unreachable implementation that adds maintenance cost.
- `IMPROVEMENT`: a concrete clarity, contract or maintainability improvement.
- `PERFORMANCE`: avoidable latency, memory, subprocess, cache or API cost
  supported by evidence.

Each finding needs `path:line`, the triggering state, consequence, severity and
a concrete remedy. Skip formatter-owned issues and unrelated pre-existing
defects. Distinguish a confirmed defect from a missing check or an unresolved
architecture choice. If no supported findings remain, say so and disclose
material verification gaps.

Use this local output format:

`path/to/file:line - [CATEGORY] [blocker|suggestion] Trigger, consequence and remedy.`

Finish with blocker and suggestion counts, a verdict (`ready`, `ready with
suggestions` or `blocked`) and the checks performed or unavailable.

## Product contracts

For changes involving product behavior, read the affected sections of
`.agents/skills/keystone-validation/references/scenarios.md`. Review the
following boundaries when they apply:

- Webhook identity, signature/body validation, explicit repository and
  base/HEAD context, duplicate delivery and queue durability/claim/retry
  semantics are contracts, not implementation details.
- Runner subprocesses receive structured arguments with validated targets,
  revisions and paths. Timeouts, cancellation, output bounds, exit-code
  fidelity and cleanup must be observable.
- OCI assembly remains daemonless. Build, test and lint gates must precede a
  push or deployment; image identity, credential redaction and partial-failure
  behavior must be defined.
- Hybrid selection preserves infrastructure mappings, dependencies,
  deletions/renames and stable deduplicated ordering. Missing graph/hash
  context or detector failure must not become a successful empty plan.
- Remote AC/CAS, repository downloads and trust policy are distinct. A client
  upload flag does not prove server enforcement or developer read-only access.
- BEP and metrics consumers handle duplicates, partial streams, missing
  completion, malformed events, cancellation and unavailable counters without
  fabricating success.
- Aspect output and failure propagation must demonstrate that a failing quality
  check blocks downstream push/deploy behavior.
- Requirements, examples and unresolved choices in product drafts remain
  separate; a roadmap example does not ratify a backend, broker, version,
  registry, namespace or retry policy.

## Harness contracts

- Skills have specific triggers, useful outcomes and loadable references.
- Templates are canonical for delegated-role bodies. Update both checked-in
  host adapters with every template body or shared metadata change; retain only
  valid host-specific metadata.
- Root guidance owns shared constraints; skills own workflows; references own
  conditional domain detail. Resolve contradictions instead of duplicating
  rules.
- Review and verification report evidence and gaps. They do not edit files,
  publish comments, approve or merge changes, push branches, call live
  providers or create operational resources implicitly.
- Changes to tool permissions, data destinations, trust boundaries or host
  metadata require explicit task scope and careful review.

## Verification expectations

Behavior changes need the smallest useful regression or contract check. Use
synthetic payloads and fake boundaries where live systems are unnecessary.
Documentation-only Harness changes need structural and reference checks rather
than application test scaffolding. Report unavailable Bazel, module, host or
network checks explicitly; do not weaken assertions or invent passing evidence.
