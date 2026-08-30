# PLAN3_IMPLEMENTATION_REVIEW.md

**Purpose:** Documents implementation changes made to `plan3.md` per `plan3_review.md`
and provides an adversarial code review prompt for the implementation code.

**Authority:** Follows the review workflow defined in `prompt_imp.md`.

---

## 1. Implementation Documentation

### 1.1 Changes Applied

Four nits from `plan3_review.md` were applied to `plan3.md`:

| Nit | Change | Rationale |
|---|---|---|
| N1 | Removed `Confidence float64` from `NormalizedEvidence` struct | Weeks of work removing stored confidence as a first-class concept. The ledger cares about evidence, not confidence. Even with the comment "never stored in ledger," it risks reintroducing the wrong mental model. |
| N2 | Renamed `internal/claim` → `internal/derive` | Better reflects logic-first semantics. We're deriving beliefs, not extracting language. Consistency with the project's terminology. |
| N3 | Added note that version strings are illustrative fixture data | Clarifies that rules are generic pattern-matchers, not hard-coded to specific versions like "v3.5.14". Documentation polish, not architecture. |
| Bonus | Renamed W3 from "Kernel integration" → "Kernel Proof" | Everything in Phase 1 exists to prove the kernel behaves correctly. Subtle but important framing change. |

### 1.2 Impact on Implementation Scope

**No change to scope.** The four nits are renaming and documentation only:

- `NormalizedEvidence` struct loses one field (`Confidence`)
- Package name changes from `claim` to `derive`
- Type name changes from `ClaimAction` to `DeriveAction`
- Function name changes from `Extract` to `Derive`
- Wave W3 renamed in documentation

The actual implementation logic is unchanged. No new files. No removed files. No architectural changes.

### 1.3 Implementation Wave Order (Unchanged)

| Wave | Packages | Purpose |
|---|---|---|
| W1 | `internal/normalize` | Domain-neutral evidence model |
| W2 | `internal/derive` | Belief derivation rules |
| W3 | `internal/belief`, `internal/intent` | Kernel Proof |
| W4 | Fixtures + unit tests | Determinism and correctness |

---

## 2. Adversarial Code Review Prompt

**Scope:** All Go code created during Phase 1 implementation of `plan3.md`.
This includes: `internal/normalize`, `internal/derive`, `internal/belief`,
`internal/intent`, and all `_test.go` files in those packages.

**Posture:** You are adversarial. Your job is to find problems, not to praise.
Every function you don't challenge is a function you've implicitly certified as
correct. Be explicit about what you certified and what you didn't.

---

### Instructions

1. Read all Go source files in `internal/normalize/`, `internal/derive/`,
   `internal/belief/`, `internal/intent/` — production code and tests.
2. Read `IMPLEMENTATION_CONTRACT.md` — the frozen architecture contract.
3. Read `DATA_PIPELINE_SPEC.md` — the authoritative domain specification.
4. Execute Pass 1 (Distributed Systems Engineer).
5. Execute Pass 2 (Data Engineer).
6. Execute Pass 3 (Security Engineer).
7. Execute Cross-Pass Synthesis.
8. Write the review document following the format below.

---

### Pass 1: Distributed Systems Engineer

You are a senior distributed systems engineer who has spent a decade debugging
production systems where "the application was supposed to handle this."

#### Attack Vectors

For each Go file, ask:

1. **Error handling:** Does every function return errors consistently? Are
   errors wrapped with context? Are there silent failures that could produce
   corrupt state?

2. **Idempotency:** Can the same function be called twice with the same inputs
   and produce the same outputs? Is there hidden state that violates this?

3. **Transaction safety:** Does `ProcessActions` (belief.go) execute kernel
   operations inside a transaction? What happens if it crashes mid-sequence?

4. **Nil/zero handling:** What happens when `NormalizedEvidence` fields are
   zero values? Does the code handle missing optional fields gracefully?

5. **Panic safety:** Are there any nil pointer dereferences, index-out-of-bounds,
   or type assertion failures that could panic in production?

6. **Concurrency:** If two goroutines call `ProcessActions` for the same scenario
   simultaneously, what happens? Does the kernel's serialization retry handle this?

#### Questions to Force

- Can `Derive` (derive.go) return an empty action list for valid evidence?
- Can `ProcessActions` (belief.go) leave the system in a half-applied state?
- Does `ProposeIntent` (intent.go) handle the case where the belief doesn't exist?
- Are all `DeriveAction.Type` values handled in `ProcessActions`?
- What happens if `Derive` returns an action with an empty `BeliefID`?

#### Verdict Format

```
### Pass 1 Verdict

**Status:** BLOCKED / CONDITIONALLY APPROVED / APPROVED

**Blockers:** (numbered list — must be fixed before implementation)

**Nits:** (numbered list — non-blocking improvements)

**Certified Correct:** (list of functions/files you challenged and found sound)
```

---

### Pass 2: Data Engineer

You are a senior data engineer who has built ingestion pipelines that process
millions of events per day and have seen every form of upstream data corruption.

#### Attack Vectors

For each normalization function in `internal/normalize/`, ask:

1. **Determinism:** Does `Normalize` produce identical `content_sha256` for
   identical inputs across runs? What about floating-point fields, timestamps
   with different timezone representations, or JSON field ordering?

2. **Missing fields:** What happens when a required field is missing from the
   JSON payload? Does the function return an error, or proceed with a zero value?

3. **Hash stability:** If the same logical event produces slightly different JSON
   (e.g., different field ordering in the upstream API), does the normalization
   produce the same hash?

4. **Fixture coverage:** Do the captured JSON fixtures in `testdata/` cover all
   branches in the normalization code? Are there edge cases not covered?

5. **Version parsing:** How does the code handle version strings that don't
   follow semver? What about "3.5.x", "latest", or empty strings?

