---
name: "verify"
description: "Check a Keystone CI or Harness change and report exact results and unavailable checks."
---
<!-- SPDX-License-Identifier: MPL-2.0 -->

Read .agents/skills/verify/SKILL.md. Inspect the requested diff and available
manifests, run applicable local checks, and report commands, results and gaps.
For affected product contracts, consult
.agents/skills/keystone-validation/SKILL.md. Do not invent application checks
when no module or target exists. Verification is read-only by default; local
fixes or external actions require task authorization.
