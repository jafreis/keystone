<!-- SPDX-License-Identifier: MPL-2.0 -->

# Keystone state and identity contracts

- **Status:** Proposed backend contract with A1.05 domain implementation
- **Contract family:** `keystone.state`
- **Document version:** `0.3.0`
- **Fingerprint version defined here:** `F1` (`keystone/fingerprint/v1`)
- **Scope:** Sprint 1, Tasks A1.01 and A1.04–A1.05
- **Implementation status:** Pure validated plan, job, analysis, work-reference,
attempt, reservation, evidence and result contracts are present in
`internal/domain`; no state adapter, queue, HTTP route or worker is present in
this checkout.

This document is the normative, backend-neutral contract for identities,
normalized replay comparison, immutable plans and transaction boundaries. It
constrains the later A1 state implementation and the A2/A3 consumers. It does
not select a database, broker, provider policy, revision strategy, Kubernetes
implementation or deployment adapter.

The terminology `MUST`, `MUST NOT`, `SHOULD`, `SHOULD NOT` and `MAY` expresses
the contract strength. An implementation may choose different field or table
names, but it must preserve the identities, authority boundaries, atomicity and
recovery behavior defined here.

## 1. Authority, provenance and scope

The [delivery plan](../delivery-plan.md) is the architectural source for the
proposed control-plane boundaries, durable admission/outbox flow, immutable
plans, fenced execution and state-based reconciliation. The owning worktree's
`docs/spec.md`, `docs/plan.md`, `docs/milestones/A1.md` and
`docs/milestones/A2.md` are requirement drafts referenced by task and section
name; they are not copied into this checkout. No A0 decision record was
available when this contract was written.

This repository currently contains development guidance and planning documents,
not the product runtime. A future implementation must provide evidence for
backend durability, consistency, retention, restore and uncertain-commit
behavior before claiming these guarantees operationally.

### 1.1 Authority rules

No field gets authority merely because it is present in a signed body, a
provider header, a repository file or a worker message. Authority is assigned
per field and remains attached to the field through normalization and storage.

| Contract fact | Produced by | Validated by | Authoritative source and use |
|---|---|---|---|
| Provider instance identity | Adapter configuration and the authenticated integration binding | Authentication adapter and trusted registration lookup | Operator-managed integration identity; it is part of the delivery namespace. |
| Registered repository ID | Repository registry | Admission boundary | Operator-managed immutable repository record; it selects fetch endpoints, policy and credential domains. |
| Provider repository ID | Authenticated provider payload | Adapter and registration binding | Provider metadata, only after it matches the registered integration. It is a binding fact, not a fetch URL. |
| Delivery ID | Provider delivery metadata | Adapter-specific validation | Validated provider metadata. Missing, malformed or ambiguous values follow the selected A2 policy and are never synthesized. |
| Raw-body signature evidence | Authentication adapter | Provider-specific verifier | Verification result plus a reference to the key/configuration version; signature headers and secrets are not semantic event fields. |
| Normalized event kind/action | Adapter | Adapter schema and policy | Versioned adapter normalization. Unsupported or ambiguous events are rejected or recorded as ignored according to policy. |
| Push or pull-request candidate revisions | Adapter | Adapter field validation | Authenticated event facts. They remain candidates until immutable object resolution succeeds. |
| Source and target repository binding for a PR | Adapter | Registered target binding and provider metadata | The target registration is authoritative for privileges; a source repository is retained as context and cannot grant target privileges. |
| Actor, installation or other immutable identity | Adapter | Authentication and trusted integration binding | Included in admission policy only when that policy names the field. A policy-required field that is absent is not replaced with a display name. |
| Original policy/configuration decision | Admission policy evaluator | Trusted operator policy and registration | A versioned snapshot and references committed with the admission. A replay returns this decision and does not silently recompute it. |
| Resolved base/head objects | Revision resolver | Isolated resolver plus repository binding | Verified immutable repository objects and fetch evidence. A branch name, provider URL or synthetic merge value is not proof of resolution. |
| Plan and job graph | Plan compiler | Control plane and store preconditions | Immutable normalized plan content and digest. Repository input can contribute evidence but cannot publish an arbitrary executable graph. |
| State transition, record version and fence | State store | Conditional write in the selected backend | The authoritative durable state. A broker delivery, pod observation or worker claim request cannot bypass it. |
| Queue publication receipt | Outbox/publisher adapter | Broker confirmation and outbox bookkeeping | The broker's confirmed receipt, paired with the durable outbox record. An attempted send is not a confirmed publication. |
| Attempt result and gate evidence | Authorized worker under an active fence | Store and result contract | The fenced result record plus bounded evidence references. Missing required evidence is unknown and cannot pass a gate. |
| External push/deploy receipt | Selected external adapter | Destination-specific reconciliation | The external system's receipt or observed state. A local commit cannot claim that an external effect occurred. |

Signature verification authenticates the bytes covered by the provider's
verification mechanism. It does not automatically authenticate every header,
delivery identifier or provider field. The adapter must record provenance for
each field it uses. A payload URL, branch name, actor display name, repository
name or repository-controlled configuration cannot select credentials, increase
execution trust or replace a registered identity.

### 1.2 Contract decision identifiers

The following identifiers are stable references for review and later
implementation. They are contract decisions, not runtime records.

| ID | Decision |
|---|---|
| `ST-001` | A delivery is unique within `provider identity + registered repository ID + delivery ID`. |
| `ST-002` | Semantic normalized content under a recorded fingerprint version decides replay equality; a raw-body digest is separate evidence. |
| `ST-003` | The original admission decision and policy references are retained; an identical replay does not re-evaluate current policy. |
| `ST-004` | Candidate revisions, resolved revisions, failed resolution and explicit complete-no-work are distinct states. |
| `ST-005` | Runs, plans, jobs and logical operation keys are immutable at their identity scope; attempts and retries do not mutate their keys. |
| `ST-006` | Store commit precedes the success response or queue acknowledgment that depends on it. A lost response is unknown until authoritative read/replay reconciliation. |
| `ST-007` | State transactions stop at durable intent and evidence. Broker publication, pod creation, artifact transfer, registry mutation and deployment mutation require separate reconciliation. |
| `ST-008` | Schema, fingerprint, policy, plan and operation-key versions are recorded independently and are not silently rewritten during replay. |

