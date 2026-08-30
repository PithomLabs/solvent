# FABLE_REMEDIATION.md

**Date:** 2026-08-10
**Status:** Implementation complete, verification green. Awaiting Technical Lead review.

---

## Summary

Claude Fable's adversarial review found the ledger real but its acceptance gates red.
This remediation repairs the correspondence between the code, its contracts, and its
receipts. No architectural redesign was performed. Waves 1–4 are preserved.

---

## Findings Addressed

### F-1: I-7 gate expects 6 but actual kernel has 7 ExecuteTx sites

**Files changed:** `scripts/check_i7.sh`
**Why:** `EnsureBelief` (Wave 3) added a 7th `crdb.ExecuteTx` site. The gate
hardcoded `EXPECT_TX=6` and never updated. Both `m1_accept.sh` and `m2_accept.sh`
failed at this step.
**Fix:** Changed `EXPECT_TX=6` to `EXPECT_TX=7`, added `EnsureBelief` to the comment.

### F-2: gofmt failure

**Files changed:** `internal/derive/derive.go`, `internal/derive/derive_test.go`,
`internal/normalize/normalize.go`, `internal/normalize/types.go`
**Why:** Four files had formatting violations. Both acceptance gates hard-block on
`gofmt -l` producing no output.
**Fix:** Ran `gofmt -w` on all four files. Added gofmt check to README "Before you
commit" section.

### F-3: EnsureBelief missing from compile-time Contract

**Files changed:** `internal/kernel/contract.go`, `IMPLEMENTATION_CONTRACT.md`
**Why:** The compile-time contract interface listed 7 methods; `*Store` has 8. The
drift-detection guarantee silently excluded `EnsureBelief`. The supreme contract
document was never amended.
**Fix:** Added `EnsureBelief(context.Context, string, string, ClaimType) (string, error)`
to the `Contract` interface. Updated `IMPLEMENTATION_CONTRACT.md` §4 with the approved
eighth function signature and behavioral contract.

### F-4: RetractCascade unreachable from shipped binary

**Files changed:** `cmd/solvent/main.go`
**Why:** The headline capability — atomic retract cascade — was exercised only by kernel
tests. No shipped binary could demonstrate it.
**Fix:** Added `-falsify <belief-id>` flag to `cmd/solvent`. When set, calls
`kernel.RetractCascade` and prints a concise receipt with cascade count and audit result.
Requires `--scenario` to scope the cascade. Mutually exclusive with the normal pipeline
path.

### F-5/F-8: Stale frozen receipts

**Files changed:** `docs/M0_TRANSCRIPT.md`, `docs/M1_I7.md`, `docs/M1_PREPARE.md`,
`docs/M2_TEST.md`, `docs/M2_TRANSCRIPT.md`, `cmd/m1verify/main.go`,
`internal/kernel/kernel_test.go`
**Why:** Receipts were generated from older builds. M1_PREPARE said 8/8 (now 9/9).
M1_I7 said 6 sites (now 7). B-15 text said "6 ExecuteTx write sites". m1verify
`--skip` diagnostic hardcoded "expected: 8".
**Fix:** Regenerated all receipts through the actual acceptance machinery from one
current build/test run. Updated B-15 Expected text to "7 ExecuteTx write sites".
Updated m1verify `--skip` diagnostic to "expected: 9 statements prepared".

### F-6: cmd/solvent configuration wrong

**Files changed:** `cmd/solvent/main.go`
**Why:** CLI defaulted to `testdb.DSN()` which reads `FABLE_TEST_DSN` (not `FABLE_DSN`).
Setting `FABLE_DSN` did nothing. Default target was `fable_test`. Fixture path was
CWD-relative only.
**Fix:** Default DSN now reads `FABLE_DSN` env var (empty if unset, failing with clear
error). Added `--fixtures` flag with `internal/derive/testdata` as default. Schema path
resolved relative to fixture directory when not found from CWD. Added `cliReset` that
doesn't require `_test` suffix. CLI now works from any directory with explicit
`--fixtures` and `--dsn`, or from repo root with defaults.

### F-7: Result.DebtItems contains empty strings

