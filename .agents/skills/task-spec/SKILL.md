---
name: task-spec
description: Turn a scoped Keystone CI capability or approved design into ordered implementation tasks with observable acceptance criteria.
---

# Task specification

Determine the delivery stage and read the relevant requirements. Consult [CI
validation](../keystone-validation/SKILL.md) when decomposing webhooks, jobs,
runners, target selection, OCI, caching, BEP or aspects. Mark unresolved
choices before tasks that depend on them; do not select PostgreSQL, Redis,
NATS, a dashboard, a registry or a tool version just because it appears in an
example.

For each task state:

- a stable ID, verb-first outcome and target component or proposed paths;
- the contract changed, sprint/stage and explicit dependency IDs;
- acceptance criteria covering the useful result and material failure,
  cancellation, trust and boundary cases;
- a synthetic fixture, test or other observable completion evidence;
- data, credential and side-effect scope plus rollback or migration concerns
  where relevant.

Order shared event, revision, job and trust contracts before their consumers.
Keep graph targets, infrastructure mappings and plan merge behavior explicit;
preserve dependencies and stable ordering. Make cache policy and telemetry
failure semantics separate decisions from command construction.

Output the tasks and unresolved prerequisites. Do not create tickets, persistent
tasks, sessions, product files or external integrations unless the user
explicitly requests that action.