## 2. Identity vocabulary and scopes

Identifiers are opaque, typed values. They must not be reconstructed from
display names, timestamps or mutable branch tips. A future storage schema may
use UUIDs, encoded strings or another representation, provided the scope and
immutability below remain observable.

| Identity | Meaning | Uniqueness or stability scope |
|---|---|---|
| `ProviderIdentity` | A provider kind plus an unambiguous provider integration/instance identity. | Stable for the configured integration; two legitimate provider instances are separate namespaces. |
| `RegisteredRepositoryID` | The operator-managed repository record used for policy, fetch and credential binding. | Immutable within a registration history. A URL or display name is not this ID. |
| `ProviderRepositoryID` | The provider's immutable repository identifier. | Unique within its provider instance; it must bind to the registered repository before admission. |
| `DeliveryID` | The provider's delivery/event identifier as validated by the adapter. | Provider-defined; it is not generated by the control plane. |
| `DeliveryKey` | `ProviderIdentity + RegisteredRepositoryID + DeliveryID`. | The deduplication key for one delivery namespace. |
| `AdmissionFingerprint` | Versioned digest of the normalized immutable event projection. | Compared only under its recorded fingerprint version and delivery key. |
| `RerunRequestKey` | The caller/request idempotency identity for a deliberate rerun. | Scoped by the authenticated rerun authority and registered repository according to the API policy. It is distinct from `DeliveryKey`. |
| `RunID` | One admitted execution intent, including a deliberate rerun. | Stable for the run's complete history; redeliveries and worker retries reuse it. |
| `RevisionID` | A typed reference to a pending, resolved or failed revision context. | Stable for the recorded context; resolved content is immutable. |
| `PlanID` | The identity of one immutable compiled plan. | Stable for a run and plan schema. A correction uses an explicit replacement/rerun identity. |
| `PlanDigest` | Digest of the canonical plan content under its plan schema version. | Same content and version has the same digest; a changed digest under the same `PlanID` conflicts. |
| `JobID` | One immutable node in a plan. | Stable within its plan; dependency edges refer to job IDs from that plan only. |
| `WorkReferenceID` | Durable reference to analysis or executable work. | Stable across outbox publication retries and broker redelivery. |
| `MessageID` | Broker-facing stable message identity for a work reference. | Stable across publication retries; it grants no execution authority. |
| `OperationKey` | Logical external or execution operation identity derived from immutable run-scoped job semantics. | Stable across attempts, workers, leases and retries for one run. |
| `AttemptNumber` | Monotonic attempt ordinal for a job or analysis work item. | Increases only when the state contract authorizes a new attempt. |
| `FenceToken` | Conditional ownership token for an active attempt. | Only the current token may renew or finalize that attempt. |
| `RecordVersion` | Monotonic version used by conditional state writes. | Advances on the state record governed by the store. |
| `CancellationVersion` | Version of persisted cancellation intent. | Advances on cancellation changes; later work must check it. |
| `ExternalOperationID` | Destination-specific identity sent to or reconciled with an external system. | Stable for one destination mutation; it does not imply that the mutation succeeded. |

The following equality rules apply:

1. A delivery replay uses the exact `DeliveryKey`. A same-looking delivery
   from a different registered repository or provider instance is a different
   namespace.
2. A `DeliveryKey` with an equal normalized projection is an identical replay,
   even when harmless JSON serialization varies. The original durable outcome
   is returned.
3. A `DeliveryKey` with a different normalized projection is a conflicting
   replay. The existing admission remains unchanged and the caller receives a
   conflict outcome.
4. A deliberate rerun must carry a new `RerunRequestKey`, produce a new
   `RunID`, and link to the source run or delivery. Repeating that same rerun
   request reuses the new run.
5. A worker retry never creates a new run, plan or operation key. It creates a
   new `AttemptNumber` only after the state contract classifies the prior
   attempt and authorizes retry.

## 3. Delivery identity and normalized replay comparison

### 3.1 Delivery key and namespace

The unique delivery key is:

```text
DeliveryKey =
  ProviderIdentity
  + RegisteredRepositoryID
  + DeliveryID
```

`ProviderIdentity` must include enough provider-instance or integration
identity to avoid aliasing two legitimate installations. `RegisteredRepositoryID`
is the trusted repository record, not a payload URL or display name. The
provider repository identifier is retained and checked against that record.

A request whose provider integration, provider repository ID, PR target or
registered repository does not match the trusted registration is rejected or
ignored before admission according to the selected A2 policy. It must not
create a delivery record, run, quota reservation or outbox item. A separate
legitimate registered repository may use the same provider-local delivery ID
without colliding because its `RegisteredRepositoryID` differs.

The adapter must reject or explicitly classify all of the following before
computing a semantic fingerprint:

- missing, malformed or ambiguous delivery identity;
- duplicate security fields whose interpretation could vary;
- a provider repository ID that does not match the registered binding;
- a PR target that does not match the outer registered repository;
- required event fields that are absent or have an invalid type;
- a source/head or base candidate that violates the selected event policy.

The adapter must not invent a delivery ID, substitute the current branch tip,
or use a raw body digest as a replacement identity.

### 3.2 Fingerprint version `F1`

`F1` is the semantic comparison contract for a normalized delivery. The
implementation must retain the fingerprint version and either the canonical
normalized projection or sufficient collision evidence to compare it under
that version. It must not re-fingerprint an old record with a new rule during
replay.

The `F1` digest construction is:

```text
AdmissionFingerprint =
  SHA-256(
    UTF-8("keystone/fingerprint/v1\0")
    || CanonicalProjection
  )
```

The canonical projection uses deterministic UTF-8 encoding with:

- fixed field names and an explicit type tag for every value;
- length-delimited strings and byte sequences;
- canonical decimal representation for numeric identifiers;
- explicit `absent`, `null` and present-value markers;
- fixed object-field order defined by this contract;
- semantic array order only where order matters; otherwise a typed,
  deterministically sorted set;
