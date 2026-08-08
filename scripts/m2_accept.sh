#!/usr/bin/env bash
# The M2 gate. Runs, in order:
#   E2   harness isolation      (scripts/check_isolation.sh)
#        build + vet + gofmt
#   I-7  static write-path gate (scripts/check_i7.sh)  -> docs/M1_I7.md   [case B-15]
#   run 1  behavioral suite     (go test)              -> docs/M2_TRANSCRIPT.md
#   run 2  behavioral suite     (go test)              -> I-6's second pass (A2)
#
# A suite that failed once has nothing to prove by repeating, so run 2 is skipped on
# failure and the gate exits non-zero with docs/M2_FAILURE.md already written.
set -euo pipefail

cd "$(dirname "$0")/.."

TEST_OUT=docs/M2_TEST.md
mkdir -p docs

echo "== E2: harness isolation =="
./scripts/check_isolation.sh

echo
echo "== build + vet + gofmt =="
go build ./... || { echo "M2 BLOCKED: go build ./... failed"; exit 1; }
go vet ./...   || { echo "M2 BLOCKED: go vet ./... failed"; exit 1; }
fmt_log=$(gofmt -l cmd internal)
if [ -n "$fmt_log" ]; then
  echo "M2 BLOCKED"
  echo "  criterion: gofmt -l cmd internal is empty"
  echo "  observed:  $fmt_log"
  echo "  expected:  no output"
  exit 1
fi
echo "build 0, vet 0, gofmt clean"

echo
echo "== I-7: static write-path gate (case B-15) =="
./scripts/check_i7.sh

echo
echo "== M2 behavioral suite: run 1 =="
{
  echo "# M2 Test Transcript"
  echo
  echo 'Captured `go test -v` output. Two runs: the second is contract §7 M2'\''s'
  echo 'second pass (A2), which under R2 also proves the suite is repeatable.'
  echo
  echo "## Run 1"
  echo
  echo '```'
} > "$TEST_OUT"

if go test ./internal/kernel/ -v -count=1 2>&1 | tee -a "$TEST_OUT"; then
  run1=0
else
  run1=1
fi
echo '```' >> "$TEST_OUT"

if [ "$run1" -ne 0 ]; then
  echo
  echo "M2 BLOCKED on run 1."
  echo "  criterion: every behavioral case passes"
  echo "  observed:  at least one case failed — see docs/M2_FAILURE.md"
  echo "  expected:  all cases PASS"
  echo "  decision:  STOP. Do not redesign, do not adapt the test to the observed behavior."
  echo
  echo "Run 2 skipped: a suite that failed once has nothing to prove by repeating."
  exit 1
fi

echo
echo "== M2 behavioral suite: run 2 (contract §7 M2 second pass) =="
{
  echo
  echo "## Run 2"
  echo
  echo '```'
} >> "$TEST_OUT"

if go test ./internal/kernel/ -v -count=1 2>&1 | tee -a "$TEST_OUT"; then
  run2=0
else
  run2=1
fi
echo '```' >> "$TEST_OUT"

if [ "$run2" -ne 0 ]; then
  echo
  echo "M2 BLOCKED on run 2."
  echo "  criterion: the suite passes a second consecutive time (A2)"
  echo "  observed:  run 1 passed, run 2 failed — the suite is not repeatable"
  echo "  expected:  both runs green"
  echo "  decision:  STOP"
  exit 1
fi

echo
echo "M2 gate green."
echo "  docs/M2_TRANSCRIPT.md  behavioral receipts"
echo "  docs/M2_TEST.md        both go test runs"
echo "  docs/M1_I7.md          I-7 static gate (case B-15)"
echo "  docs/M2_FAILURE.md     absent, as expected on a green run"
