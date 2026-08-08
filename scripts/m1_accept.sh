#!/usr/bin/env bash
# The M1 gate. Runs, in order:
#   E2  harness isolation      (scripts/check_isolation.sh, D-023 — carried from M0)
#       build                  go build ./...   -> docs/M1_BUILD.md
#       vet                    go vet ./...     -> docs/M1_BUILD.md
#       gofmt                  formatting check
#   I-7 write-path gate        (scripts/check_i7.sh)        -> docs/M1_I7.md
#       PREPARE sweep          (cmd/m1verify)               -> docs/M1_PREPARE.md
#
# Any step non-zero stops the script and prints criterion / observed / expected.
#
# The PREPARE sweep needs a cluster: run scripts/m0_up.sh first. Pass --skip-prepare
# only in a DB-less environment; it is reported loudly and does NOT count as green.
set -euo pipefail

cd "$(dirname "$0")/.."

HOST="${FABLE_HOST:-localhost}"
PORT="${FABLE_PORT:-26260}"
DB="${FABLE_DB:-fable}"
DSN="${FABLE_DSN:-postgresql://root@${HOST}:${PORT}/${DB}?sslmode=disable}"

SKIP_PREPARE=0
[ "${1:-}" = "--skip-prepare" ] && SKIP_PREPARE=1

BUILD_OUT=docs/M1_BUILD.md
mkdir -p docs

echo "== E2: harness isolation =="
./scripts/check_isolation.sh

echo
echo "== build + vet =="
{
  echo "# M1 Build Transcript"
  echo
  echo "Both commands run over the whole module, per contract §7 M1."
  echo
} > "$BUILD_OUT"

build_log=$(go build ./... 2>&1) && build_rc=0 || build_rc=$?
{
  echo "## \`go build ./...\`"
  echo
  echo '```'
  [ -n "$build_log" ] && echo "$build_log" || echo "(no output)"
  echo '```'
  echo
  echo "exit code: **${build_rc}**"
  echo
} >> "$BUILD_OUT"

vet_log=$(go vet ./... 2>&1) && vet_rc=0 || vet_rc=$?
{
  echo "## \`go vet ./...\`"
  echo
  echo '```'
  [ -n "$vet_log" ] && echo "$vet_log" || echo "(no output)"
  echo '```'
  echo
  echo "exit code: **${vet_rc}**"
  echo
} >> "$BUILD_OUT"

fmt_log=$(gofmt -l cmd internal 2>&1)
{
  echo "## \`gofmt -l cmd internal\`"
  echo
  echo '```'
  [ -n "$fmt_log" ] && echo "$fmt_log" || echo "(no output — all files formatted)"
  echo '```'
} >> "$BUILD_OUT"

if [ "$build_rc" -ne 0 ]; then
  echo "M1 BLOCKED"
  echo "  criterion: go build ./... exits 0"
  echo "  observed:  exit $build_rc — $build_log"
  echo "  expected:  exit 0"
  exit 1
fi
if [ "$vet_rc" -ne 0 ]; then
  echo "M1 BLOCKED"
  echo "  criterion: go vet ./... exits 0"
  echo "  observed:  exit $vet_rc — $vet_log"
  echo "  expected:  exit 0"
  exit 1
fi
if [ -n "$fmt_log" ]; then
  echo "M1 BLOCKED"
  echo "  criterion: gofmt -l cmd internal is empty"
  echo "  observed:  $fmt_log"
  echo "  expected:  no output"
  exit 1
fi
echo "build 0, vet 0, gofmt clean -> $BUILD_OUT"

echo
echo "== I-7: write-path gate =="
./scripts/check_i7.sh

echo
echo "== PREPARE sweep (D-039) =="
if [ "$SKIP_PREPARE" -eq 1 ]; then
  go run ./cmd/m1verify --skip || true
  echo
  echo "M1 NOT GREEN: the PREPARE sweep was skipped."
  exit 2
fi
go run ./cmd/m1verify --dsn "$DSN" --out docs/M1_PREPARE.md

echo
echo "M1 gate green."
echo "  docs/M1_BUILD.md   build + vet + gofmt"
echo "  docs/M1_I7.md      I-7 write-path report"
echo "  docs/M1_PREPARE.md PREPARE transcript"