- normalized provider IDs, refs and actions only according to the adapter's
  versioned rules;
- rejection of duplicate fields that could produce different security or
  identity interpretations.

JSON whitespace, object-key order and irrelevant additive provider fields do
not affect `F1`. Equivalence is limited to forms explicitly normalized by the
provider adapter. A value that changes event meaning, binding, revision
candidate, policy input or comparison intent remains different.

The store MUST retain the canonical normalized projection alongside the
digest. If a digest matches but the canonical projections differ, the store
must return a `fingerprint_collision` or equivalent indeterminate conflict and
must make no partial write. Digest equality alone is not permission to merge
different projections.

### 3.3 Fingerprint field participation

The following table defines the `F1` projection. A conditional row is included
when and only when the versioned admission policy consumes that fact; the
policy must make the choice explicit.

| Projection group | `F1` fields and provenance | Rules |
|---|---|---|
| Provider binding | Provider kind, provider instance/integration ID, registered repository ID, validated provider repository ID | All are typed IDs. Registration supplies the trusted repository ID; payload names and URLs are not substitutes. |
| Delivery metadata | Delivery ID and its validated provenance marker | The ID is included to bind the projection to the delivery record. Header spelling or raw header bytes are excluded. |
| Event | Normalized event kind and action | The adapter maps provider names into the versioned event vocabulary. Unsupported values do not reach fingerprint comparison. |
| Push context | Full ref, before candidate, head/after candidate, created, deleted and forced flags | Preserve explicit absent/null values. Do not infer a base or use the payload commit list as a complete diff. |
| Pull-request identity | Provider PR identity/number, source repository ID, target registered/provider repository ID, source/head ref, target/base ref, head candidate, base candidate and comparison intent | Source and target remain separate. The target binding must already be valid. A synthetic merge value is a candidate, not a verified revision. |
| Policy-consumed immutable facts | Immutable actor ID, installation ID or other provider fact actually named by the admission policy | Include the typed value and provenance. If required by policy and absent, reject; do not fall back to a display name. |
| Adapter semantic version | Provider adapter normalization version and `F1` | This identifies the interpretation of the projection. It is not a current policy snapshot. |

The following are retained as separate admission evidence or state metadata,
not as semantic content fields:

- original admission outcome, run identity and policy/configuration references;
- authentication result, key/configuration version and references to verified
  evidence;
- raw-body digest, when retained under approved A2 policy;
- receipt and processing timestamps, retry counts and response metadata;
- later resolved base/head objects, analysis results, plan content and worker
  outcomes.

Raw request bytes, signatures, authorization values and secrets MUST NOT be
retained in durable admission records, outbox messages or diagnostics. Raw
bytes are used only within the bounded authentication request lifetime;
durable authentication evidence contains verification results and approved
references, never credential values.

A raw-body digest may be useful for forensic evidence or a provider-specific
replay policy. It is not authoritative for semantic replay equality. Two
serializations that normalize to the same `F1` projection must replay
identically even if their raw bytes and raw-body digests differ. The A2.2
wording that currently lists a body digest as a fingerprint input must be
aligned to this rule before implementation.

### 3.4 Worked comparison cases

| Input case | Delivery namespace | `F1` result | Required outcome |
|---|---|---|---|
| Same push fields with JSON whitespace, key-order or irrelevant additive-field changes | Same `ProviderIdentity`, `RegisteredRepositoryID`, `DeliveryID` | Equal canonical projection | Identical replay; return the stored outcome and do not create work. |
| Same PR fields serialized with different object order or harmless provider formatting | Same delivery key | Equal canonical projection | Identical replay; preserve the original policy decision and run. |
| Same delivery key but different head candidate | Same delivery key | Different projection | Conflicting replay; leave all prior records unchanged. |
| Same delivery key but different base candidate or comparison intent | Same delivery key | Different projection | Conflicting replay; do not resolve or replace the prior revision. |
| Same delivery key but different normalized action or source repository | Same delivery key | Different projection | Conflicting replay; no second admission. |
| Same provider-local delivery ID for two legitimate registered repositories | Different `RegisteredRepositoryID` | Different delivery keys | Separate admissions may proceed under their respective policies. |
| Payload repository ID or PR target mismatches trusted registration | Namespace cannot be trusted | No admission fingerprint | Reject/ignore before any durable admission write. |
| Required delivery, event, repository or revision field omitted | Incomplete identity/content | No admission fingerprint | Apply selected invalid-input policy; never treat missing context as an empty successful plan. |

## 4. Revision, policy and authority representation

### 4.1 Candidate, pending, resolved and failed context

Revision resolution is a state transition on an admitted run. It is not a
change to delivery identity and it does not change the replay fingerprint.
Every representation must state its resolution status and the policy/version
that produced it.

| Representation | Required content | What it permits |
|---|---|---|
| Candidate event context | Validated repository bindings, immutable head candidate, base candidate if supplied, refs, event kind/action, source/target context and comparison intent | Admission and an analysis request. It does not authorize an executable plan. |
| `pending` revision | Candidate context plus resolver strategy/version, configuration/profile identity, unresolved reason or required evidence and a durable resolution attempt reference | Waiting or retrying resolution. No plan execution and no complete-no-work success. |
| `resolved` revision | Registered repository binding; verified immutable head commit object; verified base commit object when the selected strategy requires one, or an explicit verified no-base result for an initial event; strategy; configuration/profile identity; policy version; fetch/object evidence; resolver version | Plan compilation under the recorded policy. |
| `failed` revision | Candidate context, resolver strategy/version, bounded classified failure, retry/reconciliation classification and evidence reference | A visible planning failure or needs-reconciliation state. It does not authorize an empty plan. |
| Explicit complete-no-work result | A resolved revision, successful detector/configuration evidence, complete selection status and a recorded zero-work result | A terminal no-work outcome only after completeness is proven. |

The resolver must never use a branch tip in place of a required immutable
base, manufacture a hash, treat a missing hash as an empty diff, or turn
detector failure into complete-no-work. Initial commits and deletion events
require an explicit policy outcome. An explicit no-base strategy can be
resolved only when the policy and evidence say that no base object is
required; it is not a fabricated base.

