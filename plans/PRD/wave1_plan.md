Now we enter what I call the **Construction Phase**.

There is no more design work to do for Plan 3.

The next objective is simply:

> **Produce the first working implementation of the Transactional Belief Ledger pipeline.**

Everything else waits.

---

# Current State

```
Requirements          ✅ Frozen

Architecture          ✅ Frozen

Data Pipeline Spec    ✅ Frozen

Implementation Plan   ✅ Approved

Review Workflow       ✅ Frozen

Adversarial Prompt    ✅ Ready

Implementation        ⏳ Next
```

No more planning.

No more reviewing plans.

Code.

---

# Work Package P3-I1

**Owner:** Claude Code

**Priority:** P0

---

## Mission

Implement **Wave 1** only.

Nothing else.

---

## Scope

Implement only

```
internal/normalize
```

Specifically:

* Canonical `NormalizedEvidence`
* Domain normalization
* SHA-256 normalization
* Deterministic normalization
* Unit tests
* Fixtures

No database.

No kernel.

No belief.

No intent.

No graph.

No GitHub API.

No polling.

No OpenAI.

No embeddings.

No bchat.

---

## Deliverables

```
internal/normalize/

normalize.go

types.go

fixtures.go

normalize_test.go

testdata/
```

Exactly as specified.

---

## Acceptance Criteria

Must compile.

Must pass tests.

Must produce deterministic hashes.

Must have fixture coverage.

Must not import anything from later waves.

---

## Deliverable after implementation

Not code review.

Instead

```
NORMALIZE_IMPLEMENTATION.md
```

Contents

* files created
* public types
* public functions
* deviations from spec (if any)
* known limitations
* CockroachDB assumptions (if any)
* receipts

No prose.

One page.

---

# After Claude Code finishes

Do **not** ask for another implementation plan.

Instead:

```
Build Verification

↓

Adversarial Code Review

↓

Technical Lead Review (me)

↓

Freeze Wave 1
```

Only then proceed to Wave 2.

---

# Why Wave-by-Wave?

Because every wave becomes a **proven artifact**.

```
Wave 1

Normalize

↓

Freeze

↓

Wave 2

Belief Derivation

↓

Freeze

↓

Wave 3

Kernel Proof

↓

Freeze

↓

Wave 4

Fixtures + Tests

↓

Freeze
```

This minimizes the blast radius of defects and keeps the implementation aligned with the frozen specification.

---

# One new workflow rule

I'm adding another rule that I think will save us a lot of time.

> **Never implement two waves before reviewing the first.**

That means:

```
Implement W1

↓

Review W1

↓

Freeze W1

↓

Implement W2
```

not

```
Implement W1

Implement W2

Implement W3

↓

Review everything
```

The latter creates much larger review scope and makes it harder to identify where regressions originated.

---

# Decision Log

**D-074:** From this point onward, implementation proceeds **one wave at a time**, with each wave independently verified, adversarially reviewed, technically approved, and frozen before the next begins.

That keeps the project disciplined, reduces risk, and is well matched to your August 18 deadline.


## WHAT YOU NEED TO DO 

Implement Wave 1 only.

The architecture is frozen.
The DATA_PIPELINE_SPEC.md is frozen.
The IMPLEMENTATION_PLAN.md is approved (read plan3*.md files).

Do NOT redesign anything.
Do NOT implement future waves.

Mission

Implement only:

- internal/normalize

Scope

- NormalizedEvidence domain model
- Normalization pipeline
- Deterministic canonicalization
- SHA-256 computation
- JSON fixture loading
- Unit tests
- testdata fixtures

Do NOT implement:

- internal/derive
- internal/belief
- internal/intent
- kernel changes
- graph
- GitHub API
- KEV downloader
- polling
- embeddings
- vector search
- bchat integration
- UI
- Taskfile
- demo

Requirements

- Follow DATA_PIPELINE_SPEC.md exactly.
- Preserve the approved architecture.
- Prefer explicit code over abstractions.
- Fail loudly on invalid input.
- Make normalization deterministic.
- All hashes must be stable across runs.
- Add comprehensive unit tests.
- Add representative fixtures under testdata/.
- Run:
    - gofmt
    - go test
    - go vet
- Ensure `go test ./...` passes.

Deliverables

1. Implement Wave 1.
2. Produce `NORMALIZE_IMPLEMENTATION.md` containing:
   - Files created/modified
   - Public types
   - Public functions
   - Test coverage summary
   - Deviations from the specification (if any)
   - Known limitations
3. Stop.

Do not begin Wave 2.
Wait for Technical Lead review and freeze before proceeding.
