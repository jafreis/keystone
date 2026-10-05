<!-- SPDX-License-Identifier: MPL-2.0 -->

# Keystone licensing and compliance

## Decision

Keystone uses the standard Mozilla Public License 2.0 (`MPL-2.0`). It fits a
reusable CI core that should preserve openness in distributed changes to
covered files while allowing separately developed integrations in a larger
work. Apache-2.0 and MIT remain reasonable permissive alternatives; AGPL-3.0
would add a hosted-service source-offer condition that the current product
requirements do not request. The choice is recorded here so the repository
default, contributor policy and future release checks stay aligned.

The canonical terms are in [LICENSE](../LICENSE). This record describes the
repository's application of those terms; it does not grant rights in material
that Keystone does not own or use under an applicable grant.

## Scope and current inventory

The repository default applies to project-owned source, tests, hooks,
configuration, documentation and Harness guidance after sufficient rights are
established. Eligible files carry an `SPDX-License-Identifier: MPL-2.0`
notice in a syntax valid for that file type. `LICENSE` is the unmodified
license document, apart from a final newline for repository hygiene.

| Material | Current evidence | Disposition |
| --- | --- | --- |
| `internal/domain/` Go source and tests | The current module contains domain contracts and tests and imports only the Go standard library. | Project-owned repository source; covered by the repository default. |
| `hooks/` Python scripts and tests | The scripts use Python standard-library modules only. | Project-owned repository tooling; covered by the repository default. |
| `.pre-commit-config.yaml`, `.commitlintrc.yaml`, `go.mod` | These files contain repository configuration and module identity. Pinned hook and commitlint references identify tools downloaded for local checks; they do not relicense those tools. | Repository configuration is covered; downloaded tools retain their own terms and are not bundled by this checkout. |
| Repository Markdown and Harness guidance | The files describe Keystone development and product scope. Their planned product references are documentation, not runtime components. | Project-owned documentation is covered unless listed as an exception. |
| Six retained/adapted role files | `.agents/agent-templates/{code-review,verify}.md`, `.agents/agents/{code-review,verify}.md` and `agents/{code-review,verify}.agent.md` were retained or adapted from a ClueMesh Harness. The provenance record identifies source commit `c20eb519b493103ef44766daa1d933263402f590`; the inspected source checkout did not provide a license-like file for this material. | Rights confirmation from the relevant maintainer or rights holder is still required. These files remain an explicit exception and are not blanket-marked as MPL until that disposition is recorded. Their shared bodies and host-specific metadata remain synchronized. |
| Owning-worktree product drafts | `docs/spec.md`, `docs/plan.md` and `docs/milestones/` are untracked in the owning worktree and absent here. | Outside this checkout and outside this license change. Do not copy or modify them as part of Harness work. |
| Future dependencies and release artifacts | No external Go module, vendored code, Bazel target, image or product binary exists here. | Review exact versions, licenses, notices, modifications, linkage or bundling and source delivery when such artifacts are introduced. |

Git authorship, a repository path or the absence of an upstream license does not
itself prove ownership or permission. The retained role files are therefore a
bounded rights question rather than a claim of infringement. A rights-holder
confirmation can resolve the exception without modifying the source Harness.

## MPL obligations

The compliance record uses these obligation identifiers:

- **O1 — Rights:** each contributor must have sufficient rights to make the
  license grant and retained upstream notices must be preserved.
- **O2 — Source:** covered source must identify the MPL terms and recipients
  must be told how to obtain the license.
- **O3 — Executables:** when covered executable forms are distributed, the
  corresponding covered source must be available by a reasonable, timely
  method and the release must tell recipients how to obtain it.
- **O4 — Scope:** files in a larger work may use other terms, but MPL notices,
  patent-grant boundaries and the absence of a trademark grant must remain
  clear. No Exhibit B incompatibility designation is attached to Keystone
  source.

These obligations follow MPL sections 2 and 3. README and contributor links,
SPDX markers, a rights ledger and release checks are repository practices that
make the obligations reviewable; MPL does not require these filenames, a
badge, a CLA, a DCO, a scanner or a hosted-service disclosure.

Keystone's license does not automatically relicense customer repositories,
pipeline outputs or external services. Invoked tools, independently deployed
services, copied code, linked libraries and bundled executables must be
reviewed according to their actual distribution boundary.

## Dependency and distribution review

The current source tree has no external Go or Python library dependency. The
pre-commit configuration names externally fetched hooks and commitlint tools;
those references are contributor tooling and are not a distribution of their
source in this repository. Future reviews must record exact versions and
transitive components before approving client libraries, Bazel rules, cache or
queue clients, linters, runner base images or registry tooling.

In particular, do not assume a roadmap mention approves a dependency. Redis
terms vary by selected version and license option, and a linter or base image
may carry obligations independent of Keystone. Keep remote action/CAS policy,
repository dependency caching and third-party artifact licensing as separate
reviews.

## Release gate

No Keystone executable or OCI image is present in this checkout, so artifact
compliance is not yet assessed. Before distributing an actual release,
maintainers must:

1. inventory the exact artifact contents and included third-party terms;
2. map covered files and modifications to the corresponding source revision;
3. include `LICENSE` and required original notices;
4. provide corresponding covered source through an accessible location tied to
   the exact distributed revision; and
5. verify source access from a recipient's perspective before release.

Missing source, unknown component terms or absent notices blocks that artifact's
release. Container metadata labels alone are not evidence of source delivery.
Customer pipeline outputs keep their own licensing responsibilities.

## Verification record

For the current source-only checkout, the bounded verification commands are:

```sh
git diff --check
gofmt -l internal/domain/*.go internal/domain/*_test.go
go test -mod=readonly ./...
go vet -mod=readonly ./...
pre-commit validate-config
pre-commit run --all-files --hook-stage pre-commit
pre-commit run commitlint --hook-stage commit-msg \
  --commit-msg-filename /path/to/disposable-message-file
```

These checks establish repository consistency and current source behavior. They
cannot prove ownership, third-party permission, editor discovery, host schema
support, model behavior or compliance of nonexistent release artifacts. The
current source-tree conclusion is complete for project-owned files after the
listed notices are applied, and remains conditional for the six retained role
files until their rights disposition is confirmed.
