# Solvent Production MVP Plan — Adversarial Review

**Plan under review:** `mvp_plan2.md:1` (1386 lines, phases 0-11) plus `mvp_plan.md`, `mvp.md`, `plan2.1.md` delta  
**Review posture:** Plan as belief — must earn promotion. Repository is source of truth (`AGENTS.md:52` frozen 4 tables `belief, belief_edge, evidence, action_intent`; `db/001_schema.sql:1`; `kernel/kernel.go:54`; `internal/view/view.go:1`/`explain.go`; `cmd/solvent-mcp/main.go:88`; `Taskfile.yml:224`; `formal/lean/*`; `docs/ADR/0001:5`; `docs/POST_HACKATHON_ARCHITECTURE.md:1`). Where plan and repository disagree, repository wins. Findings are pre-implementation, no code/SQL/Lean/MCP modified.

---

## Review posture

We dogfood Solvent:

```
Evidence → Belief → Debt → Verified Discharge → Promotion → Authority → Action → Reassessment
```

The plan is the belief. Strategy documents are not evidence. Prior ChatGPT approvals are not evidence. Only repository behavior, measured invariants, and testable contracts count as evidence. Debt is every unresolved assumption that would let the plan grant itself authority it has not earned. Promotion is `APPROVE`; `APPROVE WITH CONDITIONS` is “promoted with open debt that must be discharged before consequential action”; `REJECT` is “would violate `promoted_is_debt_free` of the plan itself.”

This review treats incoming evidence as hostile until proven attributable — same as the product must treat evidence ingestion.

---

## Executive verdict

**APPROVE WITH CONDITIONS**

The plan's core thesis is correct and its sequencing — *harden kernel → then prove with DocTrust → then generalize* (`mvp_plan2.md:1312`) — is the right order. The four-verb slice (`internal/view/explain.go`, `cmd/solvent-mcp/tools.go:solvent_explain`, `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh`) already makes the existing `Evidence→Belief→Debt→Promotion→ActionIntent` legible without changing the kernel that proves it.

But the plan would grant itself **production authority** for a `Principal+Resource+Scope+Consequence` model it has not yet made impossible to bypass, for an evidence-ingestion path it has not yet hardened, and for a central-verification availability contract it has not yet made testable. Each is a predictable production failure mode, not a polish issue. All three are fixable before code, which is why this is not `REJECT`.

**Conditions are in `Production blockers` and `Can we start implementation?`. Violating them collapses to `REJECT`.**

---

## Findings

### F-01 — CRITICAL — Consequence binding gap: executor can perform B after Solvent authorized A

- **Severity:** CRITICAL — Could make the authority model fundamentally unsafe. Invalidates product thesis.
- **Finding:** `mvp_plan2.md:304-346` introduces the correct principle `Action+Resource+Scope → Consequence → Risk → Authorization`, but the current enforcement is `action_intent(action TEXT)` (`db/001_schema.sql:64`) plus application-supplied `action` string (`kernel/kernel.go:107` `IntentOnPromoted`). Nothing binds the executor's *actual* resource/consequence to the authorized tuple. The plan defers *how* `resource` and `consequence` become authoritative (sec 5 says “do not necessarily add as immediate columns; first define semantics” `mvp_plan2.md:312`). Until binding is schema-enforced or contract-enforced at the service boundary, authorization is advisory prose on top of an unenforced vocabulary.
- **Evidence:** `db/001_schema.sql:60-76` — only `belief_id, belief_status, action, state` on `action_intent`. `kernel/kernel.go:107` — `IntentOnPromoted(ctx, scenarioID, beliefID, action string)` takes `action` as opaque string. `docs/ADR/0001:29-31` gate checks only `(belief_id, belief_status)`, not resource/scope. `internal/view/view.go:32` `Intent{belief_id, action, state}` — no resource.
- **Why it matters:** `docs/ADR/0001:448` trust boundary is `Agent→MCP/REST→Solvent→Identity/Policy→Executor`. If the executor receives `update_record` and decides on its own what record that means, Solvent's `ALLOW` for `temporary_cache` can be applied to `patient_record_123`. The failure `Solvent authorizes A, executor performs B` (`adv_plan_prompt.md:361`) is not closed.
- **Attack scenario:** Compromised or buggy DocTrust calls `POST /authorize {action:"export", resource:"report.pdf"}` and receives `ALLOW`. Executor then exports `"/etc/passwd"` or a different tenant's document using the same bearer warrant, because `resource` was never part of the `gate` and no post-execution proof is required. The audit will show `action=export` as `ALLOW` while the consequential resource was different.
- **Recommended resolution:** Before production, make `Principal+Resource+Scope+Consequence` an **explicit service contract** that the executor must echo and Solvent must verify. Minimal: add `principal, resource, scope, consequence` as columns on `action_intent` (additive migration, no invariant change) and extend the `gate` service check to require `resource+consequence` equality at authorization + optional post-execution attestation (`actual_resource == authorized_resource`). Until then, document that current `action` is *not* the production authorization target (already partially done `mvp_plan2.md:352`) and refuse to call the MVP “resource-aware.”
- **Required before implementation?** YES — for any integration that claims resource-scoped authorization. The four-verb slice can ship without it only if it does not claim to authorize per-resource consequences.

### F-02 — CRITICAL — Evidence ingestion is authorization-critical while still conceptual

