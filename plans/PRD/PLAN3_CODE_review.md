## Technical Lead Review

**Verdict: REWORK REQUIRED**

This is, by a considerable margin, the best adversarial review we've had on the project. Unlike many AI reviews, it doesn't invent architecture changes—it attacks the implementation against the frozen specification with empirical probes and provides traceability back to the spec.  

However, I **do not accept all of its blockers**. Some are true specification violations; others are design choices that should be deferred or reconsidered.

---

# Overall Score

| Category                   |      Score |
| -------------------------- | ---------: |
| Scope discipline           |      10/10 |
| Empirical verification     |      10/10 |
| Specification traceability |      10/10 |
| CockroachDB awareness      |       9/10 |
| Prioritization             |       7/10 |
| **Overall**                | **9.4/10** |

---

# Accept as Blockers

These must be fixed before Wave 1 is frozen.

## ✅ B1 — `IngestedAt` purity

I completely agree.

Normalization should be a pure function.

`IngestedAt` belongs to the ingestion layer, not the normalization layer. 

**Accepted (P0).**

---

## ✅ B2 — Fabricated `ObservedAt`

Completely agree.

Never fabricate upstream truth.

If the upstream timestamp is missing:

* return an error, or
* leave it zero and let the caller decide.

Do not silently substitute `time.Now()`. 

**Accepted (P0).**

---

## ✅ Req1 — Required-field validation

Agreed.

A normalized object should never produce:

```
.../issues/0
```

That's exactly the kind of silent corruption we're trying to eliminate. 

**Accepted (P1).**

---

## Accept with Modification

## ⚠️ Req4 — Better canonicalization tests

Absolutely.

We already asked for this.

Add:

* reordered JSON fixture
* negative fixture
* pinned golden hash

Accepted. 

---

# I do **not** accept these as Wave 1 blockers

## ❌ Req2 — Include observation identity in `content_sha256`

I disagree with the review here. 

This is not an implementation bug.

It is a **semantic decision**.

The question is:

> Does `content_sha256` identify **content** or **observations**?

Those are different things.

I actually prefer:

```
content_sha256
```

to represent

> identical semantic content.

If later we need observation identity,

that should probably be a different key.

Do **not** change the hash without revisiting the ledger semantics.

**Demote to Design Decision.**

---

## ❌ Req3 — Store raw payload instead of normalized payload

I partially disagree. 

The review assumes the spec requires raw bytes.

But we deliberately introduced a **Canonical Evidence Model**.

If the implementation stores canonicalized evidence,

that's a perfectly valid architectural choice.

If we want immutable raw payloads,

let's change the specification first.

Do not silently change implementation.

**Demote to Specification Clarification.**

---

# Should Fix

## S1 — Severity classifier

Agree.

Word-boundary matching is better than substring matching. 

Not blocking.

---

## S2 — Mutable maintainer map

Agree.

Easy improvement. 

---

## S3 — Validate URLs

Reasonable.

Low priority.



---

# Excellent Findings

The strongest contribution of this review isn't actually the bugs.

It's the methodology.

This section:

```
same input

↓

same hash

↓

verified
```

is exactly how adversarial reviews should look.

Evidence.

Not speculation. 

---

# One thing I want to preserve

The Specification Traceability table.

Excellent.

I want this in every future review. 

---

# Revised Required Change Set

I would send this back to Claude Code:

## Must Fix

* Remove `time.Now()` from normalization (`IngestedAt`).
* Never fabricate `ObservedAt`.
* Enforce all required fields.
* Add canonicalization test.
* Add negative fixture.
* Add pinned golden hash.

## Defer

* Hash identity semantics (`content_sha256`) → Technical Lead decision after Wave 2.
* Raw vs. canonical payload → Specification clarification.

## Should Fix

* Word-boundary severity matching.
* Immutable maintainer registry.
* URL validation.

---

# Technical Lead Decision

```markdown
REWORK REQUIRED
```

**Reason:** Three implementation issues (purity, timestamp provenance, required-field validation) genuinely violate the intended behavior and should be corrected before Wave 1 is frozen. The review also surfaced two broader design questions—hash identity semantics and raw versus canonical payload storage—that should **not** be "fixed" in code without first resolving them at the specification level. Those are architecture decisions, not implementation defects.

Once the accepted fixes are applied, rerun:

```bash
go build ./...
go vet ./...
go test ./internal/normalize/... -v
```

Then submit the updated Wave 1 for a short verification review. If those checks pass, I would approve freezing Wave 1 and authorize work on Wave 2.
