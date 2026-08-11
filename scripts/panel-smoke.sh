#!/usr/bin/env bash
# Drives the panel in a real browser against a real `muxio serve`, because the
# panel only exists as something a binary embeds and serves.
set -euo pipefail
cd "$(dirname "$0")/.."

home="$(mktemp -d)/muxio"
binary="$PWD/bin/muxio-smoke"
port="${MUXIO_SMOKE_PORT:-9922}"

go build -o "$binary" ./cmd/muxio

export MUXIO_HOME="$home"
printf '%s\n' \
  '{"external_id":"note-1","title":"First","body":"content"}' \
  '{"external_id":"note-2","title":"Second","body":"more"}' \
  '{"body":"missing external_id"}' \
  | "$binary" import --source notes >/dev/null 2>&1 || true

MUXIO_LOG_LEVEL=debug "$binary" serve --addr "127.0.0.1:$port" >/tmp/muxio-smoke.log 2>&1 &
serve_pid=$!
trap 'kill "$serve_pid" 2>/dev/null || true; rm -f "$binary"' EXIT

for _ in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done

MUXIO_BASE_URL="http://127.0.0.1:$port" npm --prefix web run test:e2e
