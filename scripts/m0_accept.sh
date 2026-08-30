#!/usr/bin/env bash
# The M0 gate. Runs, in order:
#   E2  harness isolation           (scripts/check_isolation.sh, D-023)
#       build                       (./cmd/m0verify only — never ./...)
#   A–D the acceptance transcript   (docs/M0_TRANSCRIPT.md)
#   E1  determinism                 (two --no-timings runs, diffed below the
#                                    volatile fence — D-024 makes this a criterion,
#                                    not a convenience)
#
# Any step non-zero stops the script and prints criterion / observed / expected.
set -euo pipefail

cd "$(dirname "$0")/.."

HOST="${FABLE_HOST:-localhost}"
PORT="${FABLE_PORT:-26260}"
# fable_m0, not fable. This script runs `m0verify --reset` three times, which DROPs and
# recreates the target database. Defaulting to `fable` meant running M0 acceptance
# destroyed the demo ledger, the 7,239-row corpus and its embeddings -- and it did,
# during Phase 5 verification. M0 needs a database of its own; it asserts on schema and
# gate behaviour, never on demo content.
DB="${FABLE_DB:-fable_m0}"
DSN="${FABLE_DSN:-postgresql://root@${HOST}:${PORT}/${DB}?sslmode=disable}"

BIN="$(mktemp -d)/m0verify"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK" "$(dirname "$BIN")"' EXIT

echo "== E2: harness isolation =="
./scripts/check_isolation.sh

echo
echo "== build =="
# ./cmd/m0verify specifically, so an unrelated problem in the pre-supplied
# the kernel package cannot masquerade as an M0 failure.
go build -o "$BIN" ./cmd/m0verify
echo "built $(basename "$BIN")"

echo
echo "== A-D: acceptance transcript =="
"$BIN" --dsn "$DSN" --reset --apply --out docs/M0_TRANSCRIPT.md

echo
echo "== E1: determinism (D-024) =="
"$BIN" --dsn "$DSN" --reset --apply --no-timings --out "$WORK/run1.md" >/dev/null
"$BIN" --dsn "$DSN" --reset --apply --no-timings --out "$WORK/run2.md" >/dev/null

# Strip through the volatile fence — exact, not line-count-based.
for f in run1 run2; do
  sed '1,/^<!-- volatile:end -->$/d' "$WORK/$f.md" > "$WORK/$f.stable"
done

if ! diff -u "$WORK/run1.stable" "$WORK/run2.stable" > "$WORK/e1.diff"; then
  echo "E1 FAIL"
  echo "  criterion: two clean runs must be byte-identical below the volatile header (D-024)"
  echo "  observed:  the transcripts differ —"
  sed 's/^/    /' "$WORK/e1.diff"
  echo "  expected:  empty diff"
  exit 1
fi
echo "E1 PASS: two independent runs are byte-identical below the volatile fence"

echo
echo "M0 gate green. Transcript: docs/M0_TRANSCRIPT.md"
