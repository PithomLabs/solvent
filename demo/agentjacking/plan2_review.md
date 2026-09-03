Yes — **this revised plan is much closer to the architecture I would actually approve for implementation.** The critical change is that Sentry/Agentjacking is now treated as an adapter concern rather than something that expands the generic normalize/derive/kernel vocabulary. The plan explicitly states “zero changes to the generic core,” with the adapter constructing existing `NormalizedEvidence` directly and producing a fixed-shape `DerivedBelief`. 

There are, however, **two things I would correct before implementation**, one architectural and one practical.

## 1. The six-debt clarification is excellent

This was an important correction.

The plan now says the prompt's earlier debt names were stale and that the real code uses:

```text
needProvenanceCheck
needContradictionSweep
needBlastRadius
needRollbackPlan
needVersionPin
needOperatorSignoff
```

via `kernel.FullDebt`, rather than hardcoding a list. 

That is exactly how the coding agent should implement it.

**Do not hardcode those six strings into the Agentjacking package.** Use `kernel.FullDebt` everywhere tests need to assert the starting debt. That keeps the demo coupled to the actual kernel contract rather than duplicating it.

## 2. `internal/agentjacking` is the right boundary

This is the biggest improvement.

The proposed package:

```text
internal/agentjacking/
    Event
    Parse
    DetectCommands
    BuildEvidence
    Belief
    Ingest
```

is a reasonable boundary because Sentry semantics remain encapsulated there. The generic pipeline sees only the existing evidence/belief abstractions. 

That gives you:

```text
Sentry-specific
      ↓
internal/agentjacking
      ↓
generic Solvent types
      ↓
existing belief/debt machinery
      ↓
existing DB invariants
```

rather than:

```text
Sentry
 ↓
normalize knows Sentry
 ↓
derive knows Sentry
 ↓
debt model knows Sentry
```

That is the architecture I wanted.

---

# One thing I would change: `StripMarkdown`

The plan proposes:

> “one tiny generic change: export `StripMarkdown` from `internal/normalize` as a 3-line wrapper.” 

This is defensible, but **I would avoid it if possible**.

You're trying very hard to preserve the principle:

> Sentry-specific functionality lives outside the generic core.

Adding an exported normalization helper isn't a serious violation, but it weakens the boundary unnecessarily.

I would prefer one of these, in order:

```text
1. Put a tiny markdown helper inside internal/agentjacking
2. Reuse an already-exported generic utility if one exists
3. Only then export StripMarkdown from normalize
```

The concern isn't three lines of Go. It's API surface.

Once `normalize.StripMarkdown` becomes exported, it becomes a generic package API that future code may begin depending upon. For a demo-specific integration, that's unnecessary architectural gravity.

So I would change the prompt to:

> **Do not modify `internal/normalize` merely to expose `stripMarkdown`. Reuse an existing exported utility if available; otherwise keep the adapter's markdown normalization local and small. Do not duplicate a substantial parser.**

That makes the zero-core-change claim literally true.

---

# Another thing to watch: `Ingest()` is starting to become too powerful

The proposed:

```go
Ingest(ctx, db, scenarioID, fixturePath)
```

does:

```text
Parse
→ BuildEvidence
→ Belief
→ EnsureBelief
→ Process
→ GetSnapshot
```

The plan says this is still adapter code. 

That's acceptable for the demo, but I would keep a very strict distinction:

```text
Parse / BuildEvidence / Belief
    = pure adapter logic

Ingest
    = demo orchestration
```

Do **not** start adding business rules to `Ingest`.

In particular, it should not decide:

* whether something is promotable;
* whether evidence retires debt;
* whether an action is authorized;
* whether a Sentry command is dangerous.

Those remain existing Solvent responsibilities.

---

# One subtle issue with `SourceType = "sentry_error"`

This is now acceptable because the plan explicitly makes it an **adapter-level string**, not a `normalize.SourceSentryError` constant. 

That's an important distinction.

The adapter can say:

```go
SourceType: "sentry_error"
```

inside the evidence it constructs, because the generic evidence structure already permits a source-type string.

But the generic packages should **not switch on it**.

So this invariant should be enforced:

```text
"sentry_error" must not appear in:
    internal/derive
    internal/belief
    kernel
    db schema
    pipeline source registry
```

The plan already explicitly says no `pipeline.sourceTypeMap` registration. 

Good.

---

# The `--reset` concern

This line deserves care:

> `--reset` clears only the demo scenario's rows (scoped DELETEs, demo-boundary concern — never in the kernel/MCP). 

Architecturally, I agree with keeping it outside the kernel/MCP.

But I would **not casually add SQL DELETE logic to `demo/agentjacking/ingest`** without examining how the repository's existing demo reset mechanism works.

Prefer:

```text
existing reset mechanism
    ↓
Track 3 scope
```

over introducing a one-off SQL deletion implementation.

If there isn't an existing scoped-reset utility, then a demo-only reset is reasonable, but make it obviously disposable and isolated from production packages.

---

# The MCP plan is now particularly strong

The revised plan captures the distinction correctly:

```text
tool_output
    → immediate refusal
    → no DB
    → no audit

user_typed
    → existing kernel path
    → DB gate
```

The plan explicitly uses the absence/presence of the audit envelope as the observable fingerprint of whether the DB path was reached. 

That's excellent for a live demo because the audience can literally see:

```text
Layer 4:
NO audit envelope
NO DB

then:

Layer 3:
23503
audit envelope present
```

That makes the architecture tangible.

---

# The detector-miss test is exactly right

This is arguably the best addition in the revised plan:

```text
sentry_error_nodetect.json
→ embedded_commands = []
→ claim still non-actionable
→ evidence intact
→ six debts remain
→ promotion fails
→ authorization fails
```

The plan explicitly calls this out as Test D. 

That proves an important architectural fact:

> **The detector is observability, not security.**

I would preserve this exactly.

---

# One more important test improvement

The current plan says the pure adapter test checks:

> `message_raw` byte-identical to fixture message

Be careful here.

The **fixture file itself** is the raw artifact, but `message_raw` is the message field inside the JSON. Those are not the same byte stream.

The plan correctly says the ledger evidence SHA is the SHA of the **raw fixture bytes**. 

So tests should distinguish:

```text
fixture raw bytes
    → ContentSHA256

JSON message field
    → message_raw
```

Do not accidentally calculate the evidence SHA from `message_raw`.

That would break the forensic traceability you're trying to demonstrate.

---

# Final architecture

After this revision, the architecture is clean:

```text
                 SENTRY
                   │
                   ▼
        ┌─────────────────────┐
        │ internal/agentjacking│
        │                     │
        │ Parse               │
        │ DetectCommands      │
        │ BuildEvidence       │
        │ BuildBelief         │
        └──────────┬──────────┘
                   │
                   ▼
          generic Solvent types
                   │
                   ▼
            belief.Process
                   │
                   ▼
            kernel.FullDebt
                   │
             ┌─────┴─────┐
             ▼           ▼
          Promote     Authorize
             │           │
          23514        23503
```

The generic core doesn't know whether the evidence came from:

```text
Sentry
Datadog
GitHub
a log
a scanner
some future observability platform
```

That is exactly what we want.

## My final verdict

**Greenlight with two small amendments:**

**1. Avoid exporting `StripMarkdown` from `internal/normalize` unless there is genuinely no better reuse option.** Keep the generic core untouched if practical.

**2. Keep `Ingest()` strictly as adapter/demo orchestration; do not let it acquire Solvent business-policy semantics.**

Everything else in this revised plan is aligned with the architecture, including the Track 3 centralization, DB-free Layer 4 validation, detector-miss proof, real `kernel.FullDebt` assertions, and explicit pre-v0 scope.  

**I would now send this to the coding agent.**
