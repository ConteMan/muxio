.PHONY: bootstrap fmt test vet build api-check check

bootstrap:
	go mod download

fmt:
	gofmt -w cmd internal

test:
	go test -race ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/muxio ./cmd/muxio

api-check:
	go run github.com/getkin/kin-openapi/cmd/validate@v0.142.0 -multi api/openapi.yaml

check:
	./scripts/selftest.sh