**Files changed:** `internal/pipeline/pipeline.go`, `internal/pipeline/pipeline_test.go`
**Why:** `queryBelief` returned `debtLen` (count) and `Result.DebtItems` was populated
with `make([]string, debtLen)` — empty strings whose length happened to be correct.
Any consumer reading elements got `["", "", …]`.
**Fix:** Changed `queryBelief` to `SELECT debt::STRING` and parse the PostgreSQL array
literal. `Result.DebtItems` now contains actual debt identifiers. Added test asserting
no element is empty string.

### F-9: evidenceExists swallows query error

**Files changed:** `internal/belief/belief.go`
**Why:** `_ = db.QueryRowContext(…).Scan(&count)` discarded database errors. A transient
DB failure silently degraded into a duplicate-insert path.
**Fix:** Changed `evidenceExists` from `bool` to `(bool, error)`. Propagates query errors
through the caller. Returns `(false, err)` on DB errors.

### F-13: Release normalizer hardcodes etcd

**Files changed:** `internal/normalize/normalize.go`, `internal/normalize/normalize_test.go`
**Why:** `normalizeRelease` hardcoded `github.com/etcd-io/etcd` in the URL and `"etcd"`
in the subject for every release. The normalize layer is supposed to be domain-neutral.
**Fix:** Added regex to extract repository from asset URLs. URL and subject now derived
from actual payload data. Added `TestNormalizeRelease_NonEtcd` proving a non-etcd release
does not receive etcd URL/subject.

### F-21: 13.7 MB compiled binary tracked in git

**Files changed:** `.gitignore`, `solvent` (removed from tracking)
**Why:** `solvent` binary was committed. `.gitignore` covered `*.exe`/`*.test`/`*.out`
but not extensionless Linux binaries.
**Fix:** `git rm --cached solvent`. Added `/solvent` to `.gitignore`.

### F-22: Stale files/ directory and files.zip

**Files changed:** `files/` (removed), `files.zip` (removed)
**Why:** `files/` contained byte-identical duplicates of authoritative files plus a
stale pre-EnsureBelief `kernel.go` draft dangerous for future agents.
**Fix:** `git rm -r files/` and `git rm files.zip`.

### F-23: README contradictions

**Files changed:** `README.md`
**Why:** Line 91 claimed "Vector column is nullable and used for merge-not-fork" —
there is no vector column (F3 ratified). Line 100 said "three-tables" — schema has
four tables. gofmt was missing from "Before you commit".
**Fix:** Replaced vector claim with accurate "No embedding column" statement. Changed
"three-tables" to "four-tables". Added gofmt check. Documented `-falsify` CLI path.

---

## Verification Results

| Check | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `gofmt -l cmd internal` | empty (no output) |
| `go test -count=1 -p 1 ./...` | **all 6 test packages ok** |
| `scripts/check_isolation.sh` | **E2 PASS** |
| `scripts/check_i7.sh` | **I-7 PASS — 7 ExecuteTx write sites** |
| `scripts/m1_accept.sh` | **exit 0** — build, vet, gofmt, I-7, PREPARE 9/9 |
| `scripts/m2_accept.sh` | **exit 0** — build, vet, gofmt, I-7, M2 28/28 twice |
| `cmd/m0verify --reset --apply --no-timings` | **16/16 GREEN** |
| `cmd/m1verify` PREPARE sweep | **9/9 statements prepared** |
| `cmd/solvent --reset --scenario …` | **VERDICT: PASS** |
| `cmd/solvent -falsify <id> --scenario …` | **VERDICT: PASS** — 1 belief retracted |
| CLI from different directory | works with `--fixtures` and `--dsn` |
| Replay determinism | byte-identical |
| `git status` | no tracked binary, no files/, no files.zip |
| Total tests | **84, all passing** |

---

## Explicitly Deferred Fable Findings

The following were acknowledged but out of scope per the remediation instructions:

- F-10: `go test ./internal/kernel` overwrites `docs/M2_TRANSCRIPT.md` (side effect)
- F-11: Latent same-CVE aggregation gap
- F-12: Theoretically unreachable SHA marshal failure
- F-14: Concurrent EnsureBelief limitation
- F-15: M0 probe A3 receipt wording discrepancy
- F-16: GitHub issue body parsing (title-only)
- F-17: Stronger sentinel assertions in tests
- F-18: Reset-lock timeout
- F-19: Mutable FullDebt
- F-20: Dead code