- **Severity:** CRITICAL — A poisoned source manufactures justification for authority; product thesis collapses to “trusted evidence is trusted.”
- **Finding:** `mvp_plan2.md:143-155` makes “minimal generic ingestion contract” MVP-essential and `mvp_plan2.md:707-728` lists evidence ingestion as TCB, yet Phases 0-2 describe no ingestion hardening and defer concrete provenance/replay/poisoning controls to Phase 6. The current ingestion is `internal/derive/testdata/etcd_real` fixtures + `pipeline.Run` (`internal/pipeline/pipeline.go:288`) with no provenance verification, no replay protection, no source-compromise handling. The plan claims `Evidence has provenance and can participate in durable reassessment` as an MVP success criterion (`mvp_plan2.md:1188`) while current `evidence.provenance_class` (`db/001_schema.sql:51`) is an unenforced enum and `content_sha256` is caller-supplied.
- **Evidence:** `internal/pipeline/pipeline.go:59-121` `ProcessEvidence` trusts `sourceTypeMap` filename mapping; `internal/belief/belief.go` `RetireDebt` binding not shown but ingestion does not verify attestation; `docs/ADR/0001:222-240` deferred evidence gateway as `OUT OF SCOPE` for hackathon, correctly. `mvp_plan2.md:1188` vs reality — contradiction.
- **Why it matters:** Every downstream guarantee (`Debt → Promotion → Warrant`) amplifies evidence trust. If evidence can be forged, debt discharge is manufactured, promotion is earned dishonestly, and `gate` then dutifully authorizes the wrong consequence. This is `evidence poisoning` (`adv_plan_prompt.md:404`) and the Solvent-self attack surface.
- **Attack scenario:** Attacker with write to a future webhook (or a malicious connector) posts a forged `release_v3528.json` with correct `source_url` but attacker-controlled payload claiming `needOperatorSignoff` is satisfied. Pipeline treats it as `reproducible_artifact`, retires debt, belief promotes, DocTrust exports `approve_claim` as `ALLOW`. No attestation, no provenance check. The attack is indistinguishable from legitimate evidence at the ledger.
- **Recommended resolution:** Make Phase 3 (attestation) and Phase 4 (reassessment) depend on a *provenance-hardened ingestion* subphase: every `evidence` row must carry `content_sha256` verified by the server (not client), `provenance_class` enforced at ingress, `source_observed_at` and `ingested_at` ordering, and replay protection (content-hash deduplication + source sequence). The `EvidenceFeed` abstraction (`AGENTS.md:56`) must be the enforcement point, not documentation. Block production M-E (evidence) success criterion until this subphase has tests (malformed, forged, replay, duplicate, conflicting).
- **Required before implementation?** YES — before any external evidence source is marked MVP-essential. The four-verb slice can proceed if ingestion remains fixture-only and the MVP success criterion is restated as “ingestion hardening is a Phase 6 blocker, not Phase 0-2 complete.”

### F-03 — HIGH — MCP/REST parity risk: two adapters become two authorities

- **Severity:** HIGH — Production deployment should not proceed with diverging authority paths; attacker chooses weaker interface.
- **Finding:** `mvp_plan2.md:121-141` declares REST MVP-essential and mandates `MCP ─┐ ├──→ Solvent service → kernel` with identical semantics, but no service layer exists today (`cmd/solvent-mcp/tools.go:1` handlers call `kernel` directly; no `internal/service` package). The plan does not allocate work to extract the service until Phase 5, while REST and MCP would be built in Phases 2-5. Without a single extracted authority service, each adapter will reimplement validation, identity, error mapping (`23503`/`23514` vs HTTP), retry (`crdb.ExecuteTx`), and audit.
- **Evidence:** `cmd/solvent-mcp/tools.go:19` `handleSolventLedger` vs inexistence of `internal/service` or `cmd/solvent-api`. `Taskfile.yml:224` tests only MCP boundary. `docs/FOUR_VERB.md` maps `Connect→MCP` and claims `REST` future, but no parity test exists.
- **Why it matters:** `Can an attacker choose the weaker interface?` (`adv_plan_prompt.md:390`) is a security boundary. Divergent error semantics cause clients to misinterpret `DENY` vs `REVIEW` vs `unavailable`.
- **Attack scenario:** REST implements `POST /authorize {principal, action}` with permissive `action` normalization (lowercasing), while MCP's `solvent_authorize_action` requires exact `belief_id` + `action` string. Attacker crafts `Action: Update_Record` via REST that normalizes to `update_record` and bypasses a `needOperatorSignoff` check that MCP's `solvent_explain` would have flagged as blocked. Both return `ALLOW` under different rules; audit shows conflicting decisions for the same belief.
- **Recommended resolution:** Introduce `internal/service` (or `internal/authority`) in Phase 0-1 *before* any REST work: one function `Authorize(ctx, principal, resource, scope, consequence, beliefID) (decision, reason, sqlstate)` that both adapters call. Add a protocol test: same scenario+belief, MCP and REST produce identical `ALLOW/DENY/REVIEW` and identical `audit_live_on_nonpromoted` for fixtures. Block REST MVP until parity test is green.
- **Required before implementation?** YES — if REST is MVP-essential (see F-09). If REST is deferred to post-MVP, this drops to MEDIUM.

### F-04 — HIGH — Debt-free is necessary, not sufficient: debt as checkbox theater

- **Severity:** HIGH — Production would ship “compliance with debt=empty” while promotion remains under-justified.
- **Finding:** `mvp_plan2.md:367` promotes on `promoted_is_debt_free` only (`db/001_schema.sql:30`). Debt items are retired by any `solvent_retire_debt` call for a recognized enum item (`cmd/solvent-mcp/tools.go:129`), with no proof that the underlying claim was verified. Same `content_sha256` evidence could be reused to retire `needProvenanceCheck` and `needContradictionSweep` (violates `AGENTS.md:74` `One citation per retrieval debt` in spirit), one actor can create and discharge its own debt, and a customer can define nonsensical obligations that make `debt=[]` meaningless. The plan correctly notes debt could become a disguised policy engine (`adv_plan_prompt.md:254`) but does not bound it.
- **Evidence:** `kernel/kernel.go:83` `RetireDebt` is `array_remove` idempotent, no provenance check. `internal/belief/belief.go:Mapping` retired items are caller-supplied. `docs/ADR/0001:29` debt check is cardinality only. `mvp_plan2.md:381-402` assumes “each impossible state is rejected,” but `live intent + unpromoted belief` is the only debt-adjacent impossible state that is DB-enforced.
- **Why it matters:** `Is debt-free actually sufficient for promotion?` (`adv_plan_prompt.md:262`) is the product's honesty test. Customers will optimize for `debt=[]` fastest, not most truthfully.
- **Attack scenario:** Agent splits `needOperatorSignoff` (intended: human review of clinical safety) into `needOperatorSignoffPart1…Part6`, each trivially retired by a single synthetic `operator_asserted` evidence row with attacker-controlled `source_url`. Belief promotes in minutes, `ALLOW`, and audit shows “all obligations discharged” while justification is vacuous.
- **Recommended resolution:** Before production, enrich debt semantics without enlarging kernel: (a) enforce one-citation-per-debt at the service layer (reuse `AGENTS.md:74` logic already for retrieval), (b) require `provenance_class` binding per debt item (e.g., `needOperatorSignoff` only via `operator_asserted` with future attestation, not `external_feed`), (c) add `FullDebt`-per-tenancy validation that rejects empty or tautological debt sets. Keep enforcement at service layer first, promote to CHECK only after evidence.
- **Required before implementation?** YES — before DocTrust integration; otherwise DocTrust can define debt that is formally empty but substantively vacuous, proving compatibility without proving safety.

### F-05 — HIGH — Human attestation lifecycle still unbounded PKI after ADR correctly bounds it

