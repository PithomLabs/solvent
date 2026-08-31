## Goal

Perform the hostile implementation-gate review of `docs/ARCHITECTURE/schema-design.md` (1,181 lines, read in full) and produce exactly one new file: `docs/REVIEWS/SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW.md` (status: `REVIEW ARTIFACT — NOT IMPLEMENTED`). No DDL, no modifications to the schema design or any Go/SQL/Lean/MCP/REST/ADR/architecture file, no commit, no sync.

## Inputs status

Already in hand from earlier exploration (verified against the actual checkout):
- Repository truth: `db/001–004` (all tables/constraints verbatim), `kernel/` (8 verbs, `crdb.ExecuteTx`, SQLSTATE classification), `internal/belief|pipeline|view|wizard|corpus`, `cmd/solvent-mcp` (7 tools, hardcoded track1/track2 scenarios, no principal anywhere), `formal/lean` (3 invariants, 11 theorems).
- Corpus: `schema-readiness-decision.md` (D1–D16, F1–F7, gate constraints), both authority specs (N1–N4, I-A/I-P labels, executor contract), `SCHEMA_DESIGN_ADVERSARIAL_REVIEW_FINAL.md` (F-5.2-*, C-1..C-7 exit tests), `ADR/0001`, `POST_HACKATHON_ARCHITECTURE.md`, `FOUR_VERB.md`.
- The artifact under attack: `schema-design.md` — read line-by-line above.

Still to read during execution (one pass, for verification only):
- `docs/ARCHITECTURE/kernel-enforcement-boundary-and-trust-model.md` (listed as authoritative input but not yet read).
- `.kilo/plans/SCHEMA_DESIGN_ADVERSARIAL_REVIEW.md` (the prompt's preceding-review pointer; note `..._FINAL.md` lives in `docs/REVIEWS/` — already read).

## Review execution

Work attack groups 1–28 against the actual relational model, with findings written in the mandated `SDAR-N — SEVERITY — Title` format (Claim attacked / Evidence / Attack / Exact failure boundary / Why schema succeeds or fails / Required resolution / DDL-blocking / Exit test). Attack direct SQL and the intended API, never accept "the service checks it" where a DB relationship could close the path, and keep the TCB threat model honest (compromised service = kernel compromise per D2).

Key attack leads already identified from the close read (each will be developed or refuted with evidence):

1. **CRITICAL candidate — mutable `snapshot_id` pointer on an active target.** `authority_target` needs UPDATE privilege (for `proposed→active`, `→revoked`), so nothing structural stops `UPDATE authority_target SET snapshot_id=... WHERE state='active'` — silent authority mutation, violating the design's own locked principle 10. The REVOKE UPDATE on `target_snapshot` defends the wrong object.
2. **CRITICAL/HIGH candidate — `proposed` state is unrepresentable.** `authority_target` carries no candidate tuple anywhere, and `target_snapshot` is immutable from creation — so CreateTarget(tuple) has no home. The four-operation model (Create/Attach/Request/Approve) cannot be expressed as specified.
3. **HIGH candidate — C-2 not actually resolved to a DB mechanism.** The justification→epoch binding is only "conceptually bound"; a composite FK `(belief_id, promotion_epoch)` needs a UNIQUE target on `belief` — which means altering the frozen table the design elsewhere promises not to touch (tension with the C-6 decision). No epoch backfill semantics for existing promoted beliefs.
4. **HIGH candidate — discharge `status='retracted'` implies an UPDATE path**, letting an accepted instrument be revoked then re-accepted (replay through revocation), contradicting D9's counter-entry rule.
5. **HIGH candidate — transitional dual truth defanges C-4.** Promotion is gated by `array_length(debt,1)=0` on unversioned strings; the discharge ledger's `policy_version_id` binding is recorded but consumed by nothing, so "a v1 discharge cannot satisfy a v2 obligation" is not achieved in the transitional model.
6. **Missing/undeclared FKs:** `target_snapshot` declares no FKs at all (target, principal, policy composite targets), `belief_tenant` declares no FK to `belief(id)`, `principal` PK is global `principal_id` with no `UNIQUE(tenant_id, principal_id)` to support the composite tenant FKs the design relies on.
7. **Retry-changes-the-approved-set race (Approve × Attach):** serializable retry re-reads justifications, so a link committed mid-retry can enter the approval snapshot without the human approver having seen it — attacks the "exact set the approver approved" guarantee.
8. Second-source-of-truth items: `justification_set` JSONB blob unverifiable against live rows by the DB; `approver_principal_type` copy reintroduces what the prior review explicitly removed; `UNIQUE(tenant_id, revision)` vs the C-5 exit test's "PK enforces total order".
9. Smaller items: requester identity absent (separation of duties unrecordable), no NOT-EMPTY CHECKs on opaque fields, `issued_at <= revoked_at` boundary and DB-vs-service enforcement of C-7, `belief_tenant` insert authority as confused-deputy vector, migration epoch backfill, empty-string/empty-JSON wildcard probes, DocTrust testability gaps.

## Deliverable structure

Exactly as mandated: Status / Review Scope / Repository Truth / Executive Verdict (APPROVE | APPROVE WITH CONDITIONS | REJECT/REDESIGN) / Critical / High / Medium-Low findings / C-1..C-7 verdicts / per-object verdicts (Principal, AuthorityTarget, target_snapshot, Justification, Debt-Discharge, Policy Version, Tenant, Evidence) / Concurrency / Migration / Service TCB / Executor Boundary / DocTrust / Formal-Lean verdicts / Security Property Matrix (16+ rows) / Impossible-State Verdict / Self-Attack Matrix / Second-Source-of-Truth Audit / DDL Readiness Gate (SAFE or NOT SAFE) / Required Conditions Before DDL / Dogfood Evidence Ledger / Final Dogfood Test / Validation (git status + exact files changed).

Special rules honored: no invented invariant IDs, no table-count forcing, attribution ≠ authorization, hash ≠ version semantics, byte equality ≠ semantic equivalence, precise residual risk over fake guarantees.

## Execution steps

1. Read `kernel-enforcement-boundary-and-trust-model.md` + `.kilo/plans/SCHEMA_DESIGN_ADVERSARIAL_REVIEW.md` (verification pass only).
2. Spot-verify any schema-design CURRENT claims I cite against the actual files (already largely done).
3. Write `docs/REVIEWS/SCHEMA_DESIGN_IMPLEMENTATION_GATE_REVIEW.md` in one artifact, constructing concrete interleavings for the concurrency groups and the full final attack chain (legitimate principal → promoted belief → target → approval → ALLOW, then strengthening each control to find where authority is still manufacturable).
4. Run `git status --short`; confirm only the one new file; report. No commit, no sync.