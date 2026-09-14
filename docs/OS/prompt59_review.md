## Z

Disposition first, then the attack. The consolidation landed most of last round's findings: the envelope now carries `packet_id/task_ref/project_ref/scenario_ref/corpus_ref/scope` (H1 ✓), evidence anchors via mandatory `content_sha256` (H2 ✓, stronger than requested — the kernel enforces it), and §13.2 acknowledges reference resolution (C1 half-landed — the two-form reference *schema* still isn't specified). **H3 and H4 — the decision record and pre-registered pilot criteria — are missed for the third consecutive round**, and this round makes them worse, because the architecture flipped again (research state: coordinator-held → Solvent-held) and a frozen component is about to be opened. Findings, with your exception treated first.

## The debt-vocabulary fix — the exception you flagged

Before touching the kernel, answer one question: **where does the vocabulary actually live?** Three possibilities with completely different governance:

| Location | Consequence |
|---|---|
| Kernel Go code validates/mints the six items | Kernel defect → Growth Gate → fix |
| Service-layer defaults stamped at belief creation | Service change; kernel untouched; record, no gate |
| DB CHECK constraint | Additive migration; schema-discipline gate |

And here is the finding that changes the character of this fix entirely: **the kernel's own documented doctrine already authorizes it.** The original Solvent writeup, §4.3: *"The crucial design decision was to make debt opaque vocabulary, rather than encode one deployment's vocabulary inside the kernel. The kernel only needs the structural rule: a belief can be promoted when its debt is empty. The vocabulary belongs to the application, policy, or deployment."* If the six items are hardcoded in kernel code, the kernel **deviates from its own frozen design principle** — and your fix is not an expansion but a *restoration*. That is the strongest Growth Gate case available: the change reduces kernel content, aligns implementation with documented doctrine, and the TEXT[] schema already supports it (the kernel treats debt as opaque; promotion requires the empty set; nothing else needs to know the strings).

Process requirements, none waivable by the elegance of the justification: decision record with the doctrine citation; full kernel test suite re-run (regression discipline from the original freeze); **re-freeze at a new commit and pin the new hash** — Phase 1.5's Test H baseline (`7602699`) and the Phase 2 gate references are now stale the moment this lands; and scope discipline — the kernel is open once, touch *only* the vocabulary, nothing adjacent. Two follow-ons: legacy beliefs carrying the old six items need migration-or-grandfathering, and once the vocabulary is deployment-owned, the coordinator writes `needMap` directly — **§6's mapping table dissolves** except as legacy-data handling. One more subtraction your fix buys for free.

## Critical

**C1 — Two ledgers, multi-destination compile, and no transactional or reconciliation story.** The previous design had one store; partial failure was impossible. Now one packet compiles to Solvent (beliefs, evidence, debt, edges) *and* Conductor (tasks, dependencies, activity), and the failure modes are real: Solvent committed + Conductor failed → epistemic objects with no operational projection (invisible work); Conductor committed + Solvent failed → tasks whose `governance_ref` dangles at nonexistent beliefs. Required in the coordinator spec: (a) **ordering — Solvent first, Conductor second**; projection is retryable, canonical state is not, and the reverse order corrupts; (b) **idempotency — `packet_id` as the idempotency key**, content-hash dedup for evidence (kernel-enforced), claim-hash or mapping-based dedup for beliefs, so a retried packet doesn't double-create; (c) **reconciliation — partial failure records an activity event and enters a retry state, never half-compiles silently**; (d) **mapping durability** — "the Coordinator retains only transient packet data" is wrong as stated: the local_id→canonical_id mapping must survive coordinator restarts, which makes it durable state. Keep the no-third-database principle honest by persisting the mapping *in Solvent's snapshot JSONB* (source packet + local_id on each created object). This is one afternoon of specification and it's the difference between a compiler and a corruption source.

## High

**H2 — FALSIFIER-as-contradicts-edge is a representational loss, not a subtraction.** A conflict edge requires an existing contradicting object. A falsifier is *conditional and future* — "if toy model shows coefficient mismatch, Fisher rigidity fails" — there is nothing yet to contradict with. Forcing it into `contradicts` either fabricates a belief to serve as the edge's source or drops the conditional structure. The fix inside your constraints: falsifier conditions persist to the target belief's **snapshot JSONB** (the kernel already accepts rich snapshots) and surface in the projection/UI; the adversarial packet still carries and validates them. Relocated, not deleted — but the plan should say so, because "far more schema-faithful" is true for CONFLICT and not for FALSIFIER.

**H3 — UNKNOWN with no target has nowhere to go.** Debt attaches to a belief; unknowns like "does IST produce linear amplitude?" target nothing — they're open questions. Representing them as debt on an unrelated belief distorts; as "candidate belief with insufficient support" changes a question into a claim (and Solvent's claim_type taxonomy — postulated/accommodated/derived — has no question marker). Rule needed: *targeted unknown → debt on target; untargeted unknown → candidate belief whose claim is the question, status postulated, human triages*. Lossy but honest; specify it.

**H4 — §10 silently gives the Coordinator an execution role, and the document that said "coordinator is dumb about consequence" now has it applying authorized effects.** This is defensible — someone must apply the retraction after authorization, and it beats a human hand-driving the kernel — but it must be *named and fenced*: (a) the coordinator holds an execution capability for epistemic effects, gated by Solvent intent/claim, via the normal authorize→claim→apply sequence; (b) **kernel-API-only access — the coordinator never writes Solvent tables directly**; the DB-enforced intent gates are the enforcement, and raw SQL is the writeup's own bypass vector #4 — the one component holding the execution capability is exactly the one that must not have it; (c) this changes §12's "dumb about consequence" and the §20 ownership table — both need rows, in the decision record.

