## prompt

what next


Next: **freeze the core and build the judge-facing playground.**

### Phase 1 — Reproducible demo workflow

Have the coding agent build:

1. **Taskfile**

   * `task setup`
   * `task db:up`
   * `task demo:track1`
   * `task demo:replay`
   * `task demo:track2`
   * `task test`

2. **One-command deterministic reset**

   * clean CockroachDB
   * load pinned fixtures
   * execute the appropriate scenario
   * produce readable output

3. **Judge inspection commands**

   * show beliefs
   * show evidence
   * show debts
   * show intents
   * show retraction/audit state

### Phase 2 — README / playground

Make the judge experience:

```text
git clone
↓
task setup
↓
task demo:track1
↓
"What just happened?"
↓
inspect ledger
↓
task demo:replay
↓
"Nothing duplicated."
↓
task demo:track2
↓
watch a previously authorized action get retracted
```

The README should explain the **principle through the actual etcd stories**, not drown judges in implementation details.

### Phase 3 — bchat

Only after the CLI playground is clean:

```text
bchat
  ↓
agent
  ↓
Solvent transactional belief ledger
  ↓
CockroachDB
```

Use bchat to let judges **ask questions and trigger the existing workflows**, rather than building a second domain-specific UI.

### Critical constraint

**Do not add new ledger semantics now.**

The verified core is the product. The remaining work is making its behavior **easy to experience, inspect, and understand**.

So the immediate next action is: **give the coding agent a Taskfile + playground implementation prompt.**



## prompt

just to clarify, task setup assumes there is no cockroachdb installed, right?


**Yes, that's the goal.** `task setup` should assume a **fresh judge machine** where CockroachDB may not be installed.

But we should be precise about what it does:

```text
task setup
  ↓
check prerequisites
  ↓
if CockroachDB absent
  → install/provision it using a supported reproducible method
  ↓
start CockroachDB
  ↓
initialize Solvent schema
  ↓
verify connection
  ↓
ready for demo
```

For the hackathon, I'd strongly prefer **Docker-based CockroachDB** if the judge machine has Docker, rather than requiring them to install CockroachDB manually.

So ideally:

```text
task setup
    ↓
Docker available?
    ↓
run pinned CockroachDB container
    ↓
create/reset database
    ↓
migrate schema
    ↓
ready
```

And separately:

```text
task db:up
task db:reset
task db:down
```

This gives judges a clean experience without permanently installing CockroachDB.

**Important:** `task setup` should **not silently install system packages** or modify the host unexpectedly. It should detect missing prerequisites and give a clear message if Docker/Go/etc. is unavailable.

So yes: **the intended UX is "clone → task setup → you're ready," with CockroachDB provisioned automatically rather than assumed to already be installed.**



## prompt

give the coding agent a Taskfile + playground implementation prompt.


I reviewed the playground implementation plan. **Approved with minor corrections — ready for implementation.**

The plan is appropriately narrow: Taskfile, Dockerized CockroachDB, demo scripts, inspection, README; no schema/kernel/architecture changes. 

### Two things I would fix before implementation

**1. `task demo:replay` is listed but `scripts/demo/replay.sh` is missing from Deliverables.**

The execution order creates it, and the replay design is present, but the deliverables list omits it. 

Add:

```text
scripts/demo/replay.sh
```

**2. Don't hard-code the DSN/database name in multiple places.**

The plan uses `fable` and `$FABLE_DSN`. Make the demo configuration explicit in one place, e.g.:

```text
SOLVENT_DB_NAME=fable
SOLVENT_DSN=...
SOLVENT_CRDB_CONTAINER=solvent-crdb
```

The scripts should source a common small configuration file or receive these consistently from Taskfile. This prevents `setup`, `reset`, and demos from drifting.

### One small design concern

The plan says:

> `docker run ... single-node insecure` 

For a **local hackathon playground**, that's fine. Just make the README explicitly say this is a **local disposable demo database**, not a production CockroachDB deployment.

### Otherwise: green light

The actual demo flow is exactly what we want:

```text
Track 1
fixtures
  ↓
pipeline
  ↓
operator review
  ↓
promotion
  ↓
live intent
```

and:

```text
Track 2
historical decision
  ↓
promotion
  ↓
live intent
  ↓
postmortem
  ↓
falsify
  ↓
retracted belief
  ↓
cancelled intent
```

The inspection command also exposes the four things judges actually need to see: beliefs, evidence, intents, and audit state. 

**Verdict: IMPLEMENT.**

No Claude/Fable review needed **before** implementation. Let the coding agent build this. Then, because this is a new judge-facing layer, we can do one focused verification of the actual fresh-machine experience afterward.



