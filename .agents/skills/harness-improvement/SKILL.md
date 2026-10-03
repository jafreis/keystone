---
name: harness-improvement
description: Improve Keystone repository guidance, skills, templates or host adapters while preserving scope and user choices.
---

# Harness maintenance

Read root guidance, the current diff and
[Harness usage](../../../docs/ai-harness/README.md).
Determine which artifact owns the behavior: shared constraints in AGENTS.md,
workflow decisions in a skill, conditional CI detail in a linked reference,
review format in .agents/REVIEW_GUIDELINES.md or delegated role text in a
template. Keep one authoritative home.

Change instructions only when an explicit request, demonstrated mismatch or
recurring failure supports it. Preserve the distinction between current
checkout state, product requirements, roadmap examples and unresolved choices.
Do not turn one incident into a universal rule or confuse development skills
with runtime worker prompts and permission configuration.

For template edits, synchronize the body and shared name/description in
.agents/agents/ and .github/agents/. Retain valid host-specific metadata such
as Augment color and use host-default models. Never add an adapter without its
canonical template.

Check frontmatter, reference resolution, routing, authority boundaries and
template parity. Walk through clean/no-change input, unavailable application
checks, missing base/hash context and requested-only side effects. Update
[provenance](../../../docs/ai-harness/provenance.md) when source, product scope,
host support or exclusions change. Validator/generator tools, new host trees
and autonomous workflows remain deferred unless separately requested.
