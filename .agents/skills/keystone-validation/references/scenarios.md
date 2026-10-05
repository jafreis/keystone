<!-- SPDX-License-Identifier: MPL-2.0 -->

# Keystone CI validation scenarios

Use only sections matching the changed boundary. These are test and design
criteria, not claims that the product or its integrations exist.

## Sprint 1 — webhooks, state, jobs and Kubernetes lifecycle

Validate webhook signatures and bodies before acceptance; reject malformed,
replayed or ambiguous repository events. Preserve repository identity and
explicit base/HEAD revisions. Define run/job identity, dependencies,
durability, claim/ack, retry, idempotency and bounded worker behavior for the
Build, Test, Push and Deploy job types. Exercise queue failure, duplicate
delivery, cancellation, shutdown, backpressure and a worker that loses its
lease. A missing state or queue response must be surfaced, not treated as an
empty successful run.

Use a fake Kubernetes client to cover pod-create failure, startup timeout,
cancellation, cleanup, repeated delivery, namespace/service-account/resource
boundaries and credential selection. Keep the execution trust boundary
configured and explicit before repository-controlled work can reach privileged
cluster, cache, registry or deployment operations.

## Sprint 2 — runner execution and daemonless OCI

Pass subprocess arguments as structured values. Validate repository, revision,
target, path and supported flag boundaries; never interpolate webhook or source
text into a shell. Test isolated checkouts, timeout/cancellation, process
cleanup, bounded output, diagnostics redaction and exact exit-code propagation.
Queue and runner errors must identify whether work was never claimed, failed
before execution or failed in the command.

Keep image assembly daemonless and deterministic with rules_oci or the
selected equivalent. Build, test and lint gates must precede a push/deploy
step. Check image digest/tag identity, credential redaction, push failure,
partial success and retry policy. The specification's push flags are
illustrative: verify the selected target and rules version's actual arguments
before documenting a command.

## Sprint 3 — hybrid target selection and plan merge

Define graph configuration, base/HEAD availability and the selected target
determinant's limits. Graph hashing or impacted-target tooling may require
configured Bazel analysis; do not describe it as query-free or exact for every
configuration without evidence. Exercise additions, modifications, renames,
deletions, shared dependencies, BUILD/bzl/module/toolchain changes and
detector failure.

Keep out-of-tree mapping configurable rather than assuming infra/ paths.
Exercise safe path handling, mapping precedence, unknown paths, infrastructure-
only changes, target-to-infrastructure cross-pollination, deduplication and
stable ordering. Preserve mapped work when the Bazel target set is empty. A
genuinely empty plan must not trigger a default target build, and missing
context must not be reported as “nothing changed.”

## Sprint 4 — remote and repository caches

Distinguish remote action-cache/content-addressable-storage reads and writes
from external repository downloads. Exercise trusted CI write policy,
developer read-only policy, invalid credentials, unavailable cache, isolated
trust domains and the difference between a client upload flag and server-side
enforcement. Do not claim cache safety from
--remote_upload_local_results=false or its inverse alone.

For shared repository caching, define content/version identity, pinning,
concurrent Persistent Volume access, storage failure and cleanup behavior.
Measure warm/cold workloads with a stated denominator, scope and interval.
Dependency download counts, misses and “near-100%” hit-rate ambitions need
evidence; they are not universal guarantees.

## Sprint 5 — BEP, metrics and quality gates

Treat BEP as a structured JSON/protobuf contract with run, event and attempt
identity. Exercise duplicates, ordering, malformed or truncated streams,
missing completion, late events, cancellation, partial ingestion and parser
failure. Telemetry loss must not fabricate build, test, push or deploy success.

For cache metrics, handle counter reset, unavailable endpoints, zero
denominators, sampling interval and scope. Report missing measurements instead
of manufacturing a ratio.

For Bazel aspects and linting, verify the selected aspect/tool versions,
affected-target propagation and output-group behavior where relevant. Include
a failing lint action and show that its result blocks downstream push/deploy;
the presence of an aspect flag or a report alone does not establish a quality
gate.

## Cross-cutting review cases

An initial checkout without a Go module or Bazel target reports application
checks as unavailable while still checking the Harness. A no-change review
reports no supported findings without inventing code or tests. Requested review
remains read-only; requested verification does not repair or publish;
retrospective output proposes edits without applying them. Template changes
route maintenance to both adapter copies. Open backend, broker, dashboard,
transport, lifecycle, retry, trust and metric policies remain prerequisites
until the product explicitly decides them.
