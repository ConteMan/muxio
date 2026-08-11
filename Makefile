.PHONY: bootstrap fmt test vet build api-check check status web-install web-dev web-build panel-smoke

# Release identity comes from Git; there is no second hard-coded version.
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := github.com/ConteMan/muxio/internal/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) \
           -X $(VERSION_PKG).Commit=$(COMMIT) \
           -X $(VERSION_PKG).Date=$(DATE)

bootstrap:
	go mod download
	npm --prefix web ci

web-install:
	npm --prefix web ci

web-dev:
	npm --prefix web run dev

web-build:
	npm --prefix web run build

panel-smoke:
	./scripts/panel-smoke.sh

fmt:
	gofmt -w cmd internal

test:
	go test -race ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/muxio ./cmd/muxio

api-check:
	go run github.com/getkin/kin-openapi/cmd/validate@v0.142.0 -multi api/openapi.yaml

check:
	./scripts/selftest.sh

status:
	./scripts/status.sh
