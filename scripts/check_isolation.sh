#!/usr/bin/env bash
# Probe E2 (D-023): internal/m0 is verification-only and must never be imported by
# the kernel. Prose cannot enforce a boundary; this check can.
#
# The kernel lives at ./kernel (Phase 1 moved it out of internal/ so it is importable
# as github.com/PithomLabs/solvent/kernel). Override with K_PKG if that ever changes.
set -euo pipefail

cd "$(dirname "$0")/.."

PKG="${K_PKG:-kernel}"

# A missing kernel is a breakage, not a pass. This used to `exit 0` when the directory
# was absent -- a vacuous green that would survive any future move of the package.
if [ ! -d "$PKG" ]; then
  echo "E2 FAIL"
  echo "  criterion: the kernel package must exist at $PKG"
  echo "  observed:  no such directory"
  echo "  expected:  $PKG present (set K_PKG to override)"
  exit 1
fi

if grep -rn --include='*.go' 'solvent/internal/m0' "$PKG"; then
  echo
  echo "E2 FAIL"
  echo "  criterion: $PKG must contain zero imports of internal/m0 (D-023)"
  echo "  observed:  the import(s) listed above"
  echo "  expected:  no match"
  exit 1
fi

echo "E2 PASS: $PKG imports nothing from internal/m0"
