# Contributing to Muxio

[中文](CONTRIBUTING.md) | **English**

Muxio is documentation-first and applies the same collaboration rules to humans and agents.

## Development environment

Only Go 1.25 or newer is currently required. The Web toolchain will be added after its implementation spec is accepted.

```sh
git clone git@github.com:ConteMan/muxio.git
cd muxio
make bootstrap
make check
```

## Before starting

1. Follow the reading order in [AGENTS.md](AGENTS.md): project map, design, ADRs, roadmap, and specs.
2. Search existing Issues and add evidence to the original Issue instead of opening duplicates.
3. Use `enhancement` for small work inside an existing spec and roadmap. Use `spec-needed` for larger work that requires a new contract and maintainer direction.
4. Architecture, data model, configuration, public CLI, and API changes must update their durable contracts in the same PR.

## Branches and commits

Create `feat/<name>`, `fix/<name>`, `docs/<name>`, or `chore/<name>` from `main`. Commits follow Conventional Commits. Commit and PR prose is maintained in Chinese; code, identifiers, paths, type, and scope remain in English.

Keep one logical change per commit and pull request.

## Pull requests

- Link an Issue with `Closes #N`, or explain why no Issue is needed.
- Describe the change, reason, validation evidence, risks, and rollback.
- Run `make check` before submission; red CI must not be merged.
- Record user-visible changes in `CHANGELOG.md`.
- Update the Chinese and English mirrors together when changing the main README or contribution guide.

## Dependencies

Do not add dependencies for hypothetical future needs. A new direct Go dependency or Web toolchain must be justified in a spec or ADR with its purpose, maintenance status, and why the standard library or existing capabilities are insufficient.

## License

By contributing, you agree that your contribution is licensed under the [MIT License](LICENSE).