**H5 — The decision record and pre-registered pilot criteria: missed for the third round, now load-bearing.** The freeze language gets stronger each round ("I would lock this now," "strongest formulation") while the record doesn't exist. This round's supersessions alone require it: research-state ownership (coordinator → Solvent), Multi-Agent Protocol v0.4 superseded by two roles, Reasoning Agent deferred indefinitely, v0.4 retirement gates deferred with it, the unified data model superseded by the two-ledger split, plus the kernel re-freeze. And Phase 7 still has zero success criteria — the project's oldest recurring finding, now attached to the run that ends the specification era. Write both before Phase 1, or the pilot's PASS is unfalsifiable the same way Phase 1's was.

## Medium

The three-way state distinction (`source_claimed_state / agent_assessment / system_state`) — last round's "hard protocol invariant" — has no home in the revised packet; it's representable (source claim → snapshot JSONB; agent assessment → evidence with `provenance_class: agent_analysis`; system state → kernel `status`) but the invariant must be restated with those mappings or it dies in consolidation. `scenario_id` is being repurposed from security isolation to research-domain scope — fine, say so. CONFLICT-as-`contradicts` overstates scope-difference tensions (two beliefs can clash without contradicting) — accept the lossiness explicitly or route to candidate-belief. Duplicate-belief handling: exact-hash detection is mechanical and allowed; near-duplicate flagging for humans; the merge mechanics from last round (§16) still need their retargeting rule. Packet reference forms (C1 residue): local refs valid intra-packet, canonical refs for existing state, validation resolves each — one paragraph in the packet spec.

## Verdict

This is the strongest architectural version of the entire series — the two-ledger decision is correct, the coordinator-as-compiler is now genuinely thin, the debt-vocabulary fix is doctrine-restoring rather than doctrine-violating, and the §12 trigger condition (task cancellation ≠ belief retraction) is exactly the right line. The blocking set is small and concrete: the transactional spec (C1), the coordinator-executor naming (H4), the kernel re-freeze protocol, and the two perpetually-missing artifacts (H5). None reopen architecture. Land those, and the specification era doesn't just end — it ends with every boundary enforced by a kernel that was designed for exactly this, which is the quiet vindication of the whole seven-month review record.


## Gemini

Flushing the Coordinator-owned research database and leveraging Solvent and Conductor directly simplifies the architecture, but compressing EBP objects into frozen database schemas introduces critical operational vulnerabilities.

| Vulnerability Vector | Underlying Mechanism | Failure Mode |
| --- | --- | --- |
| **Lossy Falsifier Compression** | Downcasting `FALSIFIER` objects to simple `contradicts` edges in `belief_edge`. | Destroys test metadata (`test_condition`, failure bounds), forcing operational logic into unindexed JSON blobs. |
| **Dual-Ledger Atomicity Risk** | Writing epistemic state to Solvent and workflow state to Conductor without 2PC. | Ingestion failure mid-packet leaves orphaned beliefs in Solvent with no tracking tasks in Conductor. |
| **Query Isolation in Conductor** | Storing EBP mappings inside opaque `governance_ref` strings. | Disables native SQL joins; finding all tasks blocked by open debts requires fetching full task trees into application memory. |
| **Cascading Retraction Flares** | Triggering Solvent's `RetractCascade` via automated packet ingestion. | A single flawed adversarial packet invalidating a core node can wipe out downstream research branches instantly. |

**1. Lossy Compression of Testable Falsifiers**
A falsifier is not merely a directional contradiction edge; it is an active test contract containing explicit execution conditions, target metrics, and failure thresholds. Flattening a `FALSIFIER` into a `belief_edge(kind = contradicts)` strips out actionable operational semantics. Unless test payloads are buried inside unindexed JSON fields, the system loses the ability to automatically re-evaluate or execute targeted falsification runs.

**2. Distributed State Desynchronization**
Dual-ledger architectures suffer from classic dual-write failure modes. Packet ingestion requires non-atomic writes across two independent storage systems: Solvent for epistemic truth and Conductor for operational tasks. If network or DB failures strike after Solvent accepts a belief write but before Conductor creates the associated work task, the system enters a corrupted zombie state that requires manual intervention to reconcile.

**3. Query Blindspots via Opaque References**
Treating Conductor's `governance_ref` as an unindexed JSON payload keeps the Conductor kernel frozen, but pushes computational complexity into the Go Coordinator. To answer simple operational questions—such as *"Which active tasks belong to beliefs held up by open debts?"*—you can no longer execute a single SQL query. The Coordinator must pull entire task graphs, parse stringified payloads, and join them in application memory.

**4. Retraction Cascade Over-Triggering**
Solvent's `RetractCascade` is a destructive database-level operation that recursively cancels live intents and retracts downstream beliefs. If packet processing automatically translates an agent's challenge into an authorized `action_intent`, a single overzealous adversarial packet can accidentally purge legitimate research branches before a human can intervene.

To make this architecture production-ready, enforce strict two-phase commit patterns during packet ingestion, require explicit human confirmation before firing Solvent retraction cascades, and ensure test execution metadata isn't stripped during edge normalization.