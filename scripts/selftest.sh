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
  index="docs/$index_dir/README.md"
  referenced=$(grep -oE '\]\([0-9]{3}-[^)]*\.md\)' "$index" | sed 's/^](//;s/)$//' | sort -u || true)

  # Forward: every reference in the index must resolve to a real file.
  while read -r document; do
    test -n "$document" || continue
    test -f "docs/$index_dir/$document" || {
      echo "$index references missing file: $document"
      failed=1
    }
  done <<<"$referenced"

  # Reverse: every numbered document must be reachable from the index.
  while read -r path; do
    test -n "$path" || continue
    document=$(basename "$path")
    grep -qxF "$document" <<<"$referenced" || {
      echo "$index does not reference existing document: $document"
      failed=1
    }
  done < <(find "docs/$index_dir" -maxdepth 1 -name '[0-9][0-9][0-9]-*.md' | sort)
done

step "bilingual document mirrors"
# The English mirror must track the Chinese source section by section. Comparing
# the heading-level sequence catches a mirror that silently drifts behind.
for pair in "README.md:README.en.md" "CONTRIBUTING.md:CONTRIBUTING.en.md"; do
  source_doc="${pair%%:*}"
  mirror_doc="${pair##*:}"
  source_shape=$(grep -oE '^#+' "$source_doc" || true)
  mirror_shape=$(grep -oE '^#+' "$mirror_doc" || true)
  if [ "$source_shape" != "$mirror_shape" ]; then
    echo "$mirror_doc is out of sync with $source_doc (heading structure differs)"
    diff <(echo "$source_shape") <(echo "$mirror_shape") || true
    failed=1
  fi
done

step "shell scripts parse"
for script in scripts/*.sh; do
  bash -n "$script" || failed=1
  test -x "$script" || { echo "script is not executable: $script"; failed=1; }
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
