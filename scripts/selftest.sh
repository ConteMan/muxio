#!/usr/bin/env bash
# Muxio unified quality gate. Local development and CI run the same script.
set -euo pipefail
cd "$(dirname "$0")/.."

failed=0
step() { echo "==> $1"; }

step "required project contracts"
required_files=(
  AGENTS.md
  README.md
  README.en.md
  CONTRIBUTING.md
  CONTRIBUTING.en.md
  CHANGELOG.md
  api/openapi.yaml
  docs/MAP.md
  docs/design/architecture.md
  docs/design/data-model.md
  docs/decisions/README.md
  docs/roadmap.md
  docs/specs/README.md
)
for file in "${required_files[@]}"; do
  test -f "$file" || { echo "missing required file: $file"; failed=1; }
done

step "ADR and Spec indexes"
for index_dir in decisions specs; do
  while read -r document; do
    test -f "docs/$index_dir/$document" || {
      echo "docs/$index_dir/README.md references missing file: $document"
      failed=1
    }
  done < <(grep -oE '\]\([0-9]{3}-[^)]*\.md\)' "docs/$index_dir/README.md" | sed 's/^](//;s/)$//' || true)
done

step "go.mod is tidy"
go mod tidy -diff || failed=1

step "gofmt"
unformatted=$(gofmt -l cmd internal)
if [ -n "$unformatted" ]; then
  echo "unformatted Go files:"
  echo "$unformatted"
  failed=1
fi

step "go vet"
go vet ./... || failed=1

step "go test with race detector"
go test -race ./... || failed=1

step "go build"
go build ./... || failed=1

step "OpenAPI contract"
go run github.com/getkin/kin-openapi/cmd/validate@v0.142.0 -multi api/openapi.yaml || failed=1

step "whitespace"
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git diff --check || failed=1
  git diff --cached --check || failed=1
fi

if [ "$failed" -ne 0 ]; then
  echo "selftest FAILED"
  exit 1
fi

echo "selftest OK"