- **Severity:** HIGH — Would ship a “better than agent-mediated approval” story that reintroduces the PKI it claims to avoid.
- **Finding:** `docs/ADR/0001:187-228` correctly bounds PKI to the attestation boundary and leaves mechanism open (`application-managed vs enterprise identity vs WebAuthn`). `mvp_plan2.md:474-555` re-derives the same lifecycle checklist (enrollment, binding, storage, rotation, revocation, departure, compromise, replay, auditability) but Phase 3 exit criterion (`human approval is independently attributable and integrity-protected, agent cannot forge` `mvp_plan2.md:555`) would pass with a naive `Ed25519` prototype that has no revocation or departure handling. The plan defers expert review to “before implementation” (`mvp_plan2.md:1323`) but does not gate Phase 5-6 on its outcome.
- **Evidence:** `mvp_plan2.md:548-550` lists candidates without selection criteria. No definition of “intended” vs “attested”, no replay `attestation_id` uniqueness, no `issued_at` clock skew, no two-person-approval consideration (`adv_plan_prompt.md:289`).
- **Why it matters:** Key-management is the same class as warrant PKI (`docs/ADR/0001:193` “same CLASS, smaller blast radius”). An unrevoked human attestor who leaves the company retains indefinite authority to discharge `needOperatorSignoff` for high-impact consequences.
- **Attack scenario:** Departing operator’s `application-managed` private key is not revoked; attacker replays a captured `attestation_id` for `needOperatorSignoff` on a new `belief_id` with same `debt_item` string but different claim (debt item reuse across debt vocabularies). Service accepts because binding check is `debt_item` equality, not `(belief_id, debt_item, attested_by, channel)`. Belief promotes, `ALLOW`.
- **Recommended resolution:** Gate Phase 6 (DocTrust validation) and Phase 5 exit on a written attestation lifecycle spec that includes: `attestation_id` uniqueness and idempotency, replay cache, `issued_at` window, revocation propagation (<1min), departure deactivation, WebAuthn vs enterprise identity selection with threat model, and a test for cross-belief reuse. Keep prototype fixture-only until spec approved by practitioner (as `mvp_plan2.md:1323` intends, but make it a hard gate).
- **Required before implementation?** YES — before any DocTrust attestation retires real debt. The four-verb slice should continue to use current `solvent_retire_debt` and explicitly label it non-attested.

### F-06 — HIGH — Belief as authority object underspecified: one-to-many and many-to-one

- **Severity:** HIGH — Kernel abstraction may be structurally insufficient for resource-scoped consequences.
- **Finding:** `mvp_plan2.md:36-54` evolves `Evidence→Belief→Debt→Promotion→Authority` toward `Principal+Resource+Scope+Consequence→Warrant`, but leaves `Belief` as the sole justifying object. The 14 attacks in `adv_plan_prompt.md:203-238` are largely unanswered: can one belief authorize multiple consequences with different risk? Can one consequence depend on multiple beliefs with mixed evidence? What if evidence applies only to one resource but belief is broader? What prevents authority broadening beyond justification? Current `belief.claim TEXT` (`db/001_schema.sql:9`) is unbounded and `belief_edge.kind` (`contradicts/derives`) has no semantics for partial support.
- **Evidence:** `mvp_plan2.md:50` “authorization unit is not merely an action… is a consequence against a specific resource under a specific principal and scope” — principle stated, no enforcement. `kernel/kernel.go:54` `EnterBelief` stores `claim` as opaque text. `db/001_schema.sql:47` `evidence.belief_id` is one belief per row, but one evidence row can be cited for multiple debt items via repeated `RetireDebt` calls.
- **Why it matters:** If `Belief` is too coarse, `gate` authorizes too much. If too fine, customers create belief-per-consequence and debt becomes combinatorial.
- **Attack scenario:** Belief “service X v1.2 is safe to deploy to prod” (evidence: staging canary logs) is used to authorize `deploy to prod` for `payment-service/prod` (HIGH_IMPACT) and `deploy to prod` for `logging-sidecar/prod` (LOW_RISK) under the same promotion. The latter is appropriate; the former extrapolates staging evidence to payment safety. The plan has no `scope` constraint to prevent this broadening.
- **Recommended resolution:** In Phase 1 (authority abstraction), make the mapping explicit and additive: keep `Belief` as epistemic claim, but introduce `AuthorityTarget{principal, resource, scope, consequence}` as a separate row linked to one or more `(belief_id, attestation)` pairs with at least `ONCE` debt discharge per target. Reject one-belief-authorizes-many-consequences implicitly; require explicit linking. Backport to docs as a non-schema conceptual model first, migrate to columns only after DocTrust exercises it.
- **Required before implementation?** YES — before DocTrust claims generality. DocTrust should exercise the many-to-one case (e.g., `needProvenanceCheck` from doc hash + `needOperatorSignoff` from human on the same resource) to force the kernel to express it.

### F-07 — HIGH — Central verification availability contract untested

- **Severity:** HIGH — Production would block or fail-open without a testable contract.
- **Finding:** `mvp_plan2.md:730-740` correctly chooses `Solvent unavailable → HIGH_IMPACT = DENY/REVIEW`, but leaves cache/replay/partial-failure semantics to Phase 10. The four-verb demo and `mcp_verify.sh` have no outage test. Current `solvent_explain` predicts `gate`/`promoted_is_debt_free` failures but does not distinguish `DENY` (authoritative) from `authority unavailable` (fail-closed). A cached `ALLOW` that outlives `RetractCascade` is a fail-open bug (`mvp_plan2.md:765` warns, but no test exists).
- **Evidence:** `docs/FOUR_VERB.md` demo never simulates `solvent-crdb` down. `internal/view/explain.go:predicted_*` fields exist, but no `unavailable` state. `Taskfile.yml:224` `task test` does not inject `FABLE_DSN` failure.
- **Why it matters:** Solvent becomes a critical production dependency (`adv_plan_prompt.md:331`). Without explicit “deny vs unavailable” distinction, executors will retry or fall back to cached `ALLOW`. Without bounded cache, a 2-hour Solvent outage is a 2-hour production freeze for HIGH_IMPACT, with no documented degradation path.
- **Attack scenario:** Network partition isolates executor from Solvent for 45s during deployment. Executor's HTTP client times out; retry logic treats `unavailable` as `DENY` and aborts, but a second executor's in-memory cache still holds `ALLOW` from 10m ago and proceeds to `HIGH_IMPACT_EXECUTE` after the belief was retracted via `solvent_falsify`. The stale cache authenticates an invalidated belief.
- **Recommended resolution:** Add outage and cache tests before MVP hardening: (a) `solvent_explain` must return `unavailable` vs `denied` distinctly; (b) any executor cache must be bounded and expiry-bound to `belief.validity_horizon` / warrant `expires_at` (even while temporal authority is deferred, the API contract must include `expires_at` and executors must respect it); (c) add a chaos test: `docker stop solvent-crdb` → `solvent_authorize_action` must be `DENY/REVIEW`, not `ALLOW`, and `solvent_explain` must not return stale live intents.
- **Required before implementation?** YES — for availability hardening (Phase 10) before production customer; not before four-verb slice.

### F-08 — MEDIUM — Reassessment in-flight race: stale authority executes before invalidation catches up

- **Severity:** MEDIUM — Important, can be sequenced after core MVP if explicitly documented as debt.
- **Finding:** `mvp_plan2.md:561-605` describes validity horizon, expiry, lazy expiration, periodic reassessment, but current `RetractCascade` (`kernel/kernel.go:127`) is *explicit* (operator calls `solvent_falsify`). There is no push, pull schedule, or execution-time revalidation. The plan's invalidation paths (contradiction→resolution obligation→retain/review/retract) require operator action, so a `live` intent can execute after contradictory evidence arrives but before `solvent_falsify` is invoked. The “M3-style guarantee `correct application logic + weak schema → AUDIT=1`” still holds, but the temporal gap is not the same race — it is staleness, not concurrency.
- **Evidence:** `internal/pipeline/pipeline.go:89` contradiction leaves `Contradiction=true` with `BeliefID=""` and logs `slog.Warn … no ledger mutation` — no automatic debt. `docs/POST_HACKATHON_ARCHITECTURE.md:2.4` wall-clock gap is documented, but not implemented. `mvp_plan2.md:608` “Do not rely on wall clock alone to mutate state” is stated without alternative for immediate invalidation.
- **Why it matters:** `Can stale authority execute before reassessment catches up?` (`adv_plan_prompt.md:454`) is a production SLO. Long-running agentic workflows amplify it.
- **Attack scenario:** New evidence `contradicting_evidence.json` arrives, pipeline emits `Contradiction=true`, operator is asleep, agent's `solvent_explain` still says `can_authorize=true` (belief still `promoted`), and `HIGH_IMPACT` executes. Hours later operator retracts; audit shows the window.
- **Recommended resolution:** For MVP, make reassessment *synchronous at authorization*: require executors to call `solvent_explain` or `solvent_authorize_action` (which re-checks `gate`) immediately before `HIGH_IMPACT_EXECUTE`, not rely on periodic cache. Document this as the MVP contract and add a test where a `promoted` belief with known `Contradiction=true` still `ALLOW`s until retracted — making the gap explicit rather than surprising. Defer background reassessment to after MVP but keep the test.
- **Required before implementation?** NO — but must be documented as open debt and reflected in the authorization API contract before production onboarding.

### F-09 — MEDIUM — DocTrust as Reference Implementation #1 proves compatibility, not generality

- **Severity:** MEDIUM — Important to prevent kernel contamination.
- **Finding:** `mvp_plan2.md:175-225` makes DocTrust the first independent validator with the right constraint “no shared kernel/schema/DB, consume via MCP/REST.” However DocTrust exercises only document-centric `EvidenceFeed` with deterministic `needProvenanceCheck`-like debt. It does not exercise risk-tiered consequences (`strategy.md:21`), multi-belief authorization (`F-06`), or tenant isolation. The plan lists `SOLVENT_ENGINEERING_GUIDE.md` and `docs/ADR/0001` as evidence that the kernel is domain-agnostic, but the only domain exercised is the one the kernel was built for (etcd issues).
- **Evidence:** `mvp_plan2.md:230` “No DocTrust-specific concepts should be added to kernel” — correct discipline, but no test that violation would be caught. `internal/derive/testdata/etcd_real` is the only large corpus tested.
- **Why it matters:** `Does DocTrust prove independence or merely prove compatibility with one friendly application?` (`adv_plan_prompt.md:480`). A DocTrust-shaped kernel will look general while being etcd-shaped.
- **Attack scenario:** DocTrust workflow requires atomic `create belief + evidence + retire debt → authorize → reassess` across multiple documents with conflicting evidence. Kernel's `belief_edge` traversal (`kernel/kernel.go:66` `WITH RECURSIVE`) and scenario scoping are sufficient, but DocTrust actually needs batch debt discharge (one attestation discharges one debt item per spec — `docs/ADR/0001:381`), which the current `solvent_retire_debt` does not enforce. DocTrust builds its own batch helper and the kernel check never catches the over-discharge.
- **Recommended resolution:** Before claiming `MVP success: DocTrust works entirely through public interfaces` (`mvp_plan2.md:1208`), add a second *synthetic* consumer in tests (e.g., `internal/pipeline` pipeline test with a second `SourceKeVEntry` from a different domain) that exercises a non-document resource type and multi-belief consequence. Keep it as a test fixture, not a product integration, to validate generality without building a connector catalog.
- **Required before implementation?** NO — for MVP phase sequencing, but YES before calling the kernel “domain-agnostic” in customer materials.

### F-10 — MEDIUM — REST as MVP-essential is unproven; delays do not buy evidence

- **Severity:** MEDIUM — Can be sequenced after core MVP; keeping it as MVP-essential adds parity risk (F-03) without evidence.
- **Finding:** `mvp_plan2.md:121-141` claims REST is MVP-essential because applications/SaaS/workflows will not speak MCP. No evidence is cited (customer interview, DocTrust requirement, A2A dependency). The existing production path is MCP-only (`docs/FOUR_VERB.md` is MCP-only per review decision 3). Making REST MVP-essential before DocTrust has exercised REST forces parallel MCP/REST implementation under the parity risk above.
- **Evidence:** `adv_plan_prompt.md:498` explicitly asks “Is REST truly necessary before DocTrust?” The plan does not answer with evidence. `mvp_plan2.md:1312` sequence places `Stabilize REST alongside MCP` before DocTrust, contradicting `plan2_background.md:5` “simultaneously sanity-check attestation” but hardening-first principle.
- **Why it matters:** Scope creep in the guise of completeness. REST is not free: auth, rate limiting, tenant header, error mapping, and audit streaming all become MVP debt.
- **Attack scenario:** Team builds REST to satisfy MVP checklist, but DocTrust's first real consumer is Claude via MCP, which needs `solvent_explain` fidelity, not REST. REST ships with looser `action` normalization (lowercasing) and becomes the weaker interface (F-03) that an attacker will actually use.
- **Recommended resolution:** Downgrade REST to “MVP-conditional”: build the internal service layer (`F-03`) first, ship four-verb slice via MCP only, and promote REST to MVP only when DocTrust (or first lighthouse) demonstrates a non-MCP consumer that cannot be satisfied by MCP+webhook. Keep the phase, but gate it on evidence (customer/DocTrust contract).
- **Required before implementation?** NO — decision can be deferred, but the phase order should reflect conditionality to avoid premature divergence.

### F-11 — MEDIUM — Migration of `action` → `Principal+Resource+Scope+Consequence` invalidates or over-authorizes old rows

- **Severity:** MEDIUM — Backfill safety; can be sequenced but must be designed before schema change.
- **Finding:** `mvp_plan2.md:547-552` asks `Can old action intents be interpreted under the new model?` but provides no migration strategy. Current rows have `action TEXT` only (`db/001_schema.sql:64`). Adding `principal/resource/scope/consequence` as `NOT NULL` would fail on backfill; adding as nullable risks old rows being interpreted as wildcard `resource=*` and accidentally over-authorized.
- **Evidence:** No migration file exists for `resource`/`consequence`. `internal/testdb` harness always resets to the 4-table schema; no test exercises mixed old/new semantics.
- **Why it matters:** Customers already on the current kernel (even fixture-only) would have their `live` intents re-evaluated under the new model. A stale `ALLOW` from an old `action="update_record"` could suddenly authorize a broader consequence.
- **Attack scenario:** Migration adds `resource TEXT` with `DEFAULT ''`. Old intent `action=update_record, resource=''` is interpreted as “any resource” by the new service layer that coalesces empty to wildcard. An attacker replays the old warrant reference and gets `ALLOW` for a different resource.
- **Recommended resolution:** Design migration now, implement later: new columns `DEFAULT NULL`, service layer treats `NULL` as “unknown, must be re-authorized” (fail-closed), and old warrants are considered expired at migration time. Add a test `TestMigration_OldActionIsNotOverAuthorized` as debt even before migration.
- **Required before implementation?** NO — before any `resource`/`consequence` column is added, i.e., before Phase 1 leaves design.

### F-12 — MEDIUM — Lean model proves state-machine preservation, not SQL refinement

- **Severity:** MEDIUM — Formal verification could claim more than it proves.
- **Finding:** `formal/lean/*` is correctly described as abstract state-machine preservation (`README.md:195`), but `mvp_plan2.md:421-436` proposes to “evolve in lockstep” and extend Lean when kernel semantics expand to `principal/resource/consequence`. The refinement gap (Lean → Go/SQL) is not characterized: `ON UPDATE CASCADE` re-evaluation of `live_requires_promoted` (`README.md:134`) is the critical DB behavior that Lean does not model via SQL semantics, and `crdb.ExecuteTx` retry is not in the Lean abstraction.
- **Evidence:** `formal/lean` has no `Sql` model; `docs/POST_HACKATHON_ARCHITECTURE.md:3.5` refinement is `RESEARCH`. The plan treats Lean as confidence, not as a refinement proof, which is honest, but the phase ordering could let Lean diverge from the new service-layer checks (which will be application logic, not CHECKs).
- **Why it matters:** A theorem `live_intent_implies_promoted` in Lean could remain green while a new application-layer `principal` check is bypassable in Go.
- **Attack scenario:** New `resource` authorization is implemented as pre-check in `internal/service` before `IntentOnPromoted`. Lean proves the new state machine, but the actual `INSERT` path via `cmd/solvent-mcp` still accepts the old `action`-only payload, bypassing the new check. Lean and DB both still pass.
- **Recommended resolution:** Keep formal scope minimal: prove only DB-enforced transitions in Lean; for application-layer checks (principal/resource), add executable SQL-level CHECKs where possible (or at least DB-level `CHECK`/`FK` that the Go code cannot forget), and make `internal/service` the single gate that both MCP and REST call. Do not extend Lean until the DB constraint exists to reflect the theorem.
- **Required before implementation?** NO — document the gap as debt; do not inflate theorem count for symmetry.

### F-13 — LOW — Observability/audit immutability underspecified

- **Severity:** LOW — Polish, but test the audit you claim.
- **Finding:** `mvp_plan2.md:1144-1169` requires explainability (`Why denied? Who attested? What changed?`) and “immutable enough” audit, while `refusal_log` (`db/003_wizard.sql:47`) and `evidence.content_sha256` are the only audit artifacts today. `solvent_explain` derives `human_summary` on the fly (`internal/view/explain.go:buildBeliefSummary`) — it is not persisted, so “what changed” is not queryable after the fact without an external log. The plan postpones audit streaming to Phase 10 but lists audit as an MVP success criterion (`mvp_plan2.md:1164`).
- **Evidence:** No `audit_log` table; `internal/view/explain.go` does not write. `Taskfile.yml:224` tests do not assert audit immutability.
- **Why it matters:** Enterprises buy the audit, not just the gate. If `human_summary` is not persisted, post-incident forensics will reconstruct rather than replay.
- **Attack scenario:** Operator is paged for `deploy` that Solvent denied, but the only record is `refusal_log` with `statement=promote` and no `principal/resource/consequence` context. Investigation cannot answer “who asked, with what warrant, what was the consequence?”
- **Recommended resolution:** Defer immutable audit stream, but for MVP make `solvent_explain` output explicitly ephemeral (document that it is a projection, not a log) and persist at least `refusal_log` + `action_intent` transitions with `scenario_id, belief_id, principal, resource, consequence, sqlstate`. Add a test that `solvent_explain` never writes.
- **Required before implementation?** NO.

---

## Evidence ledger

| Claim | Evidence | Confidence | Open debt | Status |
|---|---|---|---|---|
| REST API is MVP-essential | `mvp_plan2.md:121` declares, no customer/DocTrust citation | Low | Customer evidence, DocTrust contract | Open |
| DocTrust is the right first reference implementation | `mvp_plan2.md:175` rationale (independent app, domain-agnostic) — convincing but untested beyond etcd | Medium | Second synthetic domain to prove generality (F-09) | Open |
| Central verification is preferable | `docs/ADR/0001:99` revocation/reassessment controllable, avoids PKI platform; `adv_plan_prompt.md:301` attack considered | High | Outage/cache test (F-07) | Discharged (with debt) |
| High-impact actions should fail closed | `docs/ADR/0001:113` fail-closed + risk tiers; `mvp_plan2.md:730` repeats | High | Precise risk taxonomy remains policy, not kernel | Discharged |
| Attestation should be independently attributable | `docs/ADR/0001:187` key-management boundary, `mvp_plan2.md:474` attestation → verified discharge | High | Lifecycle spec: enrollment/revocation/replay (F-05) | Open |
| Principal/Resource/Scope/Consequence are the right future dimensions | `mvp_plan2.md:304` principle correct, but `action_intent.action TEXT` (`db/001_schema.sql:64`) shows insufficient binding | Medium | Formal definition + executor echo + migration (F-01, F-11) | Open |
| MCP is the primary agent-facing interface | `cmd/solvent-mcp/main.go:88` 7 tools over stdio, `Taskfile.yml:224` MCP boundary green, `internal/view/explain.go` MCP-only Ask | High | None | Discharged |
| Evidence/event ingress is MVP-essential | `mvp_plan2.md:143` generic ingress as MVP-essential | Low | Provenance/replay/poisoning hardening (F-02) | Open |
| A2A can safely be deferred | `mvp_plan2.md:157` not MVP-essential, `docs/POST_HACKATHON_ARCHITECTURE.md:3.4` research | High | None — correctly deferred | Discharged |
| Current kernel can evolve without semantic rewrite | `db/001_schema.sql:1` additive columns pattern in `004_debt_vocabulary.sql`, `kernel/kernel.go:54` domain-agnostic `ClaimType` | Medium | Migration for resource/consequence (F-11) and many-to-one belief mapping (F-06) | Open |
| `solvent_explain` is a safe, non-authoritative projection | `internal/view/explain.go:predicted_*` vs real `23503`/`23514`, `internal/view` no `Exec`, `scripts/mcp_verify.sh` read-only determinism | High | Persisted audit gap (F-13) | Discharged |
| `promoted_is_debt_free / gate / live_requires_promoted` remain sufficient for MVP | `kernel/kernel_test.go:249` B-09, B-11, `internal/view/explain_test.go` Tests A-C, `proof/isolation.log` M3 | High | Extended to cover resource binding (F-01) | Discharged |
| Four-verb slice is the right start | `docs/FOUR_VERB.md`, `scripts/demo/four_verb.sh` 10 steps green, `Taskfile.yml:224` test harness | High | None | Discharged |

---

## Architectural invariants

### Current proven invariants

- `I-1 promoted_is_debt_free` — `CHECK (status<>'promoted' OR debt='{}' AND NOT final_truth)` `23514` — proven by `kernel/kernel_test.go:249` B-09/B-10 and live explain tests, `lake build` `promote_preserves_validity`
- `I-3 gate` — `FOREIGN KEY (belief_id, belief_status) REFERENCES belief(id,status) ON UPDATE CASCADE` `23503` — B-11, `solvent_authorize_action`
- `I-4 live_requires_promoted` + `ON UPDATE CASCADE` re-evaluation — `kernel/kernel_test.go:474` B-12, `RetractCascade` cancel-before-retract `kernel/kernel.go:127`
- `I-5 AuditLiveOnNonPromoted == 0` — `kernel/kernel_test.go:701` B-08/B-13, `internal/view/explain.go` global summary
- `I-7 crdb.ExecuteTx` — `scripts/check_i7.sh` 7 sites, MCP boundary no raw Exec
- `I-8 RetractCascade atomicity` — `kernel/kernel_test.go:637` B-24/B-16
- `I-6 no embedding column` — proven vacuously

### Future required invariants

- `I-9 resource/consequence binding` — `ALLOW` implies `actual_resource==authorized_resource` and `actual_consequence==authorized_consequence` (see F-01)
- `I-10 attestation binding` — `attestation_id` unique, `belief_id+debt_item+attested_by+channel` integrity, replay cache, revocation `<1min` (F-05)
- `I-11 evidence provenance` — `evidence.content_sha256` server-verified, `provenance_class` enforced at ingress, replay protection (F-02)
- `I-12 availability` — `HIGH_IMPACT` denied when unavailable, `ALLOW` not cached past `expires_at`/`validity_horizon` (F-07)
- `I-13 reassessment freshness` — authorization revalidated at `HIGH_IMPACT` execution time, not just at `IntentOnPromoted` (F-08)

### Currently unproven assumptions

- `debt-free ⇒ justified` — debt discharge proves obligation review, not claim truth (`F-04` checkbox theater)
- `one belief ↔ one consequence` — mapping cardinality undefined (`F-06`)
- `MCP ≡ REST` semantic equivalence — no service layer or parity test yet (`F-03`)
- `Lean ≡ SQL` refinement for new dimensions — gap noted, not closed (`F-12`)
- `audit immutability` — no persisted warrant/audit stream beyond `refusal_log` (`F-13`)

---

## Kernel verdict

**Is the current kernel production-worthy?** YES for its current `Evidence→Belief→Debt→Promotion→ActionIntent` scope, NO as a resource-scoped authority substrate without the additive `AuthorityTarget` layer.

- **What must change:** Add `AuthorityTarget{principal, resource, scope, consequence}` as an explicit service-level concept with a prospective `action_intent` column migration (`M NULL, fail-closed on NULL`), plus per-debt `provenance_class` binding and one-citation-per-debt service check. Keep `Belief` as epistemic claim, not authorization target (`F-06`, `F-04`).
- **What must NOT change:** 4 frozen tables' existence, `promoted_is_debt_free`/`gate`/`live_requires_promoted`/`RetractCascade` ordering, `FullDebt` vocabulary mechanism, `crdb.ExecuteTx` discipline, domain-agnostic `EvidenceFeed`. The current kernel is the foundation, not a prototype to replace.
- **What should be reserved as future semantics:** Warrant as portable JSON (not a table until consequence binding exists), temporal `validity_horizon`/`expires_at`, `evidence_contradiction` relation, formal `principal/resource` in Lean — all correctly deferred in `docs/POST_HACKATHON_ARCHITECTURE.md`.

---

## Interface verdict

- **MCP:** MVP-essential — YES. Why: primary agent surface, already 7 tools, read-only Ask via `solvent_explain` proven. Main risk: none at kernel layer; risk is overloading it with policy. Readiness: **ready**.
- **REST/JSON API:** MVP-essential — CONDITIONAL (see F-10). Why: credible for DocTrust if DocTrust cannot speak MCP, but no customer evidence yet and parity risk high. Main risk: weaker interface (F-03) + premature `resource` normalization. Readiness: **not ready** (no service layer, no parity test). Recommendation: gate on DocTrust contract; build service layer first regardless.
- **Evidence/Event ingress:** MVP-essential — YES, but as hardened minimal contract. Why: how reality reaches ledger. Main risk: poisoned evidence manufactures authority (F-02). Readiness: **not ready** for external sources (fixture-only today); needs provenance/replay hardening before any external webhook.
- **A2A:** MVP-essential — NO. Why: depends on stable `AuthorityTarget` + Warrant verification (F-01, F-07). Main risk: building A2A on coarse `action` string produces a “warrant layer” that cannot be verified per-resource. Readiness: **defer correctly**.

---

## DocTrust verdict

- **What DocTrust proves:** That an independent application can consume Solvent through public MCP/REST without importing kernel/schema/DB, and that `Belief→Debt→Promotion→Gate` is usable outside etcd. This validates the process claim `producing a company product, not a hackathon continuation` (`mvp_plan2.md:8`).
- **What DocTrust does NOT prove:** Domain-agnostic generality (single document domain, deterministic `needProvenanceCheck`-like debt), resource-scoped authority (no per-resource consequence), tenant isolation, or evidence poisoning resilience. See F-09.
- **What integration risks it exposes:** One-citation-per-debt violation via batch retire, debt vocabulary trivialization (F-04), kernel contamination if DocTrust needs batch/compound beliefs (`F-06`), and bypass temptation (writing `belief` directly instead of via `EvidenceFeed`) — the plan's constraint `must not import Solvent internals` (`mvp_plan2.md:885`) needs a test, not just a rule.
- **Whether it should remain Reference Implementation #1:** YES — keep DocTrust as #1, but add a lightweight synthetic second domain fixture (non-document resource type) as a test-only generality check before claiming the kernel is domain-agnocentric. This is the cheapest way to force the `AuthorityTarget` abstraction to be exercised.

---

## Security verdict

- **Trust boundaries (current):** `Agent→MCP→Solvent service→DB` is sound for the four-verb slice; `External evidence → Solvent` is the critical boundary and is currently **unhardened** (fixture-only). `Human→MCP Elicitation→Solvent` is correctly bounded per `docs/ADR/0001:187` (Elicitation ≠ signature), but the bounded PKI lifecycle is still open (F-05). `Solvent→Executor` is unbounded (F-01) — executor not required to prove it executed the authorized resource.
- **Attack surfaces:** `evidence.content_sha256` and `source_url` are client-supplied (`pipeline.go:59`), `provenance_class` is enum only, `claim` is unbounded `TEXT`. No rate limit, no auth on `solvent-retire_debt` beyond enum check (`tools.go:129`). `FABLE_DSN` is unauthenticated `root@localhost` in dev, but cloud path is `ccloud` (not yet exercised for MVP).
- **Fail-open paths:** `HIGH_IMPACT` fail-closed is stated (`mvp_plan2.md:735`) but any cached `ALLOW` (executor memory) that outlives `RetractCascade` is fail-open. No `expires_at` enforcement at executor (F-07). `READ/DRAFT` MAY continue per policy introduces a low-risk bypass if policy maps a high-impact `action` to `LOW_RISK` via misconfiguration.
- **Replay risks:** `attestation_id` uniqueness and replay cache do not exist (deferred); `evidence` deduplication by `content_sha256` is not enforced at ingress, so same evidence can discharge multiple debt items across beliefs if service layer does not check one-citation-per-debt (F-04). `RetractCascade` is atomic, but a retracted belief's old warrant JSON can be replayed to an executor that does not re-verify with Solvent.
- **Stale authority paths:** Between `contradiction` detection (`pipeline.go:90` `Contradiction=true`) and `solvent_falsify`, `can_authorize` remains `true` (F-08). Between `RetractCascade` and executor cache expiry, stale `ALLOW` remains usable (F-07).
- **Identity weaknesses:** No `principal` column on `action_intent` today; any `solvent_authorize_action` call is effectively anonymous. Future `principal` will be consumed from OIDC/OAuth but no contract exists for binding `principal` to `attested_by` for the same transaction.
- **Evidence poisoning paths:** Poisoned `release_v3528.json` → `needProvenanceCheck` retired → `Belief.promoted` → `gate` allows `HIGH_IMPACT` (F-02). Source compromise of a future webhook is Solvent compromise.
- **Solvent-self attack surface:** `solvent-mcp` runs with `FABLE_DSN` as `root` (no least privilege), `FixtureRoot` is filesystem path. A malicious fixture file with a crafted `action` string (long, control characters) is stored verbatim in `action_intent.action` and surfaced in `solvent_explain`. No input-length or charset validation exists beyond `kernel` type checks.

---

## Reliability / availability verdict

- **Solvent outage:** `solvent-mcp` pings DB at startup (`main.go:76`); if `solvent-crdb` is down, MCP fails to start (fail-closed is implicit). No health check, no multi-region strategy, no redundancy (`mvp_plan2.md:757` lists but does not implement). A 30s outage is tolerable for four-verb demo; a 2-hour outage would freeze all `HIGH_IMPACT` deploy/authorize flows with no degradation path.
- **Database outage:** CockroachDB single-node Docker (`solvent-crdb`) is not cloud HA. `40001` is correctly retried via `crdb.ExecuteTx` (`kernel/kernel_test.go:327` B-18), but no test for `database degraded` returning stale reads. `READ COMMITTED` is the deployed isolation for `gate` (`README.md:149`); `SERIALIZABLE` retry path is tested, but production region/partition is not.
- **Network partition:** Executor↔Solvent partition is indistinguishable from `DENY` at the executor without explicit `unavailable` vs `denied` distinction. `solvent_explain` already distinguishes `predicted` (`predicted_*`) from observed `23503`/`23514`, but `solvent_authorize_action` returns `isError true` for both `gate` refusal and DB unreachable — the executor cannot differentiate without inspecting `sqlstate` vs `connection error`. This must be a typed error.
- **Stale cache:** No Solvent-managed cache today (good), but any DocTrust or executor in-memory cache is unbounded. The plan's “bounded caching only where safe” (`mvp_plan2.md:763`) has no bound. Until temporal `expires_at` exists, the bound must be “no Executor cache for HIGH_IMPACT.”
- **Partial failure:** `RetractCascade` is correctly single-transaction cancel-then-retract (`kernel/kernel.go:129`), and `B-24/B-16` atomicity tests prove rollback on foreign-scenario intents. However `promoted belief → live intent → falsify → re-promote` racing with a new `solvent_authorize_action` is not tested under concurrency (the M3 `proof/isolation.log` tests a different race: `IntentOnPromoted` vs `RetractCascade`). The reassessment race (F-08) is the production variant: `Authorize` racing `RetractCascade` must result in `AUDIT=0`, which M3 does show, but not under the new `principal/resource` dimension.
- **Concurrent state changes:** `EnsureBelief` is transactional dedup (`kernel/kernel.go:169`), but `Belief.Process`'s `evidenceExists` check (`internal/belief/belief.go:64`) has a TOCTOU window (noted `wave3_qa.md`). This is correctly called “accepted for MVP” but must remain documented as open debt for high-concurrency ingestion.

---

## Production blockers

Must be solved before production customers (strict):

1. **Consequence binding** — F-01: add `AuthorityTarget` contract and make executor prove `actual_resource/consequence == authorized_*` (hard gate)
2. **Evidence provenance hardening** — F-02: server-verified `content_sha256`, `provenance_class` enforcement at ingress, replay/one-citation-per-debt check
3. **MCP/REST single service** — F-03: extract `internal/service` and add parity test; gate REST on evidence
4. **Attestation lifecycle spec** — F-05: enrollment/binding/storage/rotation/revocation/departure/compromise/replay/auditability + practitioner sign-off
5. **Availability contract** — F-07: typed `DENY` vs `unavailable`, no HIGH_IMPACT cache until `expires_at` exists, outage/partition chaos test
6. **Debt sufficiency** — F-04: one-citation-per-debt enforcement and per-debt `provenance_class` binding so `debt=[]` is not checkbox theater (can be same service layer as 2)

---

## Can we start implementation?

**YES, after conditions:**

- **Condition A (before any `principal/resource` code):** Write the Phase 1 authority-target spec (what `Resource`, `Scope`, `Consequence` are, and the invariant `I-9 AuthorityTarget binding`) and the migration rule for old `action TEXT` rows (fail-closed `NULL`). This is a docs-only 2-page spec, not a migration.
- **Condition B (before DocTrust):** Gate REST on evidence — implement `internal/service` first, keep DocTrust on MCP until a non-MCP consumer is evidenced (see F-10). This prevents two authorities.
- **Condition C (before external evidence):** Keep evidence ingestion fixture-only and restate MVP success criteria (see F-02) — do not mark M-E success until provenance hardening has tests.

If these three docs-only conditions are met, the four-verb slice and Phase 0-2 hardening can proceed immediately. Otherwise `NO` — starting `resource` columns or REST without the contract will cement the weaker interface.

---

## Revised phase ordering

Do NOT rewrite the entire plan. Minimum changes:

```
Current:  0 Semantic contract → 1 Authority abstraction → 2 Kernel hardening
          → 3 Attestation → 4 Temporal → 5 Warrant+REST → 6 Prod sec/avail
          → 7 SaaS → 8 DocTrust → 9 Lighthouse → 10 A2A → 11 Capability

Revised:  0 Semantic contract (+ Condition A spec: Resource/Scope/Consequence + migration)
          → 1 Authority abstraction (+ AuthorityTarget, debt-provenance binding sketch)
          → 2 Kernel hardening (keep DB enforcement preference, add debt one-citation check)
          → 2.5 Service extraction (NEW: internal/service + MCP/REST parity test) — gates REST
          → 3 Attestation (keep, but hard-gate exit on Condition C lifecycle spec + practitioner review)
          → 4 Temporal (keep, but clarify execution-time revalidation as MVP contract)
          → 5 Warrant+REST (MCP remains primary; REST promotes only when DocTrust shows non-MCP need — F-10)
          → 6 Prod sec/avail (+ F-07 availability contract before external customer)
          → 7 SaaS (unchanged)
          → 8 DocTrust (add synthetic second-domain fixture as generality check — F-09)
          → 9 Lighthouse (unchanged)
          → 10 A2A (unchanged, still post-MVP)
```

Change is three insertions: service extraction, spec gates, and a second-domain test fixture — no phase removal.

---

## Dogfood principle

> Does Solvent's own development plan obey the same philosophy that Solvent is supposed to enforce?

- **Does the plan distinguish evidence from belief?** PARTLY — `mvp_plan2.md:231` promises to document current transitions, and `AGENTS.md:52` freeze is respected, but the plan cites no customer or DocTrust evidence for `REST is MVP-essential` and `generic ingestion is MVP-essential` — those are beliefs without evidence (F-10).
- **Does it record unresolved debt?** MOSTLY — debt is recorded (e.g., `docs/ADR/0001:547` attestation key lifecycle), but F-02/F-04/F-06 debts are not yet in the plan's own debt list — the plan's debt list should include `resource binding`, `one-citation-per-debt`, and `evidence provenance` as explicit open items.
- **Does it prevent premature promotion?** YES — the plan correctly freezes the kernel (`AGENTS.md:52`) and gates promotion (`promoted_is_debt_free`). This review is the promotion gate for the plan itself.
- **Does it identify who grants authority?** NO — the plan itself has no attestor or principal; it was not approved by a hostile CTO/security architect. This review fills that debt by requiring `APPROVE WITH CONDITIONS` before `ALLOW` to build `Resource`/`Attestation`.
- **Can authority be revoked?** YES — `APPROVE WITH CONDITIONS` is revokable; blockers 1-6 revoke MVP production authority until discharged. The four-verb slice remains `REVIEW` (ship with warning), not `DENY`.
- **Can new evidence invalidate the plan?** YES — a DocTrust integration failure or a second-domain fixture failure would invalidate the `domain-agnostic` claim and force a kernel abstraction change (F-06, F-09) — the plan acknowledges this in Phase 8 `Use failures to harden again` (`mvp_plan2.md:1135`).
- **Is the plan itself being reassessed?** YES — this review is the reassessment. The violation is that the plan would otherwise treat `Principal+Resource+Scope+Consequence` as authoritative before `Condition A` is discharged (F-01) and treat `evidence → authority` as safe before `Condition C` is discharged (F-02).

Where violated, this review records exact remediation (blockers + revised ordering). Fixing them is `verified discharge`; shipping without them would be `agent interprets answer → RetireDebt` — the exact weak path the product exists to eliminate.

---

## Final rule

The architecture is:

- **correct** — for current `Evidence→Belief→Debt→Promotion→ActionIntent` with `gate`; incorrect as a resource-scoped authority substrate without `AuthorityTarget` (F-01)
- **enforceable** — where DB-enforced (`promoted_is_debt_free`, `gate`, `live_requires_promoted` + `RetractCascade`) yes; where still application `action TEXT` string, no — must become service/CHECK-enforceable
- **testable** — for current invariants (kernel tests + `internal/view/explain_test.go` + `mcp_verify.sh` 7 tools + `lake build` green); not yet for availability/cache, reassessment race, MCP/REST parity, or resource binding
- **operable** — as a single-node Docker demo; not yet as a production dependency (no health/multi-region/observability/rate limiting)
- **interoperable** — via MCP today; REST via same semantics is not yet proven
- **commercially viable** — only with the six blockers discharged; otherwise it is a sophisticated guardrail that authorizes the wrong resource

*A finding that forces us to change the kernel now is more valuable than another hundred features later.* This review provides six such findings (F-01 through F-06) that are cheaper to fix as a 2-page spec and a service extraction than as a migration after DocTrust and lighthouse are built.

---

*Evidence: this review cites `mvp_plan2.md` (phases 0-11), `docs/ADR/0001:5`, `docs/POST_HACKATHON_ARCHITECTURE.md:1`, `AGENTS.md:52`, `db/001_schema.sql:60`, `kernel/kernel.go:83/107/127`, `internal/view/explain.go:predicted_*`, `cmd/solvent-mcp/tools.go:129`, `Taskfile.yml:224`, `formal/lean/*`, `internal/view/explain_test.go`, `scripts/demo/four_verb.sh` live run, `scripts/check_i7.sh`, and `adv_plan_prompt.md:619`. No code/SQL/Lean/MCP was modified. No ADR-0002 was created. Repository wins over plan.*