### 4.2 Policy authority

| Policy decision | Authority | Consumer obligation |
|---|---|---|
| Repository registration, allowed fetch endpoints, integration binding, credential domain and trust class | Operator-managed registry | A2 and A1 must use the stored binding; repository input cannot edit it. |
| Supported provider events/actions, replay retention, secret rotation and admission limits | A2 policy owner after the A2.1 decision | The adapter records the policy version and maps outcomes explicitly. |
| Push/PR base strategy, force/initial/deletion behavior and resolver limits | A2 policy plus revision-resolver decision gate `D4` | The resolver records candidate versus verified state and failure behavior. |
| Configuration/profile, mapping rules, required gates and execution class | Trusted operator policy | The compiler validates and binds them to the plan; source configuration cannot promote privilege. |
| Delivery uniqueness, conditional writes, fencing, idempotency receipts and durable state | A1 contract and selected `D1` backend | The store is authoritative; later components revalidate expected versions and fences. |
| Ready release, retry eligibility, class-bound claims, queue acknowledgment and capacity reservations | A3 consumers of A1 state | Scheduler and publisher decisions are checked again by conditional writes. |
| Destination approval, external receipt and deployment reconciliation | `D9`/`D11` policy and selected adapter | Push/Deploy effects use stable operation identity and remain needs-reconciliation when uncertain. |

The original policy/configuration references and the decision outcome are
committed with admission. A duplicate replay returns that recorded decision,
even if policy has since changed. A policy change can affect a new delivery or
an explicitly authorized rerun, subject to the policy's compatibility rules.
It cannot silently rewrite an admitted run.

## 5. Run, rerun, plan and operation identity

### 5.1 Runs and deliberate reruns

An identical provider delivery replay returns the original durable outcome and
`RunID`, including when the original run is pending, failed, canceled or
terminal. It never creates another analysis item or consumes another quota
reservation.

A deliberate rerun is a separate authorized request:

1. The caller supplies or receives a stable `RerunRequestKey` under the
   authenticated rerun authority and repository scope.
2. The store atomically records the rerun request receipt, a new `RunID`, its
   source delivery/run link, the selected immutable revision/configuration
   intent, the policy version and the analysis outbox item.
3. Repeating the same rerun request returns the new run and its recorded
   outcome. It does not reinterpret the provider delivery.
4. A changed rerun intent requires a new explicit request key and new
   admission. The existing run, plan and result remain immutable.

Unless a later policy explicitly permits a changed revision, a rerun pins the
original verified revision and configuration intent. An unresolved original
context remains unresolved; a rerun cannot turn it into a fabricated verified
revision. Rerun authorization, allowed policy changes and quota accounting are
owned by the later API/policy decisions.

If a rerun commit response is lost, the caller must read or replay the same
`RerunRequestKey`. It must not generate a new key merely because the response
was lost.

### 5.2 Immutable plans and jobs

An `ExecutionPlan` is canonical content containing at least:

- `PlanID`, `PlanSchemaVersion` and `PlanDigest`;
- `RunID`, resolved revision identity and configuration/profile digest;
- policy and required-gate versions;
- deterministic ordered job specifications;
- dependency edges restricted to jobs in the same plan;
- selection completeness and provenance;
- execution class, resource profile, timeout and retry policy;
- artifact, destination and gate bindings.

The canonical digest includes all content that affects work or authorization.
The store applies these rules:

- same `PlanID`, schema version and canonical content is an idempotent replay;
- same `PlanID` with changed canonical content is a conflict with no partial
  replacement;
- a cyclic, cross-run, policy-incompatible or unresolved plan is rejected
  atomically;
- saving a plan does not release blocked jobs or imply queue publication;
- a correction uses an explicit versioned replacement or deliberate rerun
  contract and retains the prior plan and evidence;
- terminal results and gate evidence stay bound to the plan/revision/
  configuration scope that produced them.

### 5.3 Stable operation keys

The logical operation key is versioned and derived from immutable, run-scoped
job semantics:

```text
OperationKey =
  OperationKeyVersion
  + RunID
  + JobKind/Executor
  + ResolvedRevisionIdentity
  + ConfigurationIdentity
  + TargetOrArtifactIdentity
  + DestinationIdentity
  + GateAndPolicyScope
```

The canonical encoding follows the same typed, length-delimited rules as `F1`.
The key includes every value that changes the logical operation and excludes:

- `AttemptNumber`, worker identity, lease/fence token and timestamps;
- broker delivery ID, publication attempt and queue receipt;
- transient pod name, retry count and process ID;
- mutable branch tips or display labels.

Retries and workers reuse the same key. A deliberate rerun has a new `RunID`
and therefore new run-scoped operation keys, even when it intentionally pins
the same revision and destination. Push and Deploy also require a
destination-specific `ExternalOperationID` and receipt/reconciliation record.
An operation key prevents an unintended duplicate identity; it cannot undo an
external effect that was already issued.

### 5.4 A1.04 domain implementation boundary

The A1.04 implementation in `internal/domain` freezes validated construction
inputs into private `ExecutionPlan`, `JobSpec` and `AnalysisSpec` snapshots.
Accessors return copies, and strict JSON decoding validates a temporary value
before replacing a receiver. The implementation provides the following v1
rules:

- plan, job, analysis and work-reference schema versions are independent;
- plan, job, analysis and operation projections use the `sha256:` plus lowercase
  hexadecimal digest grammar and typed length-delimited canonical fields;
- operation and execution class are derived from the supported operation matrix;
- Build, Test, Push and Deploy are public job kinds; analysis is a separate work
  class;
- job dependencies, producer outputs, gate producers, stored array order and
  complete-empty provenance are validated before a snapshot is returned;
- queue references are bounded to 16 KiB and contain identity/version/digest
  bindings only.

These constructors do not select targets, resolve revisions, compile plans,
publish messages, execute commands or authorize credentials. `ValidateAgainst`
methods compare snapshots with separately supplied run and trusted-binding
values; payload fields do not grant trust or execution authority.

### 5.5 A1.05 attempt, evidence and result implementation boundary

