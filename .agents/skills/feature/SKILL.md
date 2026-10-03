---
name: feature
description: Implement a new Keystone CI capability or substantial behavior change from requirements through focused verification.
---

# Feature development

Identify the requested outcome, affected sprint and delivery stage first. The
roadmap describes future control-plane, runner, change-detection, cache and
telemetry work; it does not authorize implementing every adjacent capability.

1. Read current instructions, the relevant product drafts when available and
   the diff. Separate established requirements from illustrative commands,
   proposed paths and unsettled backend, broker, trust or retry choices.
2. Define observable acceptance criteria and the smallest component boundary.
   For webhooks, jobs, runners, target selection, OCI, caching, BEP or aspects,
   read [CI validation](../keystone-validation/SKILL.md) and the matching
   scenario section.
3. Map inputs, interfaces, dependencies, failure behavior and permission
   boundaries. Record consequential unresolved decisions; make routine choices
   within the existing authorization.
4. Add a focused failing check for changed testable behavior, then implement.
   Prefer fake clients, synthetic payloads, deterministic clocks and isolated
   subprocess or filesystem fixtures over live services.
5. Exercise boundary and cancellation behavior and the documented local
   checks. Review the final diff for scope, credential handling and external
   side effects.

Report the behavior delivered, files changed, checks/results, unavailable
checks and remaining decisions. Do not claim a target, image, cache, registry,
Kubernetes or editor integration exists from instructions or mocked tests.
