# Evaluation: `plan.md` Against `prompt2.md` — Solvent Commercial MVP

**Verdict: APPROVED WITH REWORK.** One item is not a nit — as written it reopens the exact class of vulnerability the last several review rounds closed in the authority kernel, one layer up, in the product shell that sits in front of it. Everything else here is genuine nits: fixable in the same pass, none of them blocking.

The plan is, on the whole, a faithful and disciplined translation of `prompt2.md`. It carries the kernel-growth-gate default-no forward correctly, it sequences phases in the same order as Category 26 almost verbatim, it reproduces the nine questions and the full acceptance-criteria checklist without softening them, and — notably — Scenario D (Confused Deputy) is written to actually exercise the exact five-tuple target-binding mechanism the v0 authority work was built for, which is exactly what I flagged as *missing* from the earlier agentjacking demo. That continuity is worth naming: this plan is responsive to work done earlier in this project, not just to the prompt in front of it.

---

## CRITICAL — The workflow token carries `Authority`, and `Refresh` is untyped

**Where:** §7, `service/workflow/token.go`

```go
type WorkflowToken struct {
    ...
    Authority  *AuthorityRef  `json:"authority,omitempty"`
    ...
}

func (s *TokenService) Refresh(oldToken string, updates map[string]interface{}) (string, error)
```

`prompt2.md` Category 6 and Category 14 say this in nearly identical language twice, deliberately: *"Never trust a browser object, stale workflow state, or agent-provided 'approved=true' field as execution authority. Any workflow token, session state, or UI state is workflow continuity, NOT the authority source"* and *"token != authority. Every consequential operation must re-read/revalidate current authoritative state."*

As specified, the token doesn't just risk becoming the authority source — it's built to carry one. `AuthorityRef{TargetID, SnapshotID, ApprovedAt}` embedded directly in an HMAC-sealed struct is an attractive nuisance for exactly the failure this prompt was written to prevent: HMAC sealing proves the token hasn't been *tampered with since signing*; it proves nothing about whether the authority it carries is still *current*. A validly-signed token from ten minutes ago can carry a `SnapshotID` for a target that's since been revoked, and nothing about the seal detects that. The field's presence invites exactly the shortcut Category 6 names by name — a future maintainer (or a time-pressured present one) reading `token.Authority` and treating a green field as a green light, because re-querying the kernel is more code than reading a struct field that's already sitting there, signed and trustworthy-looking.

