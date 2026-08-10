# FINAL ADVERSARIAL CODE REVIEW — SOLVENT REAL-WORLD ETCD IMPLEMENTATION

You are performing the FINAL INDEPENDENT ADVERSARIAL CODE REVIEW of the
recent etcd real-world evidence implementation in Solvent.

Repository:

    https://github.com/PithomLabs/solvent

The implementation was produced by a separate coding agent.

Your role is independent verification.

DO NOT implement fixes.
DO NOT modify files.
DO NOT redesign Solvent.
DO NOT reopen the research phase.
DO NOT expand scope.

The goal is to answer one question:

    "Does the implementation actually satisfy the approved
     etcd real-data implementation specification and preserve
     the existing Solvent invariants?"

============================================================
READ THESE FIRST
============================================================

1. Current Solvent repository:
   https://github.com/PithomLabs/solvent

2. Approved implementation specification:

   plans/post/etcd_dataspec2.md

3. Implementation report:

   etcd_dataspec2_imp.md

4. If available, also inspect the earlier adversarial dataset review:

   DATASET_ADVERSARIAL_REVIEW.md

The approved implementation specification is the intended contract.

The CURRENT REPOSITORY is authoritative for what actually exists.

The implementation report is NOT evidence by itself.

Verify its claims against the code and database behavior.

============================================================
REVIEW SCOPE
============================================================

This is a focused implementation verification.

Do NOT spend time on:

- future architecture
- UI
- bchat
- Taskfile
- demo polish
- Kubernetes
- additional datasets
- additional domains
- deferred findings unrelated to this implementation
- theoretical improvements

Only identify a finding if it represents:

    correctness failure
    regression
    violation of the approved specification
    misleading implementation/reporting
    loss of an existing invariant
    reproducibility failure
    security/integrity problem relevant to the demo

============================================================
1. SPECIFICATION CONFORMANCE
============================================================

Read plans/post/etcd_dataspec2.md and compare it section-by-section with
the actual implementation.

Verify:

- Track 1 fixture structure
- Track 2 fixture structure
- five real records
- zero fabricated external evidence
- provenance/manifest
- scenario separation
- source types
- normalization
- derivation
- debt handling
- operator-review
- intent creation
- replay behavior
- falsification behavior
- acceptance criteria

Identify any divergence.

Do not assume a divergence is bad merely because implementation differs
from wording; determine whether the behavioral contract is violated.

============================================================
2. REAL FIXTURE PROCESSING
============================================================

Inspect the actual fixture files.

Verify every intended real record:

- exists
- is actually discovered by the pipeline
- is actually processed
- produces the expected evidence/belief effects
- has the expected scenario_id
- is not silently skipped
- is not accidentally processed under the wrong scenario

Pay particular attention to:

    track1/
    track2/

Verify there are no unused JSON files sitting in an ingestion directory
that the pipeline silently ignores or mishandles.

============================================================
3. TRACK 1 END-TO-END
============================================================

Independently exercise or verify the actual flow:

    real GHSA/CVE evidence
          ↓
    v3.5.27 evidence
          ↓
    v3.5.28 evidence
          ↓
    beliefs/evidence
          ↓
    debt state
          ↓
    operator review
          ↓
    promotion
          ↓
    --action
          ↓
    live intent

Verify actual CockroachDB state where possible.

Do not accept CLI output alone.

Confirm:

- correct beliefs
- correct evidence count
- expected debt state
- promotion only after required review
- intent created only after promotion
- intent is LIVE
- audit_live_on_nonpromoted = 0

============================================================
4. TRACK 1 REPLAY
============================================================

Run or inspect the actual replay behavior.

Verify:

- no duplicate beliefs
- no duplicate evidence
- no duplicate intents
- deterministic result
- same semantic ledger state
- no transaction/invariant violation

Be precise about what "replay" means.

Do NOT require intent creation to occur inside the ingestion pipeline.

The approved design deliberately keeps intent creation outside the pipeline.

============================================================
5. TRACK 2 END-TO-END
============================================================

Verify:

    v3.5.0 historical/postulated deployment decision
          ↓
    promotion
          ↓
    live intent
          ↓
    v3.5 postmortem evidence
          ↓
    --falsify
          ↓
    RetractCascade
          ↓
    belief RETRACTED
          ↓
    intent CANCELLED
          ↓
    audit invariant

Verify the chronology/scenario isolation.

Confirm that the postmortem does not accidentally become evidence for
Track 1.

Confirm that falsification is scoped to the intended scenario.

============================================================
6. OPERATOR-REVIEW --action
============================================================

Inspect the implementation carefully.

Verify:

- --action is optional
- without --action, promotion does NOT create an intent
- with --action, promotion happens first
- IntentOnPromoted is used
- no SQL bypass is used
- failed promotion cannot create an intent
- an unpromoted belief cannot acquire a live intent
- exactly one intended live intent is created
- the action representation matches the existing kernel API
- audit invariant remains valid

This is an important judge-facing behavior.

