## Z

Plan review — this is the kernel re-opening executed as engineering, and the plan is notably better than most in this series: line-precise, scope-disciplined, with a genuinely strong test program (CS_DEBT01–09 is the right battery). Verdict: **approvable in structure, with two High findings — one process debt that was explicitly required before the kernel opens, and one safety-property migration the plan performs implicitly without noticing it performed anything at all.** Plus a redundancy catch in Step 1 that suggests the default-setting chain wasn't fully traced. I can't verify line numbers (no repo access — flagging that all content quotes are taken on trust), but the plan's internal logic is checkable.

## Against the prior round's requirements

| Requirement (kernel-fix review) | Status in Plan 11 |
|---|---|
| Decision record: doctrine citation, owner, date, alternatives | **Missing** — the plan is implementation-only; the Growth Gate record must exist *at the moment the kernel opens*, not after |
| Re-freeze at new commit, pin new hash, update Test H baseline | **Missing entirely** — the moment this lands, `7602699` is stale and every freeze reference (Phase 1.5 Test H, Phase 2 gate) points at a superseded baseline |
| Scope discipline — vocabulary only, kernel opens once | ✓ Mostly excellent — `What Does NOT Change` table is the right form; kernel touches are `kernel.go` + one `contract.go` line |
| Full suite re-run | ✓ Step 8 |
| Legacy-belief migration-or-grandfathering | Implicit (Step 1b preserves rows) — make it an explicit, stated decision |
| Coordinator mapping table dissolves | Not mentioned — see H2's flip side |

## High

**H1 — The re-freeze bookkeeping is absent, and the sequencing question is live.** The plan ends at "task test passes." It must end at: new commit → new frozen hash → decision record citing the doctrine (the original writeup §4.3: *"debt opaque vocabulary... the vocabulary belongs to the application, policy, or deployment"* — this change is doctrine-*restoring*, which is the entire justification and must be in the record) → Test H baseline and Phase 2 gate references updated. Also decide explicitly: does the Phase 1.5 battery run against the old freeze (results predate the change, Test H holds, but the battery then validates a kernel you're immediately discarding) or the new one (baseline updated first)? Either is defensible; neither is chosen.

**H2 — The plan removes a deployment-wide safety net and doesn't record that it did.** The six-item default wasn't decoration: it was a *forcing function* — no belief could reach the promotion gate without discharging provenance, contradiction, blast-radius, rollback, version-pin, and operator-signoff obligations. Post-change, any caller can create a belief with empty debt, and CS_DEBT07 *celebrates* that path. Kernel-doctrine-correct — but the obligation now lives entirely in domain layers. The plan wires the replacement for wizard (`wizardDebt`), demo, operator-review, and examples — and never mentions that **the EBP coordinator must do the same**: compiling a belief whose packet omits debt must attach the EBP starting set (`ebpDebt`), or work-agent packets arrive instantly promotable and the mechanical gate becomes a rubber stamp. This isn't in Solvent's repo, so it's out of Plan 11's scope — but it's *caused by* Plan 11, and it must be written into the coordinator spec as a mandatory translation rule this week. The doctrine says vocabulary belongs to the application; the doctrine assumes the application actually attaches it.

## Medium

**M1 — Negative-validation tests will fail by surprise.** Step 3g's find-replace handles tests referencing `kernel.FullDebt` — but tests asserting *rejection* of out-of-vocabulary items (MCP previously validated; there are almost certainly tests like "unknown debt_item → error") may not reference `FullDebt` at all and will simply fail after validation is removed. Enumerate and invert/retire them deliberately rather than letting `task test` discover them.

**M2 — The `001_schema.sql` edit is dead weight — or reveals an untraced path.** Trace the chain: fresh DB applies 001 (edited: empty) → 004 (`SET DEFAULT` six items) → 010 (empty). Net default: empty — identical with or without the 001 edit, because 004 always re-sets it. The edit only matters if some schema path applies 001 *without* 004 — and if such a path exists, it needs 010 too, not just the 001 edit. Either drop the 001 edit (cleanest: nothing historical modified, 010 suffices) or document the minimal path that justifies it. As written, Step 1a is presented as load-bearing when it isn't.

**M3 — Degenerate cases unpin.** Opaque strings still deserve defined behavior: empty-string debt item (`""` — reject at API or accept?), duplicates (`["foo","foo"]` — does retire remove one or both?), and retire-nonexistent (silent no-op or error — the old vocabulary validation incidentally guaranteed existence; typos now always possible). Three one-line decisions + tests; add CS_DEBT10–12.

**M4 — Literal sweep as verification.** Add `grep -r` for all six strings across the repo to *prove* the "does not change" table — specifically that `RetireDebt` is truly generic (the table asserts it), no service-layer logic keys on the strings, and the only remaining call sites are wizard/demo/operator/examples/mapping.go exactly as planned. One command; it converts the table's claims into evidence.

**M5 — Grandfathering made explicit.** Step 1b preserves legacy rows — state it as a decision with its consequence: old-vocabulary debts remain retireable via wizard paths (`wizardDebt` matches), scenario isolation keeps wizard and EBP beliefs disjoint so cross-vocabulary conflicts shouldn't arise. One paragraph in the decision record.

## Low

Deliverable 6's wording — "no Solvent-level code inserts or requires" — is false as written: `wizardDebt` (Step 3d) inserts those exact strings, in the Solvent repo, by design. Say "no kernel-level." MCP tool-schema changes (3c) invalidate cached client schemas — note for coordinator/agents. Rollback is trivial (git revert; 010 is harmless to re-apply) — one line for the record.

## Routed order

H1 (decision record + re-freeze bookkeeping) and H2 (coordinator `ebpDebt` obligation) before merge; M1–M3 before the test run so failures are expected, not discovered; M2's trace is a five-minute check that decides whether Step 1a survives.

The deeper observation, for the record: this plan is what doctrine-restoration looks like when done seriously — the test battery doesn't just verify the change, it *characterizes the new contract* (arbitrary accepted, empty promotable, final-truth still blocked). Land H1/H2 and it's a model kernel amendment: opened once, narrowed, documented, re-frozen. The kernel comes out smaller than it went in — which, per its own §2 doctrine, is the only direction it was ever supposed to grow.


## gemini

While decoupling the debt vocabulary cleanly isolates Solvent's kernel, shifting `debt` to completely unvalidated `TEXT[]` introduces subtle fail-open security risks, input poisoning vulnerabilities, and schema bootstrap drift.

**Adversarial Failure Vectors**

| Failure Vector | Trigger Mechanism | Systemic Impact |
| --- | --- | --- |
| **Fail-Open Default Shift** | Raw SQL `INSERT` or API call omits `debt` field. | New default `ARRAY[]::TEXT[]` creates debt-free beliefs instantly, flipping the kernel posture from fail-closed (blocked by default) to fail-open (promotable immediately). |
| **Zombie Debt Lockout** | Agent/caller submits whitespace-padded or casing-mismatched debt (e.g., `"needNullModel "` vs `"needNullModel"`). | `RetireDebt` exact-string matching fails to discharge the string, permanently trapping the belief behind an un-retireable zombie debt. |
| **Array Poisoning / DoS** | Malicious or hallucinating MCP agent pushes high-cardinality or massive string payloads. | Unbounded Postgres `TEXT[]` arrays degrade table scan performance and exhaust memory during `RetireDebt` array rewrites. |
| **Dual-Truth Schema Drift** | `001_schema.sql` is modified directly while `010_debt_opaque.sql` is appended to run after `004`. | Out-of-order schema appliers or custom test harness setups execute `001` $\rightarrow$ `004` without `010`, silently re-introducing hardcoded debt defaults. |

**Critical Architectural Vulnerabilities**

* **Inversion of Security Invariants:** Hardcoded debt previously acted as an explicit safety net: no belief could be accidentally promoted without explicit discharge. By defaulting to `ARRAY[]::TEXT[]`, any unpopulated payload or missing field in `EnterBeliefRequest` automatically passes the `promoted_is_debt_free` gate.
* **Decentralized Vocabulary Fragmentation:** Shifting `wizardDebt`, `demoDebt`, and operator CLI slices into local file packages duplicates string literals across 5+ locations. A single typo between `internal/wizard` and `cmd/operator-review` breaks workflow discharge loops silently without compile-time checks.

**Mandatory Plan 11 Safeguards**

* **Enforce API Boundary Bounds:** Add strict string length (e.g., max 64 chars per debt ID) and array size limits (e.g., max 16 items per belief) in `api/belief.go` and `cmd/solvent-mcp/tools.go`.
* **Sanitize Debt Identifiers:** Force lowercase/trimmed string normalization (`strings.TrimSpace`) at the API/MCP boundary prior to Postgres insertion to prevent zombie debt mismatches.
* **Clean Migration Pattern:** Do **not** edit `001_schema.sql` directly if `010_debt_opaque.sql` is designed as an additive migration; keep historical baseline DDL immutable to ensure migration path integrity across environment bootstrapping.
* **Mandate Non-Nil API Payload Requirements:** Require `EnterBeliefRequest` callers to explicitly provide an intentional debt array—even if empty—rather than silently defaulting missing payloads to an empty state.


## claude

This is a clean, well-motivated change — worth noting explicitly that it's closing something flagged as necessary all the way back at the start of this project's strategy work ("the six debt items are vulnerability-specific vocabulary... this is a vocabulary change, not an architecture change"), and the caller inventory in Step 3 is otherwise unusually thorough — it caught the operator-review CLI and the examples/github executor, both places this thread has previously found things missing from caller inventories. Four things worth raising before this ships.

## `internal/belief/mapping.go` is acknowledged but not actually accounted for

The "What Does NOT Change" table includes the line *"`mapping.go` (domain layer) | Keeps its own vocabulary"* — but this raises the exact question it doesn't answer. `mapping.go` is the `DebtMapping` component (the thing that decides which debt items a given piece of evidence retires — this is the same file the agentjacking demo review referenced: *"deliberately no `sentry_error` entry in `DebtMapping` — the injected evidence retires zero debt"*). That component necessarily has its own hardcoded set of debt-item strings as map keys or values, and Step 3's otherwise-exhaustive caller inventory never mentions it. Two real possibilities the plan doesn't distinguish between:

- If `mapping.go` currently imports `kernel.FullDebt` directly (e.g., to validate its own map is complete against the kernel's vocabulary), deleting `FullDebt` breaks compilation there too, and this file is simply missing from the list. `go build` will catch this mechanically, so it's not going to ship broken — but it means Step 3 isn't actually complete as written.
- If `mapping.go` has its own independent literal list of the six strings (not referencing `kernel.FullDebt`), then after this change there are now **two independently-maintained copies** of the same six-item vocabulary — the new `wizardDebt` in `seed.go`, and whatever `mapping.go` already has — where previously both could at least be checked against one shared source of truth (`kernel.FullDebt`). Nothing enforces they stay in sync going forward. A future edit to `wizardDebt` (renaming an item, adding one) with no corresponding edit to `mapping.go` would silently desync evidence-retirement logic from what's actually being created, and nothing would catch it at compile time — the exact kind of drift this whole project has repeatedly caught and fixed elsewhere.

I'd want this resolved one of two ways before implementing: either `mapping.go` is added explicitly to Step 3 with its actual relationship to `kernel.FullDebt` stated, or the six-item vocabulary gets consolidated into a single domain-layer constant that both `mapping.go` and `wizardDebt` reference, rather than two hand-synchronized copies.

## Removing the vocabulary check trades a loud failure for a silent one, and that tradeoff deserves a stated mitigation, not just an acknowledgment

This is the correct architectural call — the whole point is that Solvent shouldn't know or care about the vocabulary. But it's worth being precise about what changes operationally: today, a typo'd `debt_item` string (`"needProvenanceChck"`) is rejected immediately at the MCP boundary with a clear error. After this change, the same typo succeeds silently, and the belief becomes permanently unpromotable — not because anything failed, but because a debt item that can never match any legitimate retirement call sits in the array forever. That failure surfaces much later, as a confusing "why won't this promote" investigation rather than an immediate, actionable error. Given this project's consistent instinct to fail loud rather than let a mistake go quiet (the same reasoning behind `ReconcileIntent`'s audit-ordering fix, the exact-tuple binding work), I'd add one line to the plan: the domain layer (specifically the wizard, since it's the caller that owns `wizardDebt`) should validate its own debt list against its own vocabulary *before* calling `EnterBelief`, so a typo is still caught immediately — just at the correct layer now, not the kernel.

## Removing the MCP schema's `enum` removes agent-facing discoverability, with no stated replacement

Dropping `"enum": kernel.FullDebt` is the right structural move, but it also means an agent calling `solvent_retire_debt` no longer has any schema-level signal of what strings are valid for a given belief — it would need to separately inspect the belief's current debt array to know what it can legally retire. Worth a line in the tool description pointing at `solvent_ledger` (or whichever tool surfaces current debt) as the way to discover valid values now that the schema can't enumerate them. Small, but worth not leaving implicit.

## Minor, forward-looking: the 15-file `schemaPaths` list is a recurring cost this migration is about to pay for the second time

Step 6 requires manually adding `010` to roughly fifteen independently-maintained test-suite files, while `cmd/solvent/main.go`'s `resolveSchemaPaths` auto-discovers — the plan's own hedge ("may be sufficient, but verify") notices this asymmetry without resolving it. This isn't a defect in Plan 11 specifically — the fifteen-file update is done correctly and completely as listed — but it's the same manual-list-maintenance tax that migration `011` will pay again, and `012` after that. Not blocking, but worth flagging as a good candidate for a follow-up: point the test suites at the same auto-discovery mechanism `cmd/solvent` already uses, so the next migration doesn't need its own fifteen-file checklist.

**Net:** solid plan, ready to implement once `mapping.go`'s actual relationship to the vocabulary is confirmed and accounted for — that's the one item I'd treat as blocking rather than advisory, since it's the one place the plan's own "unchanged" claim might not be true.


## prompt

what do you propose  as replacement for the hardcoded debt vocabulary that are sane defaults instead and agnostic across all domains?


## gemini

A domain-agnostic default debt vocabulary must reflect the universal lifecycle of claim validation—origin, boundaries, consistency, verification, and human authority—rather than domain-specific deployment or mathematical mechanics.

### Proposed Universal Core Vocabulary (`ebp:*`)

| Debt Token | Epistemic Meaning | Unblock / Discharge Condition |
| --- | --- | --- |
| **`ebp:need_provenance`** | Sourcing, citations, or input artifacts are missing, off-manifest, or unverified. | Valid source anchor, verified hash, or human attribution attached. |
| **`ebp:need_scope`** | Boundary conditions, explicit assumptions, or baseline/null models are missing. | Explicit scope, assumption list, or target hypothesis class declared. |
| **`ebp:need_consistency`** | Potential contradictions, rival claims, or edge-case attacks remain unexamined. | Adversarial sweep completed with no unresolved high-severity edge conflicts. |
| **`ebp:need_verification`** | Empirical execution, symbolic derivation, or test assertions are unconfirmed. | Passing test result, reproducible code run, or verified mathematical step. |
| **`ebp:need_adjudication`** | Mandatory operator sign-off, safety review, or domain expert sign-off is pending. | Human explicit approval or promotion authorization recorded. |

---

### Implementation Guidelines for the Domain/Coordinator Layer

* **Namespace Partitioning:** Enforce a `namespace:token` convention (e.g., `ebp:need_provenance`, `devops:need_rollback_plan`, `math:need_obstruction`). This allows downstream applications to add custom domain obligations while enabling generic UIs and filters to index standard `ebp:*` flags.
* **Strict Boundary Normalization:** Apply `strings.ToLower(strings.TrimSpace(token))` and validate against `^[a-z0-9_:-]{3,64}$` at the Coordinator/API ingress. This prevents zombie debt lockouts caused by case sensitivity or trailing whitespace.
* **Opt-In Debt Presets:** Rather than implicit database defaults, expose explicit factory presets in the Coordinator layer:
* **`PresetStrict`**: `[ebp:need_provenance, ebp:need_scope, ebp:need_consistency, ebp:need_verification, ebp:need_adjudication]`
* **`PresetStandard`**: `[ebp:need_provenance, ebp:need_consistency, ebp:need_verification]`
* **`PresetLightweight`**: `[ebp:need_provenance]`
* **`PresetEmpty`**: `[]` (for pre-verified or imported state)


* **Explicit Payload Requirement:** Require clients to supply the initial debt slice explicitly on belief creation (even if passing `PresetEmpty`). Disallow missing/nil fields at the API boundary to ensure intentional posture selection and avoid fail-open defaults.


## Z

The trap to avoid first: any "universal default set" that gets installed somewhere shared just re-creates the hardcoded-vocabulary problem one level up. The doctrine (writeup §4.3, which Plan 11 restores) says vocabulary belongs to the *application* — so the right deliverable isn't a new constant, it's a **two-level structure**: a universal epistemic floor as a *specification*, and per-domain instantiations attached by each adapter at belief creation. The floor is the adequacy test; the vocabularies are the implementations.

## The universal floor

Derived from the only domain-agnostic question available: *what distinguishes a candidate claim from an established one, regardless of domain?* Six obligations survive that test:

| # | Obligation | Question it answers | Discharge evidence | Instantiated in deployment six | Instantiated in EBP six |
|---|---|---|---|---|---|
| 1 | `needProvenance` | Where did this come from? | Source record, traceable origin | needProvenanceCheck | seed/corpus citations |
| 2 | `needConsistency` | Does it contradict what we already hold? | Contradiction-sweep record | needContradictionSweep | literature check (the Reginatto/Hall precedent check) |
| 3 | `needScope` | Under what conditions does it hold? | Scope statement bound to target/snapshot | needVersionPin | needMap (assumptions, scope boundaries) |
| 4 | `needVerification` | Has anything independent of the source checked it? | Verification artifact (checker run, review, replication, hash-pinned) | *(absent from the six)* | needToyCheck, the algebraic re-derivation |
| 5 | `needImpact` | What does this affect if wrong? | Impact/dependence assessment | needBlastRadius | needObstruction (adversarial consequence) |
| 6 | `needRecovery` | If invalidated later, what then? | Recovery/retraction-dependence record | needRollbackPlan | dependence graph in research program |

Two observations fall out immediately:

**The original six had exactly one item that didn't belong in a kernel: `needOperatorSignoff`.** Your own frozen doctrine says human review is *policy*, not a universal kernel rule (§12). Its presence in the hardcoded set was the original sin; its correct home is the adapter's policy layer — wizard keeps it, deployments that want dual control add it, research domains substitute their own human gate (`needFaithfulnessReview`).

**The original six also had a gap: `needVerification` was never explicit.** Signoff ≈ approval, not independent checking. EBP's methodology had it (toy checks, independent re-derivation); deployment vocabulary implied it in tests/CI but never named the obligation. The floor fixes both errors symmetrically.

## The two-tier rule (this is where "less is more" actually lands)

Don't attach all six everywhere. The floor's obligations weigh differently by domain type:

- **Epistemic domains** (research): floor items 1–4 mandatory at creation (provenance, consistency, scope, verification); 5–6 satisfied by the domain's own method items (EBP's needObstruction/dependence structure). Starting set = the EBP six, which — per the table — already covers the floor. No change needed to the POC: **`ebpInitialDebt` = the EBP six, and the floor is the proof they're sane.**
- **Operational domains** (deployment, finance, infra): all six mandatory, because their beliefs cause external effects — impact and recovery are non-negotiable there. Starting set = the floor + policy additions (`needOperatorSignoff`). The wizard's existing six is nearly exactly this, which confirms the mapping rather than requiring changes.

The adequacy rule, formalized: *a domain vocabulary is conformant iff a mapping table shows every floor obligation is discharged by at least one domain item.* That table is the template for domain #2 — and it's a half-page document, not code.

## Discipline rules that keep the floor from re-hardcoding

1. **Attachment is an adapter act.** `ebpInitialDebt`, `wizardDebt`, future `domainXDebt` — each declared in its own adapter, attached at `EnterBelief`. No shared library becomes load-bearing; the kernel's DEFAULT stays empty per Plan 11.
2. **Every floor item names its evidence type** (the table's third column) — so defaults are mechanically dischargable, per the project's verify-at-the-boundary doctrine, not promise-dischargable.
3. **Human gates stay out of the floor.** They're gate-map rows (faithfulness, promotion, final-truth), policy per §12 — never epistemic obligations in a universal set.
4. **Optional seventh, only where non-redundancy matters:** `needNonRedundancy` — "does an established rival already explain this?" (needNullModel's abstract form). Keep it domain-side unless a second domain independently demands it; six is the comfortable floor, seven invites ceremony.

## Concrete deliverables

1. **`EPISTEMIC_FLOOR.md`** — half-page: the six obligations, discharge evidence types, conformance-mapping table. Lives next to the coordinator spec, not in any kernel repo.
2. **`ebpInitialDebt` in the coordinator** — the EBP six, with a one-line comment citing floor coverage (this also discharges H2 from the Plan 11 review: without it, work-agent packets arrive instantly promotable).
3. **Floor-coverage rows added to the coordinator's mapping documentation** — replacing the dissolved Solvent-mapping table with something with a principled basis.

The elegant summary for the decision record: the kernel shipped six deployment opinions; the floor replaces them with six domain-agnostic *questions* that every domain answers in its own vocabulary — provenance, consistency, scope, verification universally; consequence and recovery where beliefs act on the world; and judgment, always, in the layer the doctrine reserved for it.

## claude

I'd push back on the framing slightly before answering it directly, because I think "sane default strings" is a trap that recreates the exact problem Plan 11 just removed, just in vaguer language.

## Why literal defaults don't work here, even generic-sounding ones

Consider the obvious-seeming candidates: `needReview`, `needEvidence`, `needApproval`. These feel domain-agnostic because they're vague — but that vagueness is the problem, not a virtue. A debt item's entire purpose is to be a specific, checkable obligation that a specific retirement action discharges. `"needReview"` doesn't tell the wizard, the operator, or a future maintainer *what kind* of review, so two different domains using it verbatim would silently mean two different things, and neither would actually gate anything meaningful — you'd have the promotion mechanism working structurally while the checklist behind it says nothing. And if Solvent *ships* these as a default that most domains just adopt because it's there, you've recreated the CVE-vocabulary problem in softer clothing: a de facto standard vocabulary that happens to live in a README instead of a `DEFAULT` clause, but functions the same way — something every domain quietly inherits instead of actually designing. Specific enough to be useful means domain-specific. Generic enough to be domain-agnostic means too vague to be useful as literal strings. There's no point in that tradeoff where "sane default" actually lands.

## What I'd propose instead: a non-enforced taxonomy of functional roles, not a set of strings

Going back through every domain this project has actually touched — the original CVE vocabulary, DocTrust's compliance checks, the physics trust-verifier's obligations, Conductor's task lifecycle — the same six *functional roles* keep recurring, even though the literal vocabulary is completely different each time:

| Role | What it's asking | Original six-item instance | DocTrust instance | Physics-verifier instance |
|---|---|---|---|---|
| **Provenance** | Is this actually traceable to a real source, not just asserted? | `needMap` (CVE↔version mapping) | page/bbox-grounded extraction | proof step traced to a cited theorem |
| **Consistency** | Does this contradict anything else already established? | `needInvariant` | cross-document reconciliation | counterexample search |
| **Impact/scope** | What's the blast radius if this is wrong? | `needObstruction` (root cause) | downstream document impact | does this affect other results |
| **Reproducibility** | Can this be independently re-derived, not just trusted? | `needToyCheck` / `needNullModel` | independent re-verification | replication of the derivation |
| **Reversibility** | If wrong, can it be undone, and is there a plan? | `needRollbackPlan` | — (often doesn't apply) | — (rarely applies) |
| **Human authority** | Does a human with real standing need to sign off? | `needFaithfulnessReview` / `needOperatorSignoff` | human TTY review | peer/advisor review |

That's a genuinely useful observation, and it cuts the other way from what I expected going in: the original six weren't arbitrary — they were an unlabeled instance of a real, recurring pattern, just expressed in CVE-specific vocabulary that made the pattern invisible. Reversibility is the interesting exception — it shows up naturally in software and rarely in physics or compliance, which is exactly the kind of domain-specific variation that proves this needs to stay a *menu*, not a *required set*.

**The proposal, concretely:** don't encode this anywhere Solvent enforces. Write it up as a short, explicitly non-normative guidance document — something like "when designing your domain's debt vocabulary, consider whether each of these roles applies to your workflow, and give each one a name that means something in your domain" — with the table above as the illustration, showing three different domains instantiating the same six roles with completely different literal strings. That's the thing that actually earns the word "agnostic": not a shared vocabulary, but a shared *shape* that different domains fill in independently, with zero code anywhere checking that anyone followed it.

## Where this lives, and the one thing worth deciding deliberately

Plan 11's Step 7 says "do not add EBP documentation to the Solvent repository" — I'd treat this the same way, since this taxonomy, once it's illustrated with real domain examples, starts to look like domain-adjacent content even though it's meta-level. I'd put it in Conductor's docs instead, or a separate `docs/patterns/` location outside both kernels — somewhere it's clearly framed as "advice for people building on top of this stack," not something either Solvent or Conductor's own AGENTS.md needs to reference or maintain. If it goes in Solvent's repo at all, it needs the same explicit "illustrative, not enforced, not a Solvent concept" label this whole project has learned to attach to anything that could otherwise be mistaken for a real constraint.