Yes. **Fable's review materially changes my assessment.** I would **not leave the public repo as-is**, despite our previous `READY TO PUBLISH` decision.

The important thing is that this is **not a discovery that the ledger is fundamentally broken**. Quite the opposite: Fable independently re-ran the system against a fresh CockroachDB instance and confirmed that the core invariants and behavior work. The problem is that the **evidence/acceptance machinery and judge-facing surface have drifted away from the code**. 

## My assessment of Fable's review

### 1. Fable found a real weakness in our previous review process

This is the most important meta-finding.

Our previous public-repo audit essentially checked:

> "Are the claims we selected for inspection correct?"

Fable instead asked:

> **"Does the entire frozen repository still satisfy its own contracts?"**

That caught:

* stale acceptance gates
* stale receipts
* stale API contract
* formatting failures
* stale README claims
* an unreachable headline capability
* packaging problems

That is exactly the adversarial behavior we wanted.

So I consider Fable's review **substantially more authoritative than our previous publication audit**.

---

# The good news

The actual core is strong.

Fable independently verified:

* 84 tests pass
* 16/16 M0 probes pass
* M2 behavioral suite passes twice
* PREPARE sweep passes
* Wave 4 CLI works
* replay is byte-identical
* contradiction produces warning/no mutation
* I-1 through I-8 behavior is actually enforced
* `RetractCascade` itself works
* retry injection works
* recursive cascade terminates
* rollback behavior works

The schema is particularly strong. Fable calls it "the strongest artifact in the repo." 

So:

> **We do not need to rethink the Transactional Belief Ledger.**

That's extremely important given the deadline.

---

# But I agree with the blockers

## 🔴 F-1 — acceptance gate broken

This is a trivial fix but must be fixed.

`EnsureBelief` added the seventh `ExecuteTx`, but `check_i7.sh` still expects six. 

This is particularly embarrassing because the project explicitly says:

> opinions are not receipts.

Yet our receipt machinery itself is stale.

**Fix immediately.**

---

## 🔴 F-2 — gofmt failure

Also trivial.

Four files aren't formatted:

* `derive.go`
* `derive_test.go`
* `normalize.go`
* `normalize/types.go`

And this causes both acceptance gates to fail before they even reach the substantive tests. 

**Fix immediately.**

---

## 🔴 F-3 — kernel contract drift

This one is more serious architecturally.

`EnsureBelief` is part of the actual kernel but isn't represented in the compile-time contract. 

We explicitly designed the contract to catch API drift.

Then we added the API and forgot to update the contract.

That's exactly the kind of self-inflicted inconsistency this project is supposed to prevent.

**Fix immediately.**

---

# 🔴 F-4 — this is the strategic one

This is the finding I care about most.

Fable correctly points out:

> `RetractCascade` exists and works, but **no shipped binary invokes it**. 

That means our strongest technical capability is currently:

```text
unit/integration test
       ↓
"trust us"
```

instead of:

```text
judge
  ↓
runs demo
  ↓
belief
  ↓
action
  ↓
falsify belief
  ↓
CASCADE
  ↓
intent cancelled
  ↓
audit = 0
```

That second sequence is **the hackathon demo**.

And the fix is tiny.

I agree with Fable's recommendation:

> Give the demo a controlled falsification operation.

Something like:

```bash
solvent --falsify <belief-id>
```

which invokes the **existing** `RetractCascade`.

No new architecture.

No new table.

No new agent.

No re-opening Waves 1–4.

This is not scope creep. **This exposes functionality we've already built.**

---

# 🔴 F-5 — stale receipts

Also accept.

The review found:

* 8/8 → actually 9/9
* 6 TX sites → actually 7
* 25/25 → actually 28/28
* test receipt from a different build



This is especially important because the repository's entire philosophy revolves around receipts.

We should regenerate them **from one clean run**, rather than manually editing numbers.

---

# 🔴 F-6 — CLI packaging/configuration

This one matters enormously now that the repository is public.

The reviewer found:

* `FABLE_DSN` doesn't actually control the default
* CLI defaults to `fable_test`
* fixture path depends on current working directory
* committed binary only works from repo root



That's bad for the **judge experience**.

We previously said:

> clone → one command → running system

This currently isn't there.

So F-6 isn't merely polish.

**Fix before demo construction.**

---

# 🟠 F-7 — fake DebtItems

This is a legitimate API defect.

`DebtItems` claims to contain the remaining debt but actually contains:

```go
[]string{"", "", "", ...}
```