============================================================
7. RETRACTCASCADE
============================================================

Inspect the actual falsification path.

Verify:

- it uses the existing kernel RetractCascade
- no alternative transaction mechanism was introduced
- no direct SQL write bypasses the kernel
- belief becomes RETRACTED
- dependent live intent becomes CANCELLED
- audit_live_on_nonpromoted remains zero
- scenario isolation is preserved

Do not require belief_edge usage where the approved design explicitly
does not use it.

============================================================
8. EXISTING INVARIANTS
============================================================

Verify that the implementation did NOT alter:

- database schema
- kernel.go semantics
- ExecuteTx write discipline
- belief status semantics
- intent status semantics
- six-item FullDebt vocabulary
- belief_edge semantics

Check:

    scripts/check_i7.sh

and inspect any newly added database writes.

Specifically look for:

    db.Exec
    tx.Exec outside ExecuteTx
    direct SQL mutations
    schema changes
    hidden writes

============================================================
9. PROVENANCE / DATA INTEGRITY
============================================================

Verify:

- manifest hashes match actual fixtures
- source URLs correspond to the intended real sources
- no fabricated external identifiers
- no fake external evidence
- attribution files are accurate
- content_sha256 semantics remain correct
- fixture hashes are not confused with evidence.content_sha256

============================================================
10. TEST COVERAGE
============================================================

Run:

    go test -count=1 -p 1 ./...

and inspect the actual package results.

Do not rely on the implementation report's test count.

Verify tests cover the newly introduced behavior, particularly:

- operator review
- --action
- intent gating
- replay
- falsification
- scenario isolation
- real fixture processing where appropriate

Also run:

    go build ./...
    go vet ./...
    gofmt -l cmd internal
    scripts/check_i7.sh

If CockroachDB is required, start it using the repository's existing
supported mechanism and run the DB-dependent tests.

============================================================
11. IMPLEMENTATION REPORT ACCURACY
============================================================

Compare etcd_dataspec2_imp.md against reality.

Flag claims that are:

- false
- incomplete in a materially misleading way
- stale
- stronger than what the implementation actually proves

Do not penalize harmless wording differences.

============================================================
12. REGRESSION CHECK
============================================================

Compare the current implementation against the previously verified
Solvent core.

Look specifically for accidental regressions in:

- normalize
- derive
- belief
- intent
- pipeline
- kernel
- existing tests

The real-data implementation must remain additive and must not silently
break existing behavior.

============================================================
SEVERITY
============================================================

Classify every finding:

BLOCKER
IMPORTANT
MINOR
NIT

BLOCKER:
Implementation is incorrect, violates a core invariant, produces a
misleading demo result, or fails a required scenario.

IMPORTANT:
Should be fixed before public demo because it materially affects
correctness or reproducibility.

MINOR:
Non-blocking correctness/documentation issue.

NIT:
Optional polish. MUST NOT delay the demo.

Do not inflate severity.

============================================================
OUTPUT
============================================================

Produce:

    FINAL_ETCD_IMPLEMENTATION_REVIEW.md

Use exactly this structure:

# 1. Verdict

Choose exactly one:

    APPROVED
    APPROVED WITH MINOR CORRECTIONS
    REWORK REQUIRED

# 2. Executive Assessment

Briefly explain why.

# 3. Specification Conformance

Table:

| Area | Status | Evidence |
|------|--------|----------|
| Track 1 | | |
| Track 1 replay | | |
| Track 2 | | |
| operator-review | | |
| --action | | |
| --falsify | | |
| provenance | | |
| invariants | | |
| tests | | |

# 4. Findings

For each finding:

    ID
    Severity
    Location
    Evidence
    Why it matters
    Required correction

If none, explicitly say:

    No material findings.

# 5. End-to-End Verification

Report actual results for:

- Track 1
- Track 1 replay
- Track 2
- falsification
- intent creation
- audit invariant

# 6. Regression Verification

Report:

- go test
- go build
- go vet
- gofmt
- I-7

# 7. Implementation Report Accuracy

State whether etcd_dataspec2_imp.md accurately describes the implementation.

# 8. Final Recommendation

If APPROVED:

    Freeze the Solvent core and proceed to the demo/playground phase.

If APPROVED WITH MINOR CORRECTIONS:

    List only the minor corrections and explicitly state that implementation
    architecture is approved.

If REWORK REQUIRED:

    List only the concrete changes required to reach approval.

============================================================
IMPORTANT FINAL RULE
============================================================

This is the LAST core implementation review.

Do not create an endless review loop.

If the implementation is functionally correct and the remaining findings
are cosmetic or future improvements, APPROVE IT.

The August 18 hackathon deadline matters.

We now need to move from:

    verified implementation

to:

    judge-friendly playground + Taskfile + bchat demo

unless you find a genuine correctness blocker.

Do NOT modify the repository.
Do NOT commit.
Do NOT implement fixes.

Only produce FINAL_ETCD_IMPLEMENTATION_REVIEW.md.