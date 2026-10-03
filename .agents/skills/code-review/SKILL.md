---
name: code-review
description: Review a local Keystone CI or Harness diff for correctness, security and maintainability without editing or publishing.
---

# Code review

Read [review guidelines](../../REVIEW_GUIDELINES.md). Determine whether the
scope is unstaged, staged, a branch diff or a supplied patch. Include untracked
files only when they belong to the requested change and derive the base ref
instead of assuming a branch name.

Read changed files with callers, tests, configuration and relevant contracts.
For product behavior, consult [CI validation](../keystone-validation/SKILL.md)
and only the affected scenario sections. For Harness-only changes, check
routing, reference resolution, authority boundaries and template/adapter parity
without prescribing product infrastructure.

Report high-confidence findings with category, severity, trigger, consequence
and remediation. Distinguish a confirmed defect from a missing check or an
unresolved architecture choice. Include no-change behavior, missing base/hash
context and unavailable tools when they affect the verdict.

Do not silently fix code, post comments, approve, merge, push, publish a review
or call live providers as part of a local review.
