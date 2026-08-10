# Muxio

[中文](README.md) | **English**

Muxio is a local-first personal information capture core. Its first goal is to collect information reliably from local devices, LAN sources, websites, and public APIs while keeping every record searchable, traceable, and exportable.

> Status: the maintainable project foundation is in place; capture and SQLite storage are not implemented yet. See the [Roadmap](docs/roadmap.md) for scope.

## Principles

- Reliable capture comes before content understanding and automation.
- One Go binary, SQLite, and a local HTTP API.
- Modular connectors without freezing an external plugin protocol too early.
- Core and Web share a repository but remain independent projects joined only by OpenAPI.
- Personal data stays local by default, and the service listens on loopback only.

## Why not an existing tool

An RSS reader solves subscriptions, a note plugin solves one source, and a script can get one collection run working. Each is valid on its own, but none provides a unified record substrate across sources: one set of idempotency, versioning, traceability, and run-observability semantics covering every source.

Muxio's first phase builds only that substrate. Higher-level capabilities — search, content understanding, automation — are built on top of it instead of being redone for each new source.

This is also the scope test: a feature is worth building only if it strengthens the substrate.

## Quick start

Go 1.25 or newer is required.

```sh
make bootstrap
go run ./cmd/muxio version
go run ./cmd/muxio serve
```

Verify from another terminal:

```sh
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/v1/status
```

Import records and confirm that repeating an import creates no duplicates:

```sh
muxio db path
echo '{"external_id":"note-1","title":"Title","body":"Body"}' | muxio import --source notes
echo '{"external_id":"note-1","title":"Title","body":"Body"}' | muxio import --source notes
```

The second run reports `imported=0 duplicate=1 failed=0`. The database lives in
the platform data directory by default; `MUXIO_HOME` overrides it.

Review past runs, and find which line failed in one of them and why:

```sh
muxio runs
muxio runs show 1
```

Run the complete quality gate:

```sh
make check
```

## Repository map

- `cmd/muxio/`: entry point for the Go binary.
- `internal/`: core implementation, not a public SDK.
- `api/openapi.yaml`: public contract between the core and Web.
- `web/`: boundary for the independent Web project.
- `docs/`: design, ADRs, roadmap, and specs.
- `scripts/selftest.sh`: shared local and CI quality gate.
- `scripts/status.sh`: `make status`, global progress and pending decisions.

See the [project map](docs/MAP.md) and [architecture](docs/design/architecture.md) for details. The durable project documents are currently maintained in Chinese.

## Contributing

Read [CONTRIBUTING.en.md](CONTRIBUTING.en.md) and [AGENTS.md](AGENTS.md) before starting. GitHub Issues track work and progress; `docs/` holds durable contracts.

## License

[MIT](LICENSE)