The A1.05 implementation in `internal/domain` adds private, validated
snapshots for attempts, capacity reservations and terminal attempt results.
`AttemptIdentity` keeps the run/work identity and positive ordinal separate
from worker, fence, reservation, time and record-version fields. Job scopes are
constructed from an authoritative plan and job; analysis scopes preserve both
pending revision-resolution and resolved analysis contexts. Active checks use a
separately supplied ownership binding and reject stale fences, versions,
workers, classes, reservations and times. Reservation records describe
capacity state and correlation; they do not allocate resources or imply a pod.

Producer-bound artifact, report and gate records retain the producer attempt,
run/repository/revision/configuration scope and structured storage identifiers.
Consumed artifacts retain their original producer across retries. Passed gates
require retained evidence, while absent required evidence is projected as
`unknown` by a pure observation helper. Selection and telemetry headers carry
only bounded identity, version, sequence and reference fields; they declare no
payload support, execution authority or telemetry completion.

`AttemptResult` preserves the exact signed 32-bit process exit observation,
independent gate state, optional telemetry completeness and bounded diagnostics.
Its canonical digest and strict codec support exact terminal replay comparison.
Diagnostics require a worker-bound versioned redaction contract; the domain
package validates that contract binding and shape but does not prove that a
producer found every secret. Result validation and replay comparison are pure;
they do not perform persistence, artifact verification, downloads, BEP
parsing, metrics, retry scheduling, gate release or external mutation.

## 6. Schema compatibility and version history

These versions have different purposes and must be recorded separately:

| Version | Governs | Replay rule |
|---|---|---|
| Domain/message schema version | Field names, types, required/optional fields and queue envelopes | Accept only supported versions; reject incompatible or ambiguous input. |
| Fingerprint version (`F1`) | Normalized projection and digest construction | Compare under the stored version; do not silently rehash an old record. |
| Plan schema version | Canonical plan fields, ordering, dependencies and digest | A plan consumer must support the recorded version or return an explicit incompatibility. |
| Operation-key version | Semantic inputs to the logical operation key | Retries preserve the recorded version; a new version is a deliberate compatibility change. |
| Attempt/result/evidence schema version | Attempt ownership, producer-bound evidence, terminal facts and reference-only extension headers | Accept only supported contract versions; preserve producer identity, fences, digests and independent observations during replay. |
| Policy/configuration version | Admission, revision, trust, limits, mapping and gate decisions | Replay uses the stored decision; new requests use the then-authorized version. |
| Storage schema version | Physical representation and conditional-write support | Migrations preserve identity, receipts, fences and history across restart/restore. |

Compatibility requirements:

1. Additive fields may be ignored only when they are outside the versioned
   semantic projection and do not affect authentication, identity, policy or
   execution. Security-sensitive duplicates are rejected.
2. Incompatible messages, unknown required versions and unsupported fingerprint
   rules are rejected or held for reconciliation. They are not silently
   coerced into an older shape or empty plan.
3. Storage changes use an expand/backfill/contract sequence with an explicit
   compatibility window. A migration must preserve delivery deduplication,
   original outcomes, outbox message IDs, plan digests, result receipts and
   fencing semantics.
4. A restore must establish a new store/recovery epoch or equivalent fence
   invalidation so a worker holding pre-restore authority cannot renew or
   finalize work. Restored data may retain history without reauthorizing old
   active leases.
5. A document edit or code rollback does not reverse an adopted identity
   contract or erase records. Any change to participating fields,
   canonicalization, operation semantics or outcome meaning is a new
   decision/version with migration and compatibility evidence.

## 7. Atomic write groups

Each row below is one logical all-or-none store transaction. The exact backend
primitive remains a D1 decision. A response that is lost after the store may
have committed is `unknown` until the same stable key is read or replayed
against the authoritative store. No group may report success or acknowledge
dependent work based only on a client timeout.

| Atomic group | Records and preconditions | Confirmed result | Lost-response read/replay |
|---|---|---|---|
| **Admission** | New `DeliveryKey` with its `F1` projection, original outcome/policy/evidence references, one `RunID`, one analysis `WorkReferenceID`/outbox item and any authoritative quota reservation. Unique-key check, fingerprint comparison, registration binding and quota preconditions are all part of the commit. | New admission, or existing identical admission. A duplicate creates no run, outbox item or extra quota. A conflicting projection changes nothing. | Read by `DeliveryKey`. Equal projection returns the stored outcome/run; different projection returns conflict; unavailable or collision remains unknown. |
| **Deliberate rerun** | One `RerunRequestKey`, authorized source run/delivery link, new run, pinned or explicitly selected revision/configuration intent, one analysis outbox item and applicable quota. The request key must be unique under its authority scope. | New rerun or existing identical rerun request. Changed intent under the same key conflicts. | Read by `RerunRequestKey`; return the stored new run or conflict. Do not mint another run because a response was lost. |
| **Revision resolution** | Verified resolution/evidence binding, resolver version/strategy, policy/configuration identity and a conditional advancement of the run/context from pending to resolved or failed. Expected `RecordVersion`, cancellation version and active analysis fence must match. | One complete resolved or classified failed/pending outcome; no partial resolved context. | Read by run/revision and resolver operation key. If the evidence binding is present, return it; if absent but the read is inconclusive, remain unknown and do not compile a plan. |
| **Plan attachment** | Immutable `PlanID`/digest, ordered jobs, dependency edges, provenance, run attachment, expected run/cancellation versions and resolved revision/configuration binding. All graph validation occurs before the write. | New plan attachment or exact idempotent replay. Conflicting content, cycles or stale/canceled run commits nothing. | Read by `RunID`/`PlanID`. Equal digest confirms the plan; a different digest is conflict; unavailable state remains unknown. |
| **Eligible release/retry** | Conditional job transition to eligible/ready, retry classification or next-attempt authorization plus one stable outbox/work reference. Preconditions include dependency/gate state, policy, cancellation version, run/job versions and retry budget. | One authorized release/retry and one durable publication intent, or a rejected stale decision. | Read by `OperationKey`/job and release version. Existing intent confirms the release; absent state with an inconclusive read does not authorize a second release. |
| **Claim/renew** | Conditional eligible ownership, `AttemptNumber`, worker/execution-class identity, `FenceToken`, expiry and work-state update. Broker delivery is only an input; it is not the claim. | One current owner/fence; stale, wrong-class, expired or canceled claims are rejected. Renewals advance only the current fence. | Read by job/attempt/fence. A matching fence confirms ownership; otherwise the worker must stop and may not finalize. |
| **Result/evidence finalization** | Active fence, attempt identity, classified result, bounded redacted diagnostics, artifact/gate/evidence references and the conditional work-state advancement. Required evidence and cancellation ordering are checked in the same write. | New terminal result or exact authenticated replay of the same terminal result. Altered result conflicts; missing required evidence remains unknown. | Read by operation/attempt/result identity. Matching terminal receipt confirms the result and permits queue acknowledgment; an inconclusive read withholds acknowledgment. |
| **Cancellation** | Versioned cancellation intent, run/job invalidation of future release/claim eligibility and expected state versions. Cleanup references may be recorded, but pod/process termination is not part of this transaction. | Persisted cancellation intent; repeated cancellation is idempotent. A terminal accepted result is retained and cannot be resurrected. | Read by run/job/cancellation version. Persisted intent confirms the request; cleanup is reconciled separately. A timeout does not prove that work stopped. |
| **Outbox publication bookkeeping** | Conditional publication ownership/status, stable `MessageID`/work reference and broker receipt when confirmed. Store state never marks publication confirmed from an attempted send. | Confirmed publication, known failure/retryable pending, or uncertain publication. | Read by work reference/message ID and, where supported, broker replay/query. Until confirmation or authoritative reconciliation, status remains uncertain/pending and no dependent acknowledgment is inferred. |
| **Capacity reservation bookkeeping** | Conditional class-scoped reservation/ownership/state update, resource profile, reservation identity, expiry and cancellation/version preconditions. | One durable reservation or a visible capacity rejection; no pod creation is implied. | Read by reservation identity and observed reconciliation state. A lost response does not create another reservation or assume a pod exists. |

