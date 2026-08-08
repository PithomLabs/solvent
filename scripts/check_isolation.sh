#!/usr/bin/env bash
# Probe E2 (D-023): internal/m0 is verification-only and must never be imported by
# internal/kernel. Prose cannot enforce a boundary; this check can.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -d internal/kernel ]; then
  echo "E2 PASS: internal/kernel does not exist yet"
  exit 0
fi

if grep -rn --include='*.go' 'solvent/internal/m0' internal/kernel; then
  echo
  echo "E2 FAIL"
  echo "  criterion: internal/kernel must contain zero imports of internal/m0 (D-023)"
  echo "  observed:  the import(s) listed above"
  echo "  expected:  no match"
  exit 1
fi

echo "E2 PASS: internal/kernel imports nothing from internal/m0"
