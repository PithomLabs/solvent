Yes. I would give the coding agent **one tightly scoped remediation batch** based directly on Fable's full-repo review, then we verify it. Do not ask it to redesign anything.

```text
# SOLVENT — POST-FABLE ADVERSARIAL REMEDIATION

You are the implementation/coding agent for Solvent.

Claude Fable has completed a full adversarial review of the frozen repository
on 2026-08-10. The review is attached/provided as:

    review_fable.md

READ THAT REVIEW IN FULL BEFORE TOUCHING CODE.

The review is authoritative for this remediation pass.

Its central conclusion is:

    "The ledger is real. The gates around it are red."

The core Transactional Belief Ledger behavior is verified and must NOT be
redesigned. The problems are primarily acceptance-gate drift, stale receipts,
public-repo hygiene, CLI packaging, and one missing production/demo path for
the already-implemented RetractCascade capability.

============================================================
MISSION
============================================================

Repair the repository so that:

1. The actual code matches its contracts.
2. The acceptance machinery passes against the CURRENT code.
3. Frozen receipts are regenerated from the CURRENT code.
4. The public README describes the CURRENT repository truthfully.
5. The CLI is actually usable by a fresh judge/developer.
6. The existing RetractCascade capability has a minimal production/demo
   entry point.
7. No unnecessary architecture is introduced.

This is NOT a new design phase.

Do NOT reopen Waves 1–4 architecturally.

Do NOT redesign the Transactional Belief Ledger.

Do NOT add embeddings.

Do NOT add new agents.

Do NOT add new feeds.

Do NOT add new tables unless the existing contract explicitly requires one
(which this remediation does not).

Do NOT implement automatic contradiction retraction inside belief.Process.

Do NOT begin bchat integration yet.

Do NOT begin the final visual demo yet.

============================================================
AUTHORITATIVE BASELINE
============================================================

Before coding, read:

- review_fable.md
- IMPLEMENTATION_CONTRACT.md
- AGENTS.md
- README.md
- PUBLIC_REPO_AUDIT.md
- DATA_PIPELINE_SPEC.md
- WAVE4_IMPLEMENTATION_PLAN.md
- WAVE4_REWORK.md
- relevant Wave 1/2/3 implementation/rework documents
- db/001_schema.sql
- current scripts/m1_accept.sh
- current scripts/m2_accept.sh
- current scripts/check_i7.sh

Also inspect the actual repository rather than trusting documentation.

Fable independently verified:

- 84 tests pass
- M0 = 16/16
- M2 = 28/28 twice
- PREPARE = 9/9
- Wave 4 CLI works
- replay is byte-identical
- core ledger invariants hold

Preserve those properties.

============================================================
PRIORITY 1 — RESTORE ACCEPTANCE GATES
============================================================

### F-1: I-7 gate expects 6 but actual kernel has 7 ExecuteTx sites

The legitimate seventh site is:

    EnsureBelief

Update:

    scripts/check_i7.sh

from the stale expected count to the actual current contract.

Update its explanatory comment/list to include EnsureBelief.

Do NOT remove EnsureBelief.

Do NOT weaken the check.

Then regenerate the affected receipts through the actual acceptance
workflow rather than manually editing numbers.

Affected material includes the I-7/M1/M2 receipts identified by Fable.

### F-2: gofmt failure

Run gofmt on the four files identified by Fable:

    internal/derive/derive.go
    internal/derive/derive_test.go
    internal/normalize/normalize.go
    internal/normalize/types.go

Then verify:

    gofmt -l cmd internal

produces no output.

Add the gofmt check explicitly to the public README's "Before you commit"
instructions because the acceptance gates already enforce it.

============================================================
PRIORITY 2 — RESTORE THE KERNEL API CONTRACT
============================================================

### F-3: EnsureBelief missing from compile-time Contract

The actual Store API now contains EnsureBelief.

The compile-time contract must contain it too.

Update:

    internal/kernel/contract.go

to include EnsureBelief with the exact current signature.

Then update:

    IMPLEMENTATION_CONTRACT.md §4

to record the approved eighth kernel API function.

Do NOT invent a different signature.

Do NOT alter the behavior of EnsureBelief.

The point of this contract is to mechanically catch API drift, so the
contract MUST reflect the actual API.

Also update any stale documentation/receipts that still say:

    seven functions

when they now mean eight.

============================================================
PRIORITY 3 — FIX THE PUBLIC CLI
============================================================

### F-6: cmd/solvent configuration is wrong

Current problems identified by Fable:

- implementation uses testdb.DSN()
- FABLE_DSN does not actually control the CLI
- default target is fable_test
- fixture path is CWD-relative
- CLI therefore isn't packaged as a proper judge-facing executable

Fix this minimally.

Requirements:

1. `--dsn` must work.
2. `FABLE_DSN` must work.
3. Do not silently default the demo to the test database.
4. Preserve an explicit development-friendly default only if clearly
   documented and safe.
5. Make the fixture directory configurable.
6. The default fixture location may remain the repository's existing fixture
   path, but it must be explicit and configurable.
7. Do not make the CLI dependent on being launched from the repository root
   unless that is unavoidable and explicitly documented.
8. Keep the existing fixture-based deterministic MVP.

Do NOT add live GitHub APIs.

Do NOT add CISA polling.

Do NOT add external infrastructure.

After fixing, verify the CLI from:

- repository root
- a different working directory, if practical
- explicit --dsn
- FABLE_DSN

The final README command must work against a fresh checkout.

============================================================
PRIORITY 4 — EXPOSE RETRACTCASCADE
============================================================

### F-4: headline capability is not reachable from a shipped binary

This is the most important demo-related remediation.

The existing kernel `RetractCascade` is already implemented and tested.

Do NOT rewrite it.

Add the smallest production/demo path needed to invoke it.

Preferred shape:

    cmd/solvent -falsify <belief-id>

or an equivalently clear CLI flag/subcommand.

Behavior:

    supplied belief ID
        ↓
    kernel.RetractCascade(...)
        ↓
    transaction atomically:
        - cancels downstream live intents
        - retracts the belief/cascade according to existing kernel semantics
        - preserves all existing invariants
        ↓
    print a concise receipt
        ↓
    run/print audit result

The CLI should make the result obvious enough for a judge to understand.

Example conceptual output:

    FALSIFY
    belief: <id>
    cascade: retracted
    live_intents_cancelled: <n>
    audit_live_on_nonpromoted: 0
    VERDICT: PASS

Do NOT add automatic contradiction handling to ProcessEvidence.

Do NOT create a new retraction model.

Do NOT change RetractCascade's transactional semantics.

Add integration coverage for the CLI path.

This is exposing existing capability, not creating new kernel functionality.

============================================================
PRIORITY 5 — REGENERATE RECEIPTS
============================================================

### F-5 / F-8

Do NOT manually edit frozen receipt numbers.

After the code and acceptance scripts are corrected, run the actual
verification machinery from a clean environment.

Regenerate all stale artifacts identified by Fable, including as applicable:

- docs/M1_I7.md
- docs/M1_BUILD.md
- docs/M1_PREPARE.md
- docs/M2_TEST.md
- docs/M2_TRANSCRIPT.md
- relevant M0/M1/M2 reports

The generated receipts must all correspond to ONE CURRENT BUILD/TEST RUN
where practical.

Ensure the following are truthful:

- PREPARE = 9/9
- I-7 = 7 ExecuteTx sites
- M2 = current actual case count
- total tests = current actual test count

Fix:

    cmd/m1verify --skip

if it still contains the stale hardcoded "expected: 8" diagnostic.

Fix B-15's stale expected text so it no longer claims 6 ExecuteTx sites.

IMPORTANT:

Running tests must not create uncontrolled stale receipt modifications.

If the test suite necessarily writes a receipt, make that behavior deliberate
and consistent with the project's frozen-evidence model.

============================================================
PRIORITY 6 — RESULT.DEBTITEMS
============================================================

### F-7

`Result.DebtItems` is documented as containing actual debt items but currently
contains only empty strings whose length happens to be correct.

Fix this.

Return the actual remaining debt items.

The implementation may require the existing `queryBelief` path to retrieve
the actual debt values.

Do NOT redesign the debt model.

Add/update tests proving:

    len(DebtItems) == number of remaining items

AND:

    DebtItems contains the actual debt identifiers/items.

============================================================
PRIORITY 7 — ERROR HANDLING
============================================================

### F-9

Fix:

    internal/belief/belief.go

where `evidenceExists` discards the QueryRow/Scan error.

Do NOT swallow database errors.

Follow the existing project error-handling contract:

    successful query → use result
    actual query error → return/wrap error
    "not found" → handle as absence only where semantically appropriate

Do not turn transient DB failures into a duplicate-insert path.

Add a focused regression test if practical.

============================================================
PRIORITY 8 — DOMAIN-NEUTRAL RELEASE NORMALIZATION
============================================================

### F-13

Fable found:

    internal/normalize/normalize.go

hardcodes:

    github.com/etcd-io/etcd
    "etcd <version>"

for every release.

The normalize layer is supposed to be domain-neutral.

Fix this minimally.

Use actual repository/project information from the release payload where
available.

Preserve the existing etcd behavior.

Add a test proving a non-etcd release does not receive an etcd URL/subject.

Do NOT create a generic multi-domain framework.

The goal is simply to remove an incorrect hardcoded assumption from a
supposedly domain-neutral layer.

============================================================
PRIORITY 9 — PUBLIC REPOSITORY HYGIENE
============================================================

### F-21

Remove the tracked 13.7 MB compiled binary:

    solvent

from the repository.

Ensure it is ignored in the future.

The .gitignore must explicitly cover extensionless Linux binaries as
appropriate.

Do NOT rewrite unrelated Git history unless absolutely necessary.

A normal current-tree removal is sufficient for this remediation.

### F-22

Remove the stale duplicate:

    files/
    files.zip

and any clearly duplicated stale source contained there.

In particular, do NOT leave:

    files/kernel.go

because it is a stale pre-EnsureBelief kernel and is dangerous for future
agents/tools that inspect the repository.

Do not delete authoritative files elsewhere merely because they have similar
names.

Verify the final tree after removal.

============================================================
PRIORITY 10 — README TRUTHFULNESS
============================================================

### F-23

Fix the README contradictions identified by Fable.

Specifically eliminate:

1. The false claim that there is a vector column.

The actual design is:

    no embedding/vector column in the current MVP

and this is intentional.

2. The false "three-tables" claim.

The actual schema has exactly four tables:

    belief
    belief_edge
    evidence
    action_intent

3. Any other statements in README that contradict the current implementation.

Do not merely patch the two lines.

Read the WHOLE README against:

    db/001_schema.sql
    IMPLEMENTATION_CONTRACT.md
    current CLI
    current test/verification behavior

The README is public judge-facing documentation.

It must distinguish:

    CURRENT MVP
    from
    FUTURE / DEFERRED CAPABILITIES

In particular, do not claim automatic contradiction-driven retraction
currently occurs in ProcessEvidence.

The existing RetractCascade capability may be described as an available
kernel/CLI capability once the new CLI path is implemented.

============================================================
IMPORTANT: DO NOT CHASE EVERY FABLE FINDING
============================================================

The following Fable findings are acknowledged but OUT OF SCOPE for this
remediation unless fixing another issue naturally touches them:

- F-11 latent same-CVE aggregation gap
- F-12 theoretically unreachable SHA marshal failure
- F-14 concurrent EnsureBelief limitation
- F-15 M0 transcript wording discrepancy
- F-16 GitHub issue body parsing
- F-17 stronger sentinel assertions
- F-18 reset-lock timeout
- F-19 mutable FullDebt
- F-20 dead code

Do NOT expand scope to solve them.

Preserve the already-adjudicated waivers:

- deferred automatic contradiction retraction in Process()
- sequential-only ProposeIfNew guarantee
- fixtures-only ingestion
- no embedding column
- no graph UI
- no live external feed
- no additional agents

============================================================
VERIFICATION — REQUIRED
============================================================

After implementation:

1. Format:

    gofmt -w <changed Go files>

2. Verify:

    gofmt -l cmd internal

    MUST produce no output.

3. Build:

    go build ./...

4. Vet:

    go vet ./...

5. Unit/integration tests:

    go test -count=1 -p 1 ./...

6. Run:

    scripts/check_isolation.sh
    scripts/check_i7.sh

7. Run the acceptance gates:

    scripts/m1_accept.sh
    scripts/m2_accept.sh

Both MUST exit 0.

8. Run the M0 verification.

9. Run the M1 PREPARE sweep.

10. Run M2 verification twice if that is part of the existing acceptance
    machinery.

11. Run the normal CLI.

12. Run the new falsification/cascade CLI path.

13. Test CLI replay determinism.

14. Verify no tracked binary/duplicate files remain.

15. Verify README commands actually work.

============================================================
DELIVERABLES
============================================================

Create:

    FABLE_REMEDIATION.md

This must contain:

- each Fable finding addressed
- exact files changed
- why each change was necessary
- acceptance-gate results
- test count
- M0 result
- M1 result
- M2 result
- CLI normal-path receipt
- CLI falsification/cascade receipt
- replay receipt
- repository hygiene result
- README verification result
- explicitly deferred Fable findings

Do NOT create another architectural plan.

Do NOT commit to GitHub.

Do NOT publish anything.

STOP after implementation + verification.

Wait for Technical Lead review.
```

### Why I want this as one batch

The coding agent should **not interpret Fable's 23 findings as 23 reasons to redesign Solvent**. The review actually establishes that the core is working. 

The remediation is about restoring the project's defining property:

> **the code, contracts, receipts, and public story must all agree.**

And the `RetractCascade` CLI is the one deliberate demo addition because it converts an already-verified kernel capability into something a judge can actually exercise. 

After this batch, I want the cycle to be:

**coding agent implements → we review its implementation receipt → focused verification → freeze → real-data/demo work.**

No more whole-repo archaeology unless the verification uncovers something genuinely new.