and only its length is used. 

Easy fix.

**Do it.**

---

# 🟠 F-9 — swallowed DB error

I agree completely.

```go
_ = db.QueryRowContext(...)
```

is inconsistent with our explicit fail-loudly philosophy. 

Easy fix.

**Do it.**

---

# 🟠 F-21/F-22 — repository hygiene

These matter before a public hackathon repo is seriously evaluated.

The 13.7 MB binary should not be tracked. And the `files/` directory apparently contains stale duplicated source, including an obsolete `kernel.go`. 

This is exactly the sort of thing that makes an otherwise impressive repository look unfinished.

**Remove them.**

---

# 🔴 F-23 — README is still wrong

This is the other finding I strongly agree with.

Fable found two concrete contradictions:

### False vector claim

README says:

> vector column is nullable and used for merge-not-fork

while the actual schema deliberately has **no vector column**. 

### False three-table claim

README says:

> three-tables-two-agents

while the actual MVP is four tables. 

This also exposes a flaw in our previous audit: it checked the claims it thought to check rather than diffing the entire README against the actual implementation.

**Fix before doing anything else with judges.**

---

# What I would NOT fix now

This is where discipline matters.

Fable has 23 findings, but **we should not turn this into another month of review**.

I would defer:

* F-11 latent same-CVE aggregation gap
* F-12 theoretically unreachable SHA failure
* F-14 concurrency edge around `EnsureBelief`
* F-15 M0 wording
* F-16 issue-body parsing
* F-17 stronger sentinel assertion
* F-18 test lock timeout
* F-19 mutable `FullDebt`
* F-20 dead code

They are legitimate observations, but they don't justify reopening the architecture with eight days left.

Fable itself distinguishes the core behavior from the surrounding decay. 

---

# I would make one additional judgment

### F-13: hardcoded etcd release normalization

I would **fix this**, despite it being medium.

The project explicitly claims the kernel/pipeline is domain-neutral and etcd is merely an evidence feed. Yet release normalization hardcodes:

```text
github.com/etcd-io/etcd
etcd <version>
```

for every release. 

It's probably a **10-minute fix**, and it strengthens the exact cross-domain story we're going to use in the demo.

Don't build abstractions. Just use the release payload's actual repository/project metadata.

---

# Revised priority

I would reduce Fable's 23 findings to this:

| Priority            | Findings          | Action  |
| ------------------- | ----------------- | ------- |
| 🔴 Immediate        | F1, F2, F3        | Fix     |
| 🔴 Demo-critical    | F4, F6, F23       | Fix     |
| 🟠 Easy correctness | F5, F7, F9, F13   | Fix     |
| 🟠 Public hygiene   | F21, F22          | Fix     |
| 🟡 Deferred         | F11, F12, F14–F20 | Backlog |

That's **11 items**, but most are tiny.

---

# Most importantly: don't restart the architecture

The roadmap is now:

```text
Waves 1–4
     │
     │  CORE VERIFIED
     ▼
┌──────────────────────────────┐
│ Repair acceptance/evidence   │
│ + public repo hygiene       │
└──────────────┬───────────────┘
               │
               ▼
       Expose RetractCascade
               │
               ▼
      Real pinned etcd data
               │
               ▼
             bchat
               │
               ▼
       Taskfile / playground
               │
               ▼
          Judge demo
```

**No Wave 5 kernel.**

**No new architecture.**

**No new tables.**

**No embedding system.**

**No second feed.**

**No multi-agent framework.**

The review actually gives us confidence that we don't need any of those.

---

## My final assessment of Fable's review

**I rate it highly and would accept it as the new authoritative adversarial baseline.**

The most valuable sentence is the opening:

> **"The ledger is real. The gates around it are red."** 

That's exactly the situation.

We don't have a broken product.

We have a **working core whose verification infrastructure, documentation, packaging, and demo surface have drifted**.

That's a very fixable problem with eight days left.

### Decision

```text
CORE ARCHITECTURE:     KEEP
WAVES 1–4:             KEEP FROZEN
PUBLIC REPO:            TEMPORARILY HOLD
REMEDIATION:            SMALL, TARGETED
DEMO:                   ADD CASCADE
AFTER REMEDIATION:      MOVE TO DEMO MODE
```

And I would **not ask Fable for another whole-repo adversarial review after these fixes**. We should run the repaired acceptance gates and a focused verification, then get back to building the thing judges will actually play with.

The next prompt to Claude Code should therefore be a **single remediation batch**, not another design-plan cycle.