`Refresh(oldToken string, updates map[string]interface{})` compounds this. An untyped map means any caller can request an update to *any* field — including `Stage`, which governs what the workflow state machine believes is permitted next (§5's `CanTransition`). A signed token is only as trustworthy as the code that decides what goes into it before signing; an API that accepts arbitrary field overwrites keyed by string doesn't constrain that at all. This isn't a forgery risk (the HMAC still holds against a third party), it's a service-trusts-itself-too-much risk: the seal protects the token from the outside, not the token's content from the service's own permissive update path.

The good news: `PrepareForAction` (§8) doesn't take a token as an argument. It re-reads authority from `*kernel.Store` directly by `scenarioID`/`beliefID`. That's the correct gate, and it's already built correctly. Which means `Authority` on the token isn't load-bearing for anything the plan currently describes — it's dead weight that happens to be shaped like a loaded gun.

**Required fix, not optional:**
1. Remove `Authority *AuthorityRef` from `WorkflowToken` entirely. `PrepareForAction` doesn't need it and nothing else in this plan reads it. If something ends up needing to *display* the authority that was checked, that's a read from the kernel at render time, not a field carried on a signed continuity token.
2. Replace `Refresh(oldToken string, updates map[string]interface{})` with named, typed methods for the specific transitions that are actually legitimate — `AdvanceStage(token, newStage)`, `AttachEvidence(token, ref)` — each one able to enforce its own precondition (e.g., `AdvanceStage` calling `CanTransition` before minting the new token) rather than accepting an arbitrary field bag.

This is the one item in the plan I'd hold Phase 4 on until it's rewritten. Everything else below can ship as scoped.

---

## HIGH — Document/Signature adapters are fully specified while explicitly deferred

**Where:** §4 (package tree), §6.1 (`adapter/port/document.go`, `adapter/port/signature.go`) vs. §16 / `prompt2.md` Category 27

Both `prompt2.md` and this plan's own §16 explicitly list "elaborate document/signature platform" under **What NOT to Build Now**. But §6.1 doesn't gesture at this — it specifies complete Go interfaces: `DocumentGenerator.Generate/Validate`, `SignatureProvider.CreateDraft/Send/Status/Download`, with request/response types fully typed out, and both files are listed in the Phase-1 target package tree in §4 alongside everything that's actually shipping. A coding agent executing this plan literally has no textual signal that these two files are different in kind from `evidence.go` or `executor.go` — they read as equally committed.

If the intent is "these exist only to prove the port pattern is extensible, no implementation ships," say that in one line next to each interface. If the intent is that these actually get built, that's scope creep against the plan's own deferral list and needs to be reconciled, not shipped by accident because it was easier to leave the interfaces in than to decide.

---

## MEDIUM — Two `Executor` interfaces, no stated relationship

**Where:** §4 lists both `service/executor/executor.go` and `adapter/port/executor.go`; §5.5 defines only the former.

Category 13 says "create a formal executor interface outside the kernel" (singular). The plan defines it twice, in two packages, with no text explaining whether `adapter/port/executor.go` is the low-level per-provider contract (`GitHubActionsExecutor` implements it) while `service/executor/executor.go` is a higher-level dispatcher that wraps registered adapter-level executors — which would be a reasonable layering — or whether this is simply an unreconciled duplicate left over from drafting §4 and §5.5 separately. As written, a coding agent would plausibly generate two conflicting `Executor` interfaces. Pick one file, or write the one sentence describing the layering.

---

## MEDIUM — `007_workflow.sql` doesn't get the scrutiny every other schema decision in this project got

**Where:** §4, `db/007_workflow.sql # NEW — workflow tables (if needed)`

Every other schema question across this entire project's history has been resolved with an explicit decision and a stated reason (Category 22: *"A new authority-core table requires justification... prefer read models, projections, service-layer derived data... before adding new kernel tables"*). This is the one place in the plan where a new migration is proposed with a shrug. The Activity Ledger (§5.4) almost certainly needs backing storage — `Record()` appends events with fields (`Actor`, `Operation`, `External`, `Metadata`) that don't exist in any of the four frozen kernel tables — so a table is very likely required. That's fine; it's a **product table outside the authority core**, which Category 22 explicitly permits without the Kernel Growth Gate ADR. But the plan should *say* that, in the same one-sentence form every other schema decision in this codebase's history has been held to, rather than leaving "(if needed)" as the only trace of the decision. This is a two-minute fix and it matters for consistency: the whole discipline this project runs on is that ambiguity about what's authority-core and what isn't gets resolved explicitly, in writing, every time.

---

## MINOR — Scenario D's wording could be misread as mutating an immutable authority

**Where:** §10, Scenario D: *"Agent corrects target — now matches authority... Human approves — execution proceeds."*

Given `target_snapshot` is immutable and `target_activation` binds 1:1 to one snapshot's exact five-tuple, there's no code path in the actual v0 design where an existing authority's target gets "corrected" in place. The sequence only makes sense as: original wrong-target request denied → agent submits a **new** `authority_target` with the corrected tuple → human approves that new target → new snapshot/activation created. That's almost certainly what's intended (a human approval step is shown right before execution, which fits a fresh approval), but the phrase "now matches authority" reads as if the same authority object retroactively applies. Worth one clause making the re-request explicit, so nobody building this scenario is tempted to implement an in-place target correction on an existing authority record.

---

## What's genuinely right here, stated plainly

- The Kernel Growth Gate (§3) is reproduced with its full ADR requirement list intact, not summarized down to something looser.
- Phase sequencing (§11) matches Category 26's five-phase order faithfully, including deferring the demo platform until after the real authorization workflow exists rather than building demos first.
- Scenario D tests the actual mechanism, not a stand-in for it. That's a direct, correct response to the confused-deputy gap identified in the prior agentjacking evaluation.
- `PrepareForAction` re-verifies against `*kernel.Store` directly rather than accepting a token — the one place in the plan where the "token != authority" principle is actually implemented correctly, which makes the CRITICAL finding above a real inconsistency within the plan itself rather than a wholesale misunderstanding of the principle.
- The deferral list (§16) matches `prompt2.md` Category 27 verbatim, including holding the line on multi-tenancy, cryptographic attestation, and global replay prevention — all things this project has independently decided to defer in earlier rounds, for reasons this plan doesn't have to re-litigate because it just carries them forward correctly.

---

## Net

Fix the token (remove `Authority`, type `Refresh`), resolve the document/signature scope question one way or the other, and reconcile the duplicate `Executor` interface. Those three are cheap. The `007_workflow.sql` sentence and the Scenario D wording are cheaper still. None of this requires another architecture pass — this is implementation-detail hardening on a plan whose structure is already sound. Build it.
