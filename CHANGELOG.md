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

### Changed

- The required Go toolchain is now `1.25` instead of the patch-level `1.25.9`.

### Fixed

- `muxio serve` verifies that the address it actually bound is on the loopback
  interface, so a hostname resolving off loopback can no longer be served.
