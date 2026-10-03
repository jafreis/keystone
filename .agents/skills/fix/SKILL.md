---
name: fix
description: Reproduce and repair a Keystone CI defect with a focused regression check and explicit boundary evidence.
---

# Fix workflow

Inspect the current diff and preserve unrelated edits. Reproduce the defect
with the smallest synthetic fixture or deterministic command, trace the input
to the first incorrect state and explain the cause before patching.

For webhooks, queueing, Kubernetes lifecycle, runner execution, target
selection, OCI, caching, BEP or quality gates, use [CI validation](../keystone-validation/SKILL.md)
to select the affected contract. Separate missing base/hash context, upstream
unavailability, incomplete telemetry and an incorrect local result; each needs
a different repair.

Add a regression check that fails for the original cause, make the smallest
safe fix and rerun the relevant checks. For concurrency, subprocess or
temporal defects, control ordering, cancellation and the clock rather than
relying on sleeps or live infrastructure. A runner defect does not require a
registry push to prove the repair.

If reproduction is unavailable, report the gap and narrow the investigation;
do not claim a verified fix. Do not weaken validation, widen credentials,
interpolate repository text into a shell, turn detector failure into an empty
plan or suppress partial BEP errors to make the symptom disappear.