6. **SHA-256 computation:** Is the hash computed on the normalized form or the
   raw form? Is it deterministic?

#### Questions to Force

- What happens when `normalizeGitHubIssue` receives JSON with no `labels` field?
- Can `normalizeKEVEntry` parse a CVE ID that doesn't match `CVE-\d{4}-\d{4,}`?
- Does `normalizeRelease` handle pre-release versions correctly?
- What happens if the same evidence is normalized twice — does it produce the
  same `content_sha256`?
- Are there any code paths that silently return a zero `NormalizedEvidence`
  instead of an error?

#### Verdict Format

```
### Pass 2 Verdict

**Status:** BLOCKED / CONDITIONALLY APPROVED / APPROVED

**Blockers:** (numbered list)

**Nits:** (numbered list)

**Certified Correct:** (list of functions you challenged and found sound)
```

---

### Pass 3: Security Engineer

You are a security engineer who models threats against autonomous systems.
You assume the upstream feeds are untrusted and that an attacker's goal is to
make the agent act on a false belief or fail to act on a true one.

#### Attack Vectors

1. **Input validation:** Do normalization functions validate inputs before
   processing? Can malformed JSON cause panics or silent corruption?

2. **Injection attacks:** The input model includes free-text fields (issue body,
   comment body). Can these contain SQL injection payloads? Does the code
   sanitize them before passing to the kernel?

3. **Claim injection:** Can an attacker craft a GitHub issue body that contains
   a fake CVE ID matching `CVE-\d{4}-\d{4,}`? Does the normalization function
   validate the CVE against any external source?

4. **Maintainer impersonation:** Can an attacker post a comment from a non-maintainer
   account that matches the maintainer keyword patterns? Is the maintainer
   list the only trust boundary?

5. **Denial of service:** Can an attacker flood the pipeline with evidence
   items to prevent real beliefs from being promoted? Is there rate limiting
   at the normalization layer?

6. **Replay attacks:** Can old evidence be replayed to re-retire debt items
   that were legitimately retired? Is there temporal validation?

7. **Belief poisoning via deduction:** Can an attacker inject evidence that
   triggers the "All Debt Retired → Ready for Promotion" rule, promoting a
   false belief before legitimate debt retirement completes?

8. **Fixture manipulation:** If fixtures are ever loaded from external sources
   (not embedded), can an attacker modify them to bypass claim extraction?

#### Questions to Force

- What prevents an attacker from creating a fake GitHub issue with a fake CVE
  ID that passes the CVE pattern validator?
- Does `Derive` (derive.go) trust the `NormalizedEvidence` completely, or does
  it perform its own validation?
- Can an attacker control the `content_sha256` hash by carefully constructing
  the upstream payload?
- Is there a maximum evidence count per belief? Can an attacker exhaust
  storage by attaching millions of evidence items to one belief?
- What prevents an attacker from injecting SQL into free-text fields that are
  later used in SQL queries?

#### Verdict Format

```
### Pass 3 Verdict

**Status:** BLOCKED / CONDITIONALLY APPROVED / APPROVED

**Blockers:** (numbered list)

**Nits:** (numbered list)

**Certified Correct:** (list of functions you challenged and found sound)
```

---

### Cross-Pass Synthesis

After all three passes, synthesize:

#### Aggregate Verdict

The overall status is the most restrictive of the three pass verdicts.
If any pass returns BLOCKED, the aggregate is BLOCKED.

#### Must-Fix Before Implementation

Combine all blockers from all three passes. Deduplicate. Prioritize by
severity: anything that could produce corrupt state or enable adversarial
manipulation is P0; anything that could cause pipeline failure under
normal operation is P1; everything else is P2.

#### Should-Fix (Non-Blocking)

Combine all nits from all three passes. These are improvements that would
strengthen the code but don't block implementation.

#### Design Questions (Escalate to Architect)

Questions that require a design decision, not just a code fix. These are
escalated to the architect for ruling. Format each as:

```
DQ-N: [question]
Options: [option A] / [option B] / [option C]
Recommendation: [which option and why]
```

---

### Scope Check

Confirm that the review stayed within bounds:
- Reviewed only Go code in `internal/normalize/`, `internal/derive/`,
  `internal/belief/`, `internal/intent/` and their test files
- Did not review kernel code (`internal/kernel/`)
- Did not review schema DDL (`db/001_schema.sql`)
- Did not review Phase 1 docs or plan documents
- Did not propose architecture changes
- Did not add new tables, agents, or evidence feeds beyond what the plan defines

---

### Authorization

The review authorizes (or does not authorize) proceeding with Wave 3 (Kernel
Proof) and Wave 4 (Tests). If BLOCKED, the review must name the specific
blocker that prevents authorization and the minimum fix required to unblock.

---

### Decision Log

Record every decision made during the review, with the reasoning:

```
- DR-N: [decision] — [reasoning]
```

---

### Output

Write the review to `plans/PRD/PLAN3_CODE_REVIEW.md`.

---

## 3. Checklist Before Review

Before launching the adversarial review, confirm:

- [ ] All `internal/normalize/` code compiles (`go build ./internal/normalize/...`)
- [ ] All `internal/derive/` code compiles (`go build ./internal/derive/...`)
- [ ] All `internal/belief/` code compiles (`go build ./internal/belief/...`)
- [ ] All `internal/intent/` code compiles (`go build ./internal/intent/...`)
- [ ] All tests pass (`go test ./internal/normalize/... ./internal/derive/... ./internal/belief/... ./internal/intent/...`)
- [ ] No vet errors (`go vet ./...`)
- [ ] Kernel tests still pass (`go test ./internal/kernel/...`)
- [ ] B-14 test updated to assert embedding column exists and is nullable
