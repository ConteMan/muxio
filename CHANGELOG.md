# Changelog

Muxio follows [Semantic Versioning](https://semver.org/). User-visible changes are recorded here.

## Unreleased

### Added

- Initial maintainable monorepo foundation.

### Fixed

- `muxio serve` verifies that the address it actually bound is on the loopback
  interface, so a hostname resolving off loopback can no longer be served.
