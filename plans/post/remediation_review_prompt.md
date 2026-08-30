You are performing the POST-FABLE REMEDIATION VERIFICATION for Solvent.

You previously performed the full adversarial repository review and produced
the findings that led to the attached remediation.

The coding agent has now implemented the remediation.

Read:

1. The original Fable adversarial review that identified F-1 through F-23.
2. The new post-remediation implementation summary:
   fix.md
3. The current repository itself.
4. The relevant acceptance scripts, contracts, README, CLI, schema, and
   implementation files.

IMPORTANT:

Do NOT perform another broad architectural redesign.

Do NOT invent new requirements.

Do NOT reopen Waves 1–4.

The purpose of this review is to determine whether the remediation actually
fixed the findings that it claimed to fix.

============================================================
PRIMARY QUESTION
============================================================

For every remediation item claimed as fixed:

    Was it actually fixed in the repository?

Do not trust the implementation summary.

Inspect the actual code and, where useful, execute the relevant checks.

Classify each finding:

    FIXED
    PARTIALLY FIXED
    NOT FIXED
    REGRESSION

============================================================
F-1 — I-7 ACCEPTANCE GATE
============================================================

Verify:

- scripts/check_i7.sh expects 7 ExecuteTx sites.
- EnsureBelief is actually counted.
- the script still detects unexpected ExecuteTx drift.
- no code was removed merely to satisfy the count.

Run the gate if practical.

Expected:

    I-7 PASS
    7 ExecuteTx write sites

============================================================
F-2 — GOFMT
============================================================

Verify:

    gofmt -l cmd internal

returns empty.

Do not merely inspect the changed files.

============================================================
F-3 — ENSUREBELIEF CONTRACT
============================================================

Verify the actual Store API and compile-time Contract interface.

Confirm:

- EnsureBelief is present in Contract.
- signature matches Store.
- var _ Contract = (*Store)(nil) remains active.
- IMPLEMENTATION_CONTRACT.md documents the eighth function.

Look for any remaining stale "7 functions" claims.

============================================================
F-4 — RETRACTCASCADE CLI
============================================================

This is the most important new capability.

Inspect cmd/solvent/main.go and actually exercise the falsification path
against CockroachDB if possible.

Verify:

    -falsify <belief-id>

actually invokes the existing kernel.RetractCascade.

Verify:

- scenario scoping is enforced.
- falsification cannot accidentally operate on another scenario.
- normal pipeline and falsification modes are mutually exclusive.
- the cascade is transactional.
- downstream live intents are cancelled/retracted according to the
  existing kernel semantics.
- audit_live_on_nonpromoted remains zero.
- the command produces a truthful receipt.
- an invalid/nonexistent belief ID fails safely.
- a belief from another scenario cannot be falsified merely by knowing its ID.

CRITICAL:

Do NOT accept the mere existence of the flag as sufficient.

Trace the actual call path:

CLI
→ scenario resolution
→ belief lookup/validation
→ RetractCascade
→ resulting database state
→ audit

This is now a judge-facing capability.

============================================================
F-5 / F-8 — RECEIPTS
============================================================

Verify the current receipts against the actual repository and current
verification run.

Check specifically:

- M1 PREPARE = 9/9
- I-7 = 7 ExecuteTx sites
- B-15 wording is current
- m1verify diagnostics no longer say expected 8
- M2 receipt matches the current implementation
- no receipt is merely manually edited to make numbers appear correct

Run the relevant acceptance machinery if possible.

============================================================
F-6 — CLI CONFIGURATION
============================================================

Verify the actual CLI behavior.

Test or inspect:

1. FABLE_DSN
2. --dsn
3. --fixtures
4. execution from repository root
5. execution from a different working directory

Confirm:

- FABLE_DSN actually controls the DSN.
- the CLI does not silently connect to fable_test.
- missing DSN produces a clear error.
- fixture discovery is not accidentally dependent on CWD.
- schema/fixture resolution is deterministic.

Do not accept documentation claims without testing.

============================================================
F-7 — DEBT ITEMS
============================================================