Store transactions do not include broker publication, pod creation, checkout,
artifact upload, registry push, deployment mutation or process termination.
Those operations consume durable intent, stable identity and policy-bound
credentials, then report receipts or uncertain outcomes back through a
conditional state operation.

## 8. Commit, response and acknowledgment boundaries

An operation may return success or acknowledge an upstream delivery only after
the authoritative state transition it depends on is confirmed. The boundaries
are:

| Boundary | Success/ack condition | Must remain pending or unknown when |
|---|---|---|
| Webhook admission | New admission transaction committed, or an exact existing admission was read and verified. | The store is unavailable, the commit response is lost and unreconciled, the identity is invalid, or the replay conflicts. |
| Analysis work delivery | Candidate/resolution evidence and the next durable state/outbox transition are committed before acknowledging the work message. | Resolution or state publication is uncertain, required revision context is missing, or the worker loses its fence. |
| Job execution delivery | A fenced terminal result or an exact stored terminal receipt is confirmed before queue acknowledgment. | Result commit response is lost, the fence is stale, required evidence is unavailable, or the external effect is uncertain. |
| Outbox publication | The broker confirms the stable message and the store records that confirmation. | Broker confirmation or bookkeeping is uncertain. An attempted send is not confirmation. |
| Cancellation request | Cancellation intent and eligibility invalidation are durably committed. | The store write is uncertain. Process/pod cleanup is still reconciliation work. |
| External Push/Deploy | The adapter records a destination receipt or verified observed state under the stable operation identity. | The request may have reached the destination but no receipt/state proof exists. The outcome is `needs-reconciliation`, never an automatic duplicate mutation. |

An acknowledgment or HTTP response can be lost after the commit. The caller
must retry with the same stable identity and let the store return the
confirmed new or existing result. A deadline or cancellation sent after a
commit request does not prove rollback. The implementation must not manufacture
success, assume rollback, generate a replacement identity or delete a durable
intent merely because the client did not receive the response.

### 8.1 Commit and lost-response sequence

```mermaid
sequenceDiagram
    participant C as Caller
    participant S as Authoritative store
    participant Q as Queue or provider
    C->>S: Commit using stable key
    S->>S: Validate preconditions and atomically write
    S--xC: Response lost or deadline expires
    C->>S: Read/replay using the same stable key
    alt Stored commit found
        S-->>C: Confirmed new or existing outcome
        C->>Q: Acknowledge only after confirmation
    else No authoritative answer
        S--xC: Read/reconciliation unavailable
        C-->>Q: Withhold success/ack; retain unknown state
    end
```

The sequence applies to admission, rerun receipts, plan attachment, release,
claim, result finalization and outbox bookkeeping with their respective stable
keys. For an external effect, a local read is insufficient unless it includes
the destination receipt or verified observed state.

## 9. Unknown outcomes and recovery taxonomy

Every state operation has an explicit outcome class:

| Outcome | Meaning | Caller behavior |
|---|---|---|
| `confirmed_new` | The transaction committed a new record or transition. | Return/ack only after the commit is confirmed. |
| `confirmed_existing` | The exact stable key and semantic content already committed. | Return the stored result; do not create another identity or side effect. |
| `conflict` | The stable key exists with different semantic content or incompatible version. | Preserve existing state and surface the conflict. |
| `invalid` | Input or binding is malformed, unsupported or unauthorized before commit. | No admission or side effect; apply the public policy mapping. |
| `stale_or_fenced` | Expected version, cancellation version, lease or fence is no longer current. | Stop the stale actor and reread authoritative state. |
| `known_precommit_failure` | The store proved that no transaction committed. | Retry only with the same stable identity if policy permits; otherwise report failure. |
| `unavailable` | The store or required reconciliation source cannot answer. | Withhold success/ack and retain a visible retry/reconciliation state. |
| `indeterminate_commit` | The request may have committed, but the response or immediate proof was lost. | Treat as unknown; read/replay the same key before any new attempt. |
| `needs_reconciliation` | An external operation may have occurred without a receipt or verified observation. | Do not blindly repeat a mutation; reconcile destination state under the same operation identity. |

### 9.1 Failure-point matrix

| Failure point | Permitted conclusion | Required recovery |
|---|---|---|
| Before a store transaction begins | No commit is known. | Retry with the same stable key if the input remains valid and policy permits. |
| Validation or precondition rejection | No records from that group change. | Surface invalid/conflict/stale outcome; do not retry as a different identity. |
| Store proves abort before commit | The group did not commit. | Retry or report failure with the same key; preserve the explicit reason. |
| Commit succeeds but response is lost | Outcome is indeterminate, not rolled back. | Read/replay by the same key; return/ack only after the stored result is verified. |
| Authoritative read/replay is unavailable | Outcome remains unknown. | Keep work pending or needs-reconciliation; do not return success, ack or create a new identity. |
| Read finds equal existing content | The original group committed. | Use the stored result and continue the dependent acknowledgment boundary. |
| Read finds different content | Conflicting replay. | Leave existing records unchanged and surface conflict. |
| External request sent without receipt | External outcome is uncertain. | Reconcile by destination and stable operation identity; never assume failure or issue an unbounded duplicate mutation. |
| Cancellation/deadline after sending a commit | Neither commit nor rollback is proven by the cancellation. | Reconcile state. A committed result remains durable; a pending operation remains unknown. |
| Lease/fence lost before finalization | The worker no longer has write authority. | Stop execution, classify the attempt through the store and prevent stale finalization. |
| Store restore after active work | Historical state may be present but old authority is not valid. | Advance restore/recovery fencing epoch and require fresh claims. |

## 10. Acceptance and contract matrix

The following cases are the minimum documentation contract for A1.01 and the
A1/A2/A3 boundary. A future implementation must prove them with isolated
state-adapter fixtures and synthetic provider/queue inputs. No live webhook,
queue, Kubernetes, registry, cache or deployment call is part of this task.

| Scenario | Trusted inputs and stable key | Expected durable records/outcome | Permitted ack or side effect | Reconciliation |
|---|---|---|---|---|
| **New delivery** | Valid provider authentication evidence, trusted `RegisteredRepositoryID`, validated `DeliveryKey` and `F1` projection. | One delivery record, one run, one analysis outbox item and one quota reservation if configured. | Webhook success after commit; no direct queue bypass. | Read by `DeliveryKey` after a lost response. |
| **Identical semantic replay** | Same `DeliveryKey`; equal `F1` projection despite whitespace, key order or irrelevant additive fields. | Original delivery, policy decision, run and outbox remain the only records. | Return original outcome/run; no new quota, run or work. | Store comparison under the recorded fingerprint version. |
| **Conflicting replay** | Same `DeliveryKey`; changed head/base/action/source repository or other participating field. | No mutation to the original records; explicit conflict. | No success or new work. | Preserve the original projection and outcome for audit. |
| **Deliberate rerun** | Authorized new `RerunRequestKey`, source run/delivery link and explicit revision/configuration intent. | One rerun receipt, new linked run, new analysis outbox and applicable quota. | New run may be accepted; replay of the same rerun key returns it. | Read by rerun key; changed intent conflicts. |
| **Missing revision resolution** | Admission exists, but base/head object or required resolver evidence is unavailable. | Pending or failed resolution with classified evidence; no executable plan. | No plan release and no complete-no-work success. | Retry/reconcile resolver by its stable operation key. |
| **Wrong repository or integration** | Payload/provider identity, PR target or provider repository ID does not match trusted registration. | No delivery admission, run, quota reservation or outbox. | Reject/ignore according to A2 policy; no side effect. | Recheck trusted registration only; never self-register from payload. |
| **Commit response lost** | Any store group submitted with a stable key; response or connection is lost. | Unknown until authoritative lookup; no speculative duplicate. | Withhold success/ack until read/replay proves new/existing commit. | Read/replay same key; unavailable reconciliation remains unknown. |
| **Concurrent duplicate at quota limit** | Concurrent equal `DeliveryKey`/`F1` requests, possibly after quota is full. | One admission and one quota reservation; duplicate does not consume another slot. | The winner returns success; exact duplicate returns stored outcome. | Unique key and quota precondition are one atomic group. |
| **Stale fence or committed cancellation** | Worker has old `FenceToken` or cancellation version has won the conditional race. | Stale write rejected; cancellation intent and accepted history retained. | No new release or result finalization under stale authority. | Reread current state; obtain a new claim only if policy permits. |
| **Identical plan replay** | Same `PlanID`, schema and canonical digest. | One immutable plan, jobs and dependencies; repeated attachment is idempotent. | No second publication merely from replay. | Read plan digest and attachment state. |
| **Changed plan under same identity** | Same `PlanID` with changed job, dependency, revision, gate or policy content. | Conflict; no partial replacement or release. | No queue or execution side effect. | Explicit replacement/rerun contract required. |
| **Identical terminal result replay** | Same attempt/fence/result identity and exact bounded evidence references/content. | Original terminal result remains authoritative. | Queue acknowledgment after stored terminal receipt is confirmed. | Read by attempt/operation key. |
| **Changed terminal result replay** | Same attempt identity but altered outcome, artifact, gate or evidence. | Conflict; prior terminal result unchanged. | No terminal resurrection or altered acknowledgment. | Escalate to reconciliation/audit. |
| **External outcome uncertain** | Stable Push/Deploy operation key with possible destination receipt but no proof. | Local state is `needs-reconciliation`; no automatic second mutation. | No success claim and no blind retry. | Query/observe destination with scoped credentials and record receipt. |

## 11. A1, A2 and A3 handoff

### A1 owns

- versioned domain identities and normalized replay comparison;
- delivery/rerun receipts and original policy decisions;
- conditional state operations and the atomic groups in section 7;
- immutable revision, plan, job, dependency, attempt and evidence bindings;
- record versions, cancellation versions, fences and recovery outcomes;
- durable outbox intent and publication bookkeeping;
- adapter conformance evidence for duplicates, conflicts, response loss,
  cancellation and restore fencing.

A1.01 specifies these contracts. A1.03–A1.06 refine the admission, plan,
attempt/result and lifecycle transition details. A1.07–A1.14 implement and
verify them only after D1 backend, schema and restore decisions are selected.

### A2 owns

- bounded request intake and raw-byte preservation for verification;
- provider-specific signature/authentication checks;
- field-specific provenance, registration binding and normalized push/PR facts;
- supported event/action policy, replay retention, limits and HTTP outcome
  mapping;
- policy decisions for missing/invalid candidate revision context;
- the A2.2 seam into A1 admission.

A2 must consume this document's `DeliveryKey` and `F1` rules. Its current
body-digest wording in A2.2 is an alignment prerequisite: a raw-body digest
may be separate evidence, but semantic normalized content is authoritative for
same-ID replay. A2 must also address the threat of a captured signed body being
resent under a different delivery ID; this identity contract cannot solve that
threat by itself.

### A3 owns

- outbox publication and broker-specific confirmation/retry;
- ready-only scheduling and conditional release of eligible jobs;
- class-bound claim/ack behavior, queue backpressure and bounded shutdown;
- capacity reservations, Kubernetes reconciliation and pod lifecycle;
- revalidation of dependencies, policy, cancellation and fences before work;
- runner result submission and queue acknowledgment after durable result state.

A3 must treat the state store as authoritative. A queue message or pod
observation cannot grant execution authority, and publication/pod creation
cannot be folded into an A1 store transaction. The detailed A3 milestone draft
was unavailable for this task; its boundaries are checked against sections
2–4 and 10 of the tracked delivery plan and must be revisited when the draft
exists.

## 12. Open prerequisites and decision gates

These gates do not block this backend-neutral contract document. They block
consuming implementation or determine its evidence:

| Gate | Owner/consumer | Unresolved choice or evidence |
|---|---|---|
| `D1` | A0 integration owner; A1 adapter | State backend and broker, transaction/conditional-write guarantees, durability, retention, failover/restore and uncertain-commit behavior. |
| `D2` | A2 policy owner | Provider instance/integration model, supported events/actions, revocation and rotation, replay retention, and captured-body replay under a different delivery ID. |
| `D3` | A1/A2/A3 implementers | Pinned Go, provider, broker, Kubernetes, detector and rules/tool versions. |
| `D4` | A2 policy and revision resolver | Exact push/PR/initial/force/deletion/base strategies and required object-resolution evidence. |
| `D7` | A1/A3 capacity owner | Numeric quota, resource, concurrency, retry, lease, timeout, expiry and retention limits with authoritative time semantics. |
| `D9` / `D11` | Deployment and gate owners | Required gates, destination approval, external receipts, environment concurrency and deployment reconciliation. |
| A2.2 alignment | A1/A2 owners | Replace body-digest-as-semantic-fingerprint wording with `F1` normalized content plus separate raw request evidence. |
| A3 detail | A3 owner | Ready/retry/publication/capacity transitions and their exact queue/broker contracts. |

No unresolved gate may be represented as an implemented behavior. A future
decision record must identify its owner, version, affected contract IDs,
compatibility/migration requirements and evidence before dependent runtime work
claims completion.

## 13. Decision and change history

| Version/date | ID | Change | Status and compatibility note |
|---|---|---|---|
| `0.1.0` / 2026-10-03 | `ST-001`–`ST-008` | Initial A1.01 contract: delivery namespace, `F1` semantic replay, revision representations, immutable identities, atomic groups, commit-before-ack and unknown-outcome recovery. | Proposed for review. No runtime implementation or adopted storage schema exists. |
| `0.1.0` / 2026-10-03 | `CHG-001` | Recorded the A2.2 alignment prerequisite: raw-body digest is separate request evidence; normalized semantic content decides same-delivery replay. | No external milestone draft was edited. A2 implementation must resolve this before consuming the contract. |
| `0.1.1` / 2026-10-03 | `CHG-002` | Corrected the admission-evidence retention rule to prohibit durable raw requests, signatures, authorization values and secrets. | Review correction aligned with A2's bounded authentication lifetime and credential-free durable records. `F1` and delivery identity are unchanged. |
| `0.2.0` / 2026-10-05 | `A1.04` | Added versioned immutable plan, job, analysis and bounded work-reference domain contracts with typed operation/class validation, canonical projections, dependency/provenance checks and strict codecs. | Domain implementation is local and backend-neutral. Existing F1, event, revision and run rules remain unchanged; consumers must support the recorded v1 contract versions. |
| `0.3.0` / 2026-10-08 | `A1.05` | Added bounded attempt and capacity-reservation records, producer-bound artifacts/reports/gates, unknown required-gate observations, reference-only selection/telemetry headers and immutable attempt results with exact exit, telemetry, diagnostics and replay contracts. | Pure domain implementation is local and backend-neutral. No allocator, lease lifecycle, artifact verifier, BEP parser, metrics consumer, retry scheduler, gate release or external adapter is present; future payload formats require their own versioned review. |

An approved change must add a new history row and, when behavior changes, a
new contract or fingerprint/schema version. Editing prose without preserving
the prior decision is not a rollback mechanism.

## 14. Scope and evidence boundary

The A1.01 state contract remains backend-neutral. The A1.04–A1.05
implementation adds pure domain records and does not implement:

- database tables, migrations or storage adapters;
- control-plane adapters beyond the pure `internal/domain` package;
- webhook routes, provider authentication or policy evaluation;
- queues, outbox loops, schedulers, workers, Kubernetes objects or leases;
- Git/Bazel revision resolution, target detection or plan compilation;
- cache writes, artifact verification/download or retention, registry pushes or deployments;
- BEP parsing, metrics, retry scheduling, gate release or concrete selection/
  telemetry payload interpretation;
- live webhooks, queue publication, pod creation, external mutations or
  executable product tests.

The completion evidence for this task is document review: the identities,
provenance, fingerprint rules, revision states, immutable plan/operation
semantics, atomic write groups, acknowledgment boundaries, recovery taxonomy,
acceptance matrix, A1/A2/A3 ownership and unresolved gates are present and
internally consistent. Backend durability, provider authentication and
operational recovery remain unverified until their consuming tasks provide
isolated evidence.
