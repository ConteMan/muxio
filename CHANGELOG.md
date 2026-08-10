# Changelog

Muxio follows [Semantic Versioning](https://semver.org/). User-visible changes are recorded here.

## Unreleased

### Added

- Initial maintainable monorepo foundation.
- Build identity injected from Git tags at build time via `make build`.
- `make status` prints a global view — milestones, spec and ADR state, and
  pending maintainer decisions — derived from repository state on every run and
  never written to disk, so it cannot go stale.
- A documented list of decisions an agent must stop and escalate on, in
  `AGENTS.md`.
- `selftest` now verifies that ADR and spec indexes are complete in both
  directions, and that the English README and contributing mirrors keep the same
  section structure as their Chinese sources.

- SQLite storage: an embedded migration framework, the `sources` and `captures`
  tables, and owner-only data directory resolution overridable with `MUXIO_HOME`.
- `muxio import --source <name>` reads JSONL capture records from stdin. Repeated
  imports are idempotent, changed content is kept as a new version alongside the
  old one, and an invalid line fails on its own without stopping the batch.
- `muxio db path` prints the database location without creating it.
- Run history: every import is recorded as a run with its status, timing and
  counts, and the reason each rejected line was rejected. `muxio runs` lists
  them and `muxio runs show <id>` explains one, so an import stays diagnosable
  after the process exits.
- A run abandoned by a killed process is marked interrupted once its heartbeat
  goes stale, without disturbing runs another process is still working on.
- Structured JSON logs on stderr, carrying the run id. `--log-level` and
  `MUXIO_LOG_LEVEL` select debug, info, warn or error.

### Changed

- The required Go toolchain is now `1.25` instead of the patch-level `1.25.9`.
- `cli.Run` takes stdin, since commands now read from it.

### Fixed

- `muxio serve` verifies that the address it actually bound is on the loopback
  interface, so a hostname resolving off loopback can no longer be served.
