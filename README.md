# keystone

Keystone is the selected product name for delta-aware CI orchestration for
Bazel monorepos. This checkout contains a root Go module with domain contracts
and tests, but no runtime implementation or Bazel targets yet. The planned
roadmap covers a Go control plane, queued jobs, ephemeral runners, hybrid
target selection, daemonless OCI assembly, caching, BEP telemetry and quality
gates.

Contributor workflows live in the repository AI Harness:

- [Harness usage and local checks](docs/ai-harness/README.md)
- [Harness provenance](docs/ai-harness/provenance.md)

## Local checks

Keystone uses [pre-commit](https://pre-commit.com/) for contributor checks.
The configuration runs public hygiene hooks, `gofmt`, `go vet`, and `go test`
from the repository root. Commit messages use the Conventional Commits
rules through commitlint.

Install the prerequisites and both Git hook stages in a fresh clone:

~~~sh
go version
python3 --version
node --version
python3 -m pip install pre-commit
pre-commit install --hook-type pre-commit --hook-type commit-msg
~~~

The current configuration requires Go 1.27.1, Python 3.9 or newer,
pre-commit 3.2.0 or newer, and Node.js 22.12 or newer for the commitlint
environment. Run the complete file-stage check from the repository root with:

~~~sh
pre-commit validate-config
pre-commit run --all-files --hook-stage pre-commit
~~~

The commit-message stage can be exercised with a disposable message file:

~~~sh
pre-commit run commitlint --hook-stage commit-msg \
  --commit-msg-filename /path/to/message-file
~~~

The `commit-msg` stage also rejects empty or comment-only messages before they
can bypass commitlint's empty-input handling.

Conventional Commit types include `build`, `chore`, `ci`, `docs`, `feat`,
`fix`, `perf`, `refactor`, `revert`, `style`, and `test`. Scopes are optional,
breaking changes use the usual `!` or footer syntax, and headers are limited
to 100 characters. Commitlint retains its default ignores for generated merge,
revert, fixup, and squash messages.

Formatting hooks can modify files and stop the commit. Review the diff, stage
the corrections, and run the hook again before committing. Go vet and Go test
run once for every pre-commit invocation, including documentation-only
changes, and use `-mod=readonly`.

These are local hooks. Each clone needs its own installation, contributors
can bypass them, and this repository does not yet have a server-side workflow
that enforces them for every remote commit.