Verify Result.DebtItems contains actual values, not merely the correct
length.

Inspect:

- queryBelief
- SQL used to retrieve debt
- PostgreSQL/CockroachDB array parsing
- tests

Verify no empty-string placeholder survives.

============================================================
F-9 — DATABASE ERROR PROPAGATION
============================================================

Inspect evidenceExists and its callers.

Verify:

- Scan/query errors propagate.
- only legitimate "absence" behavior is treated as absence.
- transient DB failures cannot silently become duplicate-insert behavior.

If practical, inspect or exercise an error path.

============================================================
F-13 — DOMAIN-NEUTRAL RELEASE NORMALIZATION
============================================================

Verify the normalizer no longer hardcodes etcd for every release.

Specifically inspect:

- repository extraction
- subject generation
- URL generation
- non-etcd test

Use a non-etcd fixture/example and verify it does not become:

    github.com/etcd-io/etcd
    etcd <version>

Do NOT require a generalized multi-domain architecture.

============================================================
F-21 — BINARY HYGIENE
============================================================

Verify:

- tracked `solvent` binary is gone.
- .gitignore prevents accidental re-addition.
- no replacement binary was committed.

============================================================
F-22 — STALE FILES
============================================================

Verify:

- files/ is gone.
- files.zip is gone.
- stale pre-EnsureBelief kernel.go is gone.
- no authoritative source was accidentally deleted.

============================================================
F-23 — README TRUTHFULNESS
============================================================

Read the entire README and compare it against:

- actual schema
- actual CLI
- actual contracts
- actual current behavior

Verify specifically:

- four tables are stated correctly:
    belief
    belief_edge
    evidence
    action_intent

- no embedding/vector column is falsely claimed.
- gofmt appears in the verification instructions.
- -falsify is documented accurately.
- automatic contradiction-driven retraction is not falsely presented as
  current ProcessEvidence behavior.

Also look for any NEW contradictions introduced by the remediation.

============================================================
REGRESSION CHECK
============================================================

Run, or independently verify the reported results:

    go build ./...
    go vet ./...
    gofmt -l cmd internal
    go test -count=1 -p 1 ./...
    scripts/check_isolation.sh
    scripts/check_i7.sh
    scripts/m1_accept.sh
    scripts/m2_accept.sh

Verify:

- 84 tests still pass.
- M0 = 16/16.
- M1 PREPARE = 9/9.
- M2 = 28/28 twice.
- I-7 = 7.
- normal CLI PASS.
- falsification CLI PASS.
- replay remains deterministic.

============================================================
IMPORTANT: F-10
============================================================

The remediation summary explicitly leaves F-10 deferred:

    go test ./internal/kernel

may overwrite docs/M2_TRANSCRIPT.md.

Do not automatically classify this as a failure of the current remediation.

Instead report it separately as:

    DEFERRED FINDING — F-10

and state whether it creates any immediate public-repository correctness
problem.

============================================================
DEFERRED FINDINGS
============================================================

Do not turn these into blockers unless the current remediation caused a
regression:

- F-10
- F-11
- F-12
- F-14
- F-15
- F-16
- F-17
- F-18
- F-19
- F-20

We are intentionally not expanding scope.

============================================================
FINAL VERDICT
============================================================

Produce:

    POST_FABLE_REMEDIATION_REVIEW.md

Include a table:

| Finding | Status | Evidence | Remaining Issue |
|---------|--------|----------|-----------------|

Then provide:

1. Remediation verification
2. Regression verification
3. Falsification CLI verification
4. Public repository verification
5. Deferred findings
6. Any NEW findings discovered

Finally give exactly one overall verdict:

    APPROVED — REMEDIATION VERIFIED

or

    REWORK REQUIRED

or

    BLOCKED

Use REWORK REQUIRED only if an actual remediation item is still broken
or a regression was introduced.

Do not recommend architectural changes merely for future robustness.

The goal is to determine whether Solvent has successfully crossed the
boundary from:

    "working core with broken scaffolding"

to:

    "verified core with truthful contracts, receipts, and a judge-playable
     falsification path."

Stop after producing the review.