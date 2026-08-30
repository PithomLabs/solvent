# Solvent — engineering notes, findings, and Devpost appendix

**Companion to [`DEVPOST.md`](DEVPOST.md).** The Devpost entry is the argument; this is the evidence
behind it, plus the questions a judge is most likely to push on.

Everything numeric here is measured. Where something is inferred rather than measured, it says so.

---

# Part I — What we set out to prove

Solvent is persistent memory for evidence, authority, and change. It is built around one separation:

```
RETRIEVAL          What does memory surface?
    ↓
JUDGMENT           What do we believe?
    ↓
AUTHORITY          What are we actually allowed to do?
```

The first two are allowed to be wrong. The third is not allowed to drift.

The sentence the whole project defends:

> **The model can propose, memory can be incomplete, and judgment can be wrong — but authority
> cannot silently survive after the belief it depended on has been invalidated.**

That word *silently* is doing the work. Nothing here prevents a bad decision. What it prevents is a
bad decision remaining **operative** after it has been falsified, without anyone noticing.

The concrete decision the demo turns on: **is etcd v3.5.x safe to deploy?**

---

# Part II — The measured findings

## II.1 — The retrieval counterexample

The corpus is **7,239 real etcd issues**, embedded with `amazon.titan-embed-text-v2:0` into a
`VECTOR(1024)` column and served by a CockroachDB vector index.

Ask it what an engineer would actually ask before a deployment:

```
"Is etcd v3.5.x safe to deploy?"
  #19220  0.372424   go.etcd.io/etcd/client/v3 updates
  #12987  0.387913   3.4 to 3.5 upgrade may panic
  #12788  0.401092   Using client in v3.5.0 and etcd server with v3.4.14
```

Every one of those is plausible, on-topic, and about upgrading etcd. None of them is the incident
that invalidates the deployment.

Reframe the same question around data integrity:

```
"etcd v3.5 data inconsistency after upgrade"
  #14139  0.199509   inconsistent data in etcd 3.5.4
```

`0.199509` against `0.372424`. Same corpus. Same index. Same embedding model. The only thing that
changed was the phrasing of the question.

And the canonical falsifier is worse still. `#13766 · Inconsistent revision and data occurs` — the
issue that actually destroys the claim — sits at distance `0.594920` from the belief's own text, a
measured **rank 573 of 7,239**.

> Rank 573 is an offline measurement we ran against the corpus. **The application does not render a
> rank.** On screen, `#13766` is introduced by URL and the interface says so:
> *"Not retrieved — introduced by URL."* In the video, the rank appears as an editorial caption, not
> as a UI field.

There is no top-k, no similarity threshold and no reranker that puts rank 573 in front of a decision
maker. This is not a tuning problem.

## II.2 — Distance is not relevance

The embedding is behaving **correctly**. That is the uncomfortable part.

A deployment-phrased question is genuinely, legitimately closer in vector space to release-process
material than to defect reports. Nothing is broken. The retrieval is doing exactly what it was
built to do, and the result is still useless for the decision at hand.

Three consequences we took seriously:

1. **You cannot fix this by lowering a threshold.** At rank 573 you would have to admit essentially
   the whole corpus to catch it, at which point ranking has told you nothing.
2. **You cannot fix this by asking better questions,** because the failure is invisible from inside
   the query. The deployment question returns confident, coherent, on-topic results. There is no
   signal that says "the thing you needed is not in here."
3. **Therefore retrieval cannot be the thing that grants authority.** Not because it is bad, but
   because its failure mode is silent.

That third point is the entire architecture in one line.

## II.3 — The provenance-collapse defect

We found this in our own code, before recording, and it is worth writing down because it is the same
class of failure the product exists to talk about.

Solvent's discharge screen has two retrieval-backed obligations:

- `needProvenanceCheck` — cite a retrieved issue
- `needContradictionSweep` — cite a retrieved issue

They are **independent obligations**. The provenance check asks "what did you rely on?"; the sweep
asks "did you look for something that contradicts you?"

The UI rendered them like this:

```
needProvenanceCheck      ✓ #14139 · 0.199509
needContradictionSweep   ✓ #14139 · 0.199509
```

Two debts, one issue, two identical receipts.

**Root cause, in two cooperating halves.** The projection kept a single "first considered citation"
and printed it against both rows:

```go
byRelation := map[string]Citation{}
for _, c := range cites {
    if _, seen := byRelation[c.Relation]; !seen { byRelation[c.Relation] = c }   // first wins
}
```

And the write path only asked whether *any* citation existed:

```sql
SELECT count(*) FROM belief_corpus_citation
 WHERE belief_id = $1::UUID AND relation = 'considered'
```

So the display was **not lying**. One citation genuinely was sufficient to retire both debts. The UI
was accurately reporting a model that was wrong. Fixing only the projection would have painted two
receipts over a one-citation reality — which is worse, because then the screen would have been
lying.

**The fix**, using data the schema already carried and changing no schema:

- `belief_corpus_citation` already stores `retrieved_at`, refreshed on re-selection, so it records
  *when the judge chose this evidence*. Ordering citations by it makes "which citation discharged
  which debt" answerable: the first thing you chose retires the first retrieval debt.
- The projection now walks those citations positionally, handing each retired retrieval check its
  own — and emits **no** receipt rather than reusing one, if none is left.
- The write path requires an *unconsumed* citation. Running out is a refusal, not a silent reuse.

After the fix:

```
needProvenanceCheck      ✓ #19220 · 0.372424
needContradictionSweep   ✓ #14139 · 0.199509
```

**Why it matters beyond the bug.** If one citation can satisfy two independent obligations, the
receipts are decoration. The demo's argument is that evidence is attributable — and attributable
means *this specific row paid this specific debt*, not "some evidence exists somewhere."

## II.4 — A second-order regression: correct by accident

Fixing II.3 broke something else, and this is the most instructive finding in the project.

The terminal screen closes with a sentence naming the evidence that discharged the contradiction
sweep. It picked that citation like this:

```js
const cited = (S.citations || []).find(c => c.relation === 'considered');
```

The *first* considered citation. That was **correct** — but only because citations were ordered by
**distance**, and the nearest one happened to be the sweep's. Nobody chose that. It was a
coincidence of two unrelated decisions.

The fix in II.3 reordered citations by selection time. The moment it shipped, "first" became the
provenance check's issue, and the closing frame of the demo rendered:

> `needContradictionSweep` was discharged by citing **#19220 at 0.372**.

directly beneath a receipt reading `✓ #14139 · 0.199509`. The final shot of the demo contradicted
itself.

It was caught by **reading the rendered frame**, not by a test — because no test asserted the
sentence, and the code had looked reasonable to everyone who read it.

**The repair** removed the guess entirely. The projection already knows which citation paid which
debt, so it now exposes the issue number and distance alongside the formatted receipt, and the
sentence reads the sweep's own binding:

```js
const sweep = (S.checks || []).find(c => c.item === 'needContradictionSweep');
const citedTxt = sweep && sweep.receipt_issue
  ? `#${sweep.receipt_issue} at ${Number(sweep.receipt_distance).toFixed(3)}`
  : 'a retrieved issue';
```

Recovering those numbers by parsing the display string would have been the same shortcut that caused
the original defect, so the values are carried explicitly.

The deployed service now renders, and this was verified against the live page before the final frame
was captured:

> `needContradictionSweep` was discharged by citing **#14139 at 0.200**. The sweep was performed. The
> judgment was wrong. Solvent recorded both, and refused to let the intent outlive the belief.

`0.200` is `0.199509` at three decimals — the same measured value the receipt above it shows in
full.

---

# Part III — The central insight: "correct by accident"

Put the findings side by side.

| # | What looked right | Why it was right | What made it wrong |
|---|---|---|---|
| 1 | Retrieval results for the deployment question | The embedding genuinely works | The question's phrasing, not the model |
| 2 | Two receipts on the discharge screen | The display accurately reported the model | The model allowed one citation to pay two debts |
| 3 | The closing sentence naming `#14139` | Citations happened to be distance-ordered | An unrelated ordering change |
| 4 | Cell 1 of the control experiment | Both transactions ran correct application logic | The schema permitted the interleaving |

Four independent cases. In every one, something was right **for a reason nobody had chosen**, and it
stopped being right when an unrelated thing moved. None of them announced the change. All four
committed successfully, rendered cleanly, and passed review.

This is the failure mode that matters for agent systems, and it is not "the model hallucinated." It
is quieter than that:

> **A system can be correct, coherent, and passing its checks, while the reason it is correct is an
> accident nobody is maintaining.**

Retrieval is correct by accident when the question happens to be phrased near the evidence.
Application logic is correct by accident when no concurrent transaction happens to interleave badly.
A projection is correct by accident when an incidental ordering happens to match the intent.

The only item in that table that could not drift is the one enforced by a constraint. Cell 3 does not
refuse because someone remembered to check — it refuses because the row cannot exist. That is the
difference between *being right* and *not being able to be wrong*, and it is why the authority
boundary lives in the schema.

It also reframes what the demo is showing. The judge is not watching a system that knows the truth.
They are watching a system where the consequences of being wrong are **persistent, attributable, and
structurally bounded**.

---

# Part IV — What the database actually enforces

Three schema-level mechanisms carry the entire guarantee.

```sql
-- 1. Debt must be empty before a belief can be promoted.
CHECK (status <> 'promoted' OR debt = '{}')          -- promoted_is_debt_free -> 23514

-- 2. An intent may only cite a belief that is promoted RIGHT NOW.
FOREIGN KEY (belief_id, belief_status)
  REFERENCES belief(id, status) ON UPDATE CASCADE     -- gate -> 23503

-- 3. And a live intent may not survive its belief losing that status.
CHECK (state <> 'live' OR belief_status = 'promoted') -- live_requires_promoted -> 23514
```

The second is a **composite** foreign key — it references `(id, status)`, not just `id`. That is what
makes an intent on a non-promoted belief *referentially impossible* rather than merely discouraged.

The third is the interesting one, and it only works because of a property we had to **measure rather
than assume**:

> **CockroachDB re-evaluates `CHECK` constraints on rows modified by `ON UPDATE CASCADE`.**

Retracting a belief cascades the new status into every citing intent, and the `CHECK` on those
modified rows then refuses the whole transaction. The application never performs this check. It
cannot forget to.

## The boundary we keep explicit

**CockroachDB does not traverse the belief graph.** The recursive walk over `belief_edge` is a
`WITH RECURSIVE` CTE in application code.

> **Belief traversal is application logic. CockroachDB enforces the invariant at the authority
> boundary.**

This distinction is easy to blur and we deliberately do not. What the database guarantees is not
"the graph was walked correctly" — it is that **the traversal cannot finish having left a live intent
attached to a non-promoted belief.** The application is free to have bugs in how it walks; it is not
free to commit the result if the walk was wrong.

If you go looking in the schema for the graph logic, it is not there, and that is intentional.

---

# Part V — The control experiment, and how to read it

Same race. Same correct application logic. Only the schema and isolation level change.

| Cell | Schema | Isolation | Observed | Ran on |
|---|---|---|---|---|
| 1 | naive | `READ COMMITTED` | no error, both transactions commit, **AUDIT = 1** | local CockroachDB v26.2.0 |
| 2 | naive | `SERIALIZABLE` | `40001 RETRY_SERIALIZABLE` | local CockroachDB v26.2.0 |
| 3 | hardened | `READ COMMITTED` | `23503 · gate` | deployed cluster, v26.2.5 |

Cells 1 and 2 run locally **on purpose** — the naive strawman schema must never touch the
judge-facing cluster. Cell 3 uses the real hardened table, so it runs against the deployed cluster.

The writer transaction reads the belief's status, sleeps, then inserts a live intent. The cascade
transaction cancels live intents in the subtree and retracts the belief. They are interleaved with a
fixed offset. Neither does anything wrong.

## Two readings we insist on

**`40001` is a retry signal, not a refusal.** Serializable detects the write-write conflict and
forces a retry. The harness deliberately does *not* retry, so the raw code is visible. Under
`crdb.ExecuteTx` the transaction would retry, re-read the belief's status on fresh state, and *then*
be refused by the constraint. The accurate chain is:

```
detect → retry → refusal on fresh state
```

Never "`40001` → transaction refused." That would misrepresent both the engine and the argument.

**Cell 1 was observed, not predicted.** The anomaly was inherited from a PostgreSQL-era claim and had
never actually been run against CockroachDB. We ran it. It reproduced. The result was left open until
it was measured, and it is reported as a measurement.

What cell 1 shows is the thesis in its sharpest form: **the corruption came from the schema, not from
a logic bug.** Both transactions were correct. Nobody made a mistake. The end state was a live intent
authorising action on a retracted belief, committed silently, with no error raised anywhere.

An error is a gift. Cell 1 is what happens when you do not get one.

---

# Part VI — Engineering lessons

**Tests must verify meaning, not execution.** Our suite passed for a while against a projection that
quietly reused one citation for two obligations. Every test was green; every test was asking whether
the code ran, not whether the answer meant anything. The regression test that now pins it asserts the
two receipts are *different* — a property, not a code path. It was verified to fail without the fix
before being trusted.

**A projection is where evidence quietly stops being evidence.** The database held two distinct
citations the entire time. No constraint could have caught the display collapsing them, because
nothing was violated. The only thing that catches this class of bug is comparing what the screen says
to what the model actually holds. We now read rendered output as part of verification, not just
assertions.

**A successful no-op is more dangerous than a crash.** Cell 1, and the receipt collapse, and the
closing sentence — none of them errored. Systems that fail loudly are the easy case.

**Verification that cannot fail is decoration.** During the final capture, our own gating assertion
failed and refused to capture anything. The assertion was looking for a receipt row on the terminal
screen — which renders no debt list at all, so there was nothing to find. The application was fine;
the check was in the wrong place. That is exactly what a gate is for, and it is worth designing checks
that are *capable* of being wrong in an observable way.

**Measure the unflattering number and publish it.** `0.594920` at rank 573 is not a good look for a
retrieval demo. It is also the most important number in the project. A submission that quietly used
a nicer distance would have been a worse submission and a dishonest one.

**Prefer the boring source of truth.** Every fix in this project came from using data the system
already had — `retrieved_at` was already stored; the receipt binding was already computed. The
temptation each time was to parse a display string or re-derive an ordering. That is how the original
defect happened.

---

# Part VII — Method, so the evidence is checkable

## Corpus provenance

**7,239** is a measured count, not an estimate and not GitHub's live issue counter. It is backed
three independent ways:

- **Fetch provenance** — the ingest metadata records `items_seen: 21617`,
  `pull_requests_excluded: 14378`, `issues_kept: 7239`, with a content digest of the NDJSON.
- **Embedding checkpoint** — `vectors_in_checkpoint: 7239`, model `amazon.titan-embed-text-v2:0`,
  `dimensions: 1024`, digest-pinned.
- **Counted from the database** — `corpus_issue` reports 7,239 embedded rows on both the local and
  the hosted cluster.

## The deploy-time assertion

The deploy does not print the retrieval numbers — it **asserts** them. Step 6 posts the exact query
`Is etcd v3.5.x safe to deploy?` to the deployed service and fails the deploy unless the nearest
result is `#19220` at `0.372424` within a tolerance of `5e-7`, on model
`amazon.titan-embed-text-v2:0`.

This exists because an early version printed the number instead of checking it, and a lowercase `is`
silently produced `0.365516` against documentation saying `0.372424`. The query text is
byte-sensitive: the embedding is of the literal string.

So the headline distances are re-verified against the live service on **every single deploy**. A
change of model, corpus or vectors fails the deploy rather than scrolling past.

## Why the demo cannot be staged

This is worth stating because "the demo is scripted" is a reasonable suspicion.

The wizard's screen is a **pure function of committed database rows** — not client state, not a step
counter. The server derives it:

```go
case hasContradiction || st.Status == "retracted":         return ScreenFalsify
case st.Status == "promoted" && st.IntentState == "live":  return ScreenFalsify
case st.Status == "entered" && len(st.Refusals) > 0:       return ScreenDischarge
case st.Status == "entered":                               return ScreenAsk
```

Two consequences:

1. **The arc cannot be clicked out of order.** The first logged refusal moves you from ASK to
   DISCHARGE permanently, and the search bar exists only on ASK. There is no back button — the
   stepper is inert text. Evidence must genuinely be selected before the refusals, or it cannot be
   selected at all.
2. **Reloading proves it.** Every screen redraws from committed rows. Nothing about the state lives
   in the browser.

This is also why the capture required two separate scenarios: the "no evidence selected" refusal and
the "evidence cited" success are mutually exclusive states of one belief. They cannot both exist in a
single run, which is a property of the model, not a limitation of the recording.



---

# Part VIII — Judge Q&A

**Isn't this just RAG with extra steps?**
No, and the difference is measurable rather than rhetorical. RAG's output is an answer. Solvent's
output is an *authorization*, and the authorization is a row that other rows depend on. The demo's
central number — rank 573 — is the proof that better retrieval was not the answer. We are not
claiming to retrieve better. We are claiming that retrieval must not be the thing that grants
permission.

**Then why not just fix the retrieval?**
Because the failure is silent from inside the query. The deployment question returns confident,
on-topic, genuinely related results. Nothing signals that the critical evidence is missing. And at
rank 573 of 7,239, no threshold or reranker recovers it without admitting the entire corpus. If we
could reliably detect this case, we would not need the architecture — that is precisely the point.

**Isn't `0.594920` just a bad embedding?**
No. The embedding is behaving correctly. `#13766` genuinely is far, in vector space, from the
sentence *"etcd v3.5.x has no known data-integrity defects"* — it discusses a revision/data anomaly
without restating the claim it refutes. Semantic distance and logical contradiction are different
relations. That is the finding, not a defect.

**Why not enforce this in application code?**
Cell 1 of the control experiment is the answer. Both transactions ran correct application logic and
the end state was still corrupt, because the *schema* permitted the interleaving. An application
check holds until someone adds a second code path, or two transactions interleave in a way nobody
modelled. A composite foreign key cannot be forgotten, cannot be bypassed by a new caller, and does
not have a code path.

**Does CockroachDB traverse the belief graph?**
No. The recursive walk over `belief_edge` is a `WITH RECURSIVE` CTE in application code. The database
enforces that the traversal cannot *finish* having left a live intent on a non-promoted belief. We
are deliberate about this because overstating it would be the easiest way to sound more impressive
and be less accurate.

**Is `40001` a refusal?**
No — it is a retry signal. Serializable detects the conflict and forces a retry; under
`crdb.ExecuteTx` the transaction retries and is then refused on fresh state. The harness does not
retry, so the raw code is visible. The accurate chain is detect → retry → refusal on fresh state.

**Why isn't rank 573 shown in the application?**
Because the application does not compute a rank, and rendering one would mean either computing an
ANN search for a row already chosen — a different question — or displaying a hardcoded number.
Neither is honest. The rank is an offline measurement, and in the video it appears as an editorial
caption, visually distinct from the interface.

**What if the agent cites evidence that is relevant but leads to the wrong conclusion?**
That is exactly what the demo shows, deliberately. The contradiction sweep is discharged by citing
`#14139` — a genuinely relevant integrity issue — and the conclusion is still wrong. Solvent does not
judge whether the reasoning was good. It records what was cited, how close it was, makes that
permanent, and then refuses to let the authorization outlive the belief. A demo where the human
reasons correctly and the database never has to intervene would demonstrate nothing.

**What happens if two retrieval obligations are discharged with the same evidence?**
They cannot be. Each retrieval debt consumes its own citation; running out is a refusal with its own
message. This was a real defect we found and fixed before recording — see II.3.

**What are you not claiming?**
See Part X. Briefly: no claim that the model is right, no claim that retrieval is complete, no
measured claim about vector index distribution, and no claim on CockroachDB's Cloud Managed MCP,
which is configured but not verified end to end.

---

# Part IX — Technical appendix

## The complete measured set

| Value | Meaning |
|---|---|
| `7,239` | ingested etcd issues, pull requests excluded |
| `amazon.titan-embed-text-v2:0` | embedding model |
| `VECTOR(1024)` | embedding column type |
| `#19220 · 0.372424` | nearest result, deployment question |
| `#12987 · 0.387913` | second result, deployment question |
| `#12788 · 0.401092` | third result, deployment question |
| `#14139 · 0.199509` | nearest result, integrity question |
| `#13766 · 0.594920` | canonical falsifier, `relation: contradicts` |
| rank `573 / 7,239` | `#13766` against the belief's claim — **offline measurement, not UI** |
| `deploy etcd v3.5.28` | the action intent |
| `live_on_nonpromoted = 0` | the audit invariant, non-vacuous while an intent is live |

The query strings are byte-sensitive and must be reproduced exactly:

```
Is etcd v3.5.x safe to deploy?
etcd v3.5 data inconsistency after upgrade
https://github.com/etcd-io/etcd/issues/13766
```

## SQLSTATE reference

| Code | Constraint | Raised when |
|---|---|---|
| `23503` | `gate` | authorizing an action on a belief that is not promoted |
| `23514` | `promoted_is_debt_free` | promoting a belief with outstanding debt |
| `23514` | `live_requires_promoted` | retracting a belief that still has a live intent |
| `23514` | *(none)* | application-raised: a retrieval check with no citation available |
| `40001` | *(none)* | serializable retry signal — **not** a refusal |

The fourth row carries **no constraint name deliberately**. It is the application refusing, not the
engine, and claiming a constraint name would be inventing engine output.

## Verification artifacts

| Artifact | What it proves |
|---|---|
| `proof/isolation.log` | The three-cell control experiment, with raw SQLSTATEs |
| `proof/act6_tier_probe.log` | `CHECK` re-evaluation on `ON UPDATE CASCADE`, verified on the deployed cluster |
| `docs/M2_TRANSCRIPT.md` | Kernel behaviour against a live cluster, per-case SQLSTATE and constraint |
| `docs/M1_I7.md` | Every write goes through `crdb.ExecuteTx`; no raw writes |
| `corpus-data/*.meta.json` | Fetch provenance and embedding checkpoint, digest-pinned |
| deploy step 6 | Asserts `#19220 / 0.372424` against the live service on every deploy |
| `/proof` | Serves the transcripts; structurally cannot show one not committed to the repository |

Run `task test` for the current suite result. We deliberately do not write a test count into
documentation — counts go stale silently, which is the same failure mode this document is about.

---

# Part X — Limitations, honestly

**We do not claim the model is right.** Solvent constrains authority; it does not improve judgment.
An agent using it can still be wrong about everything. The guarantee is narrower and more defensible:
being wrong leaves a durable, attributable trail, and the authorization does not outlive the belief.

**We do not claim retrieval completeness.** The headline finding is a retrieval failure. Reframing
the query found `#14139`, but that reframing was a human act, and nothing in the system guarantees
someone will think of it.

**Concurrency scope.** The kernel transcript covers single-session behaviour. The control experiment
is the concurrency evidence, and it is three specific interleavings, not a general proof.

**No measured claim about index distribution.** What is verified is a CockroachDB-native vector index
with `vector_cosine_ops`, scenario-prefixed, plan-checked to use the index with bounded prefix spans,
on a multi-region cluster. We have not measured node count, range distribution or sharding, and do
not claim it.

**Some checks are code-backed without a committed transcript.** The corpus verification target, the
deploy preflight, and the vector-plan assertions all exist and run, but we do not ship recorded output
for each. Where that is true we cite the code, not a log that does not exist.

**Cloud Managed MCP is not verified.** It is configured and its unauthenticated `401` was observed,
but end-to-end verification is pending a service-account key. It is excluded from every claimed-tool
list. A configured endpoint is not a verified tool.

**The demo is one decision.** A single belief graph, two beliefs deep, one action. It is deliberately
small enough to hold in your head. It is not evidence that the model scales to a large belief
network, and we do not present it as such.

## What we would do next

- **Retire the positional binding.** Citations are currently bound to retrieval debts by selection
  order. That is deterministic and correct, but it is a convention rather than a stored fact. A
  column recording which debt a citation discharged would make it structural.
- **Concurrency beyond three cells.** The control experiment proves the mechanism; a broader
  randomized interleaving harness would characterise it.
- **Close the Cloud Managed MCP verification** with a real service-account key.
- **Belief networks of non-trivial size,** where "which beliefs does this authorization depend on"
  stops being obvious by inspection.

---

*All values in this document are measured against the deployed service and the committed corpus. The
retrieval distances are re-asserted on every deploy. Where a claim is inferred rather than measured,
it is marked as such.*
