Yes. **This is now the version I would greenlight for implementation.**

The architectural boundary is now clean:

```text
Sentry fixture
    ↓
internal/agentjacking        ← Sentry-specific knowledge lives here
    ↓
generic Solvent types
    ↓
existing belief/debt machinery
    ↓
existing kernel + CockroachDB invariants
```

The plan explicitly makes the zero-core-change claim literal: no `SourceSentryError`, no `deriveFromSentry`, no `DebtMapping` entry, no pipeline registration, and no changes to `normalize`, `derive`, or the kernel. 

### What is particularly strong now

**The six-debt assertion is correctly anchored to `kernel.FullDebt`.** That avoids duplicating the kernel's debt vocabulary and makes the test validate the actual contract rather than the plan's stale names. 

**The detector is correctly demoted to audit metadata.** The nodetect fixture is the important proof: the security property survives even when `DetectCommands` fails.  

**Beat 3/4 is exactly right.** `tool_output` gets rejected without DB access, while a lying agent declaring `user_typed` reaches the genuine DB gate and gets `23503`. The plan deliberately does not content-filter the `npx` action in Beat 4, which is essential to demonstrate that `action_source` is not pretending to be trustworthy.  

**The pre-v0 scope is explicit.** The README will distinguish the current belief-promotion/action gate from the future exact-action authority model, so this demo cannot later be misread as proving the confused-deputy case is solved. 

**The SHA distinction is correct.** Fixture bytes are hashed for evidence provenance, while `message_raw` is the JSON message field. That's an important forensic distinction and is now explicitly tested. 

### One thing I would simply enforce during implementation

The `sentry_error` containment grep is a good guardrail, but make sure it excludes the legitimate adapter package itself. The intended rule is effectively:

```text
allowed:
internal/agentjacking/**

forbidden:
internal/normalize/**
internal/derive/**
internal/belief/**
internal/pipeline/**
kernel/**
db/**
```

The plan already says this conceptually. 

### Final architectural verdict

**GREENLIGHT.**

I would not reopen the design.

The implementation should now be judged against the invariant:

> **Sentry is an input format to the adapter, not a concept Solvent's core needs to understand.**

And the demo's central proof remains:

```text
poisoned telemetry
    ↓
evidence, not authority
    ↓
six debts untouched
    ↓
23514 on promotion
    ↓
23503 on attempted action

while:

human-reviewed belief
    ↓
promoted
    ↓
legitimate action succeeds
```

That is a compact, honest demonstration of Solvent's existing authority boundary without architectural bloat.
