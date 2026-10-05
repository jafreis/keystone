<!-- SPDX-License-Identifier: MPL-2.0 -->

# Contributing to Keystone

Keystone uses the [Mozilla Public License 2.0](LICENSE) (`MPL-2.0`) for
project-owned contributions accepted into this repository. By submitting a
change, you confirm that you have sufficient rights to grant the permissions
required by that license, or that the change is clearly identified with the
applicable third-party terms before it is accepted.

## Before submitting a change

- Keep the change within the current repository scope. Product drafts and
  roadmap examples do not establish implemented packages, services or release
  artifacts.
- Add the SPDX marker `MPL-2.0` to new project-owned source files using the
  comment syntax for that file type. Preserve YAML frontmatter, shebangs,
  existing copyright statements and other accurate license notices.
- Do not copy code, documentation, templates or configuration from another
  project unless its permission is documented or the relevant rights holder
  has confirmed sufficient rights. Record retained third-party material and
  its terms in [the licensing record](docs/licensing.md) and
  [the provenance record](docs/ai-harness/provenance.md).
- Do not add dependencies or bundled artifacts without recording the exact
  version, license, transitive obligations, modification status and
  distribution boundary.

The repository does not require a CLA or DCO sign-off. Those are separate
governance choices and must not be inferred from this policy.

## Notices and source delivery

The repository's default notice is:

```text
SPDX-License-Identifier: MPL-2.0
```

Use the equivalent comment form for the file type. Keep the notice after YAML
frontmatter and after a required interpreter shebang. A file that contains
retained third-party material keeps its original notices and is listed as an
exception until its applicable terms are established.

When distributing covered source, retain the MPL notice and provide the
recipient a copy of [LICENSE](LICENSE). When distributing an executable form,
make the corresponding covered source available by a reasonable, timely
method and document that mapping for the release. The release gate for future
Keystone binaries and images is defined in [docs/licensing.md](docs/licensing.md);
this checkout contains no such product artifacts.

## Local checks

From the repository root, run:

```sh
git diff --check
gofmt -l internal/domain/*.go internal/domain/*_test.go
go test -mod=readonly ./...
go vet -mod=readonly ./...
pre-commit validate-config
pre-commit run --all-files --hook-stage pre-commit
```

Exercise commitlint separately with a disposable commit-message file, as
described in [the README](README.md). Local hooks can be bypassed and are not
server-side enforcement.
