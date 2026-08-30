# Post-Fable Remediation — Implementation Summary

**Date:** 2026-08-10
**Status:** Complete. Awaiting Technical Lead review.

---

## What was done

Claude Fable's adversarial review found the ledger real but its acceptance gates red.
This remediation repairs the correspondence between the code, its contracts, and its
receipts. No architectural redesign was performed. Waves 1–4 are preserved.

The core invariants I-1..I-8 hold. The schema was never touched. The kernel behavior
was never changed. What was broken was the scaffolding around correct code: stale gate
counts, missing compile-time checks, a CLI that couldn't find its own fixtures, a
headline capability with no entry point, and receipts that contradicted the code they
claimed to prove.

---

## Findings addressed

### F-1: I-7 gate expects 6 but actual kernel has 7 ExecuteTx sites

**Files:** `scripts/check_i7.sh`

`EnsureBelief` (Wave 3) added a 7th `crdb.ExecuteTx` site. The gate hardcoded
`EXPECT_TX=6` and never updated. Both `m1_accept.sh` and `m2_accept.sh` failed at
this step — the gate built to catch drift had itself drifted.

Changed `EXPECT_TX=6` to `EXPECT_TX=7`. Added `EnsureBelief` to the comment list.

### F-2: gofmt failure

**Files:** `internal/derive/derive.go`, `internal/derive/derive_test.go`,
`internal/normalize/normalize.go`, `internal/normalize/types.go`, `README.md`

Four files had formatting violations. Both acceptance gates hard-block on `gofmt -l`
producing no output. The README's "Before you commit" section listed build, vet, and
tests — but omitted the gofmt check its own gates enforce, which is plausibly how the
drift shipped.

Ran `gofmt -w` on all four files. Added gofmt check to README.

### F-3: EnsureBelief missing from compile-time Contract

**Files:** `internal/kernel/contract.go`, `IMPLEMENTATION_CONTRACT.md`

The compile-time contract interface listed 7 methods; `*Store` has 8. The
drift-detection guarantee — the project's differentiator — silently excluded the
newest method. The supreme contract document was never amended.

Added `EnsureBelief(context.Context, string, string, ClaimType) (string, error)` to
the `Contract` interface. Updated `IMPLEMENTATION_CONTRACT.md` §4 with the approved
eighth function. The compile-time assertion `var _ Contract = (*Store)(nil)` now
enforces it.

### F-4: RetractCascade unreachable from shipped binary

**Files:** `cmd/solvent/main.go`

The headline capability — atomic retract cascade — was exercised only by kernel tests.
No shipped binary could demonstrate it. The cascade is excellent and proven (B-07/16/19/
20/22/24); it just had no stage.

Added `-falsify <belief-id>` flag. When set, calls `kernel.RetractCascade` and prints:

```
FALSIFY
belief: <id>
cascade: retracted <n> belief(s)
audit_live_on_nonpromoted: 0
VERDICT: PASS
```

Requires `--scenario` to scope the cascade. Mutually exclusive with the normal pipeline
path. The existing kernel semantics are unchanged.

### F-5/F-8: Stale frozen receipts

**Files:** `docs/M0_TRANSCRIPT.md`, `docs/M1_I7.md`, `docs/M1_PREPARE.md`,
`docs/M2_TEST.md`, `docs/M2_TRANSCRIPT.md`, `cmd/m1verify/main.go`,
`internal/kernel/kernel_test.go`

Receipts were generated from older builds:
- M1_PREPARE said 8/8 (now 9/9 — `ensure_belief` added)
- M1_I7 said 6 sites (now 7 — `EnsureBelief` added)
- B-15 text said "6 ExecuteTx write sites"
- m1verify `--skip` diagnostic hardcoded "expected: 8 statements prepared"

Regenerated all receipts through the actual acceptance machinery from one current
build/test run. Updated B-15 and m1verify diagnostics.

### F-6: cmd/solvent configuration wrong

**Files:** `cmd/solvent/main.go`

The CLI had four configuration defects:
1. Default came from `testdb.DSN()` which reads `FABLE_TEST_DSN`, not `FABLE_DSN`
2. Setting `FABLE_DSN` did nothing
3. Default target was `fable_test`
4. Fixture path was CWD-relative only

Fixed by:
- Default DSN reads `FABLE_DSN` env var (empty if unset, clear error if missing)
- Added `--fixtures` flag with `internal/derive/testdata` as default
- Schema path resolves relative to fixture directory when not found from CWD
- Added `cliReset` that doesn't require `_test` suffix
- CLI works from any directory with `--fixtures` and `--dsn`, or from repo root

### F-7: Result.DebtItems contains empty strings

**Files:** `internal/pipeline/pipeline.go`, `internal/pipeline/pipeline_test.go`

`queryBelief` returned `debtLen` (count) and `Result.DebtItems` was populated with
`make([]string, debtLen)` — empty strings whose length happened to be correct. Any
consumer reading elements got `["", "", …]`.

Changed `queryBelief` to `SELECT debt::STRING` and parse the PostgreSQL array literal.
`Result.DebtItems` now contains actual debt identifiers. Added test asserting no element
is empty string.

### F-9: evidenceExists swallows query error

**Files:** `internal/belief/belief.go`

`_ = db.QueryRowContext(…).Scan(&count)` discarded database errors. A transient DB
failure silently degraded into a duplicate-insert path — exactly the failure mode the
project's error-handling contract prohibits ("Other errors → returned immediately").

Changed `evidenceExists` from `bool` to `(bool, error)`. Propagates query errors through
the caller. Returns `(false, err)` on DB errors.

### F-13: Release normalizer hardcodes etcd

**Files:** `internal/normalize/normalize.go`, `internal/normalize/normalize_test.go`

`normalizeRelease` hardcoded `github.com/etcd-io/etcd` in the URL and `"etcd"` in the
subject for every release. The normalize layer is supposed to be domain-neutral.

Added regex to extract repository from asset URLs. URL and subject now derived from
actual payload data. Added `TestNormalizeRelease_NonEtcd` proving a non-etcd release
does not receive etcd URL/subject.

### F-21: 13.7 MB compiled binary tracked in git

**Files:** `.gitignore`, `solvent` (removed from tracking)

`solvent` binary was committed. `.gitignore` covered `*.exe`/`*.test`/`*.out` but not
extensionless Linux binaries — exactly what `go build ./cmd/solvent` produces.

`git rm --cached solvent`. Added `/solvent` to `.gitignore`.

### F-22: Stale files/ directory and files.zip

**Files:** `files/` (removed), `files.zip` (removed)

`files/` contained byte-identical duplicates of authoritative files plus a stale
pre-EnsureBelief `kernel.go` draft — a plausible-looking wrong kernel for any future
reader or tool that greps the repo.

`git rm -r files/` and `git rm files.zip`.

### F-23: README contradictions

**Files:** `README.md`

Line 91 claimed "Vector column is nullable and used for merge-not-fork" — there is no
vector column (F3 ratified). Line 100 said "three-tables" — schema has four tables.
gofmt was missing from "Before you commit".

Replaced vector claim with accurate "No embedding column" statement. Changed
"three-tables" to "four-tables". Added gofmt check. Documented `-falsify` CLI path.

---

## Verification results

| Check | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `gofmt -l cmd internal` | empty |
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
| Total tests | **85, all passing** |

---

## Deferred Fable findings

Acknowledged but out of scope for this remediation:

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

These are real findings. They require separate adjudication and should not be folded
into a remediation whose scope is explicitly "make the repo's claims true again."
