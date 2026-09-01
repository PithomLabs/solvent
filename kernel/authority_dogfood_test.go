package kernel_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/PithomLabs/solvent/kernel"
)

// --- dogfood test helpers ---------------------------------------------------

// dogfoodScenario returns a deterministic scenario UUID for DocTrust-shaped tests.
func dogfoodScenario(n int) string {
	return fmt.Sprintf("33333333-0000-0000-0000-%012x", n)
}

// dogfoodPrincipal creates a principal and returns its ID.
func dogfoodPrincipal(t *testing.T, ctx context.Context, st *kernel.Store) string {
	t.Helper()
	id, err := st.CreatePrincipal(ctx, "agent", "test-issuer")
	if err != nil {
		t.Fatalf("setup (create principal): %v", err)
	}
	return id
}

// dogfoodBelief creates and promotes a belief, returning its ID.
// Uses mustPromoted (defined in kernel_test.go) which handles debt retirement.
func dogfoodBelief(t *testing.T, ctx context.Context, st *kernel.Store, sc string) string {
	t.Helper()
	return mustPromoted(t, ctx, st, sc, "dogfood belief")
}

// dogfoodTarget creates a standard target and returns its ID.
func dogfoodTarget(t *testing.T, ctx context.Context, st *kernel.Store, principalID string) string {
	t.Helper()
	params, _ := json.Marshal(map[string]string{"env": "prod"})
	id, err := st.CreateTarget(ctx, principalID,
		"service", "svc-1", "deploy",
		"compute", "restart", "downtime", params, principalID)
	if err != nil {
		t.Fatalf("setup (create target): %v", err)
	}
	return id
}

// dogfoodTuple returns an AuthorityTuple matching the standard dogfood target.
func dogfoodTuple(pid string) kernel.AuthorityTuple {
	params, _ := json.Marshal(map[string]string{"env": "prod"})
	return kernel.AuthorityTuple{
		PrincipalID:           pid,
		ResourceType:          "service",
		ResourceID:            "svc-1",
		Scope:                 "deploy",
		ActionNamespace:       "compute",
		ActionName:            "restart",
		ConsequenceType:       "downtime",
		ConsequenceParameters: params,
	}
}

// dogfoodApproved sets up the full approval chain: principal, belief, target,
// justification, request, approve. Returns (principalID, targetID, beliefID).
func dogfoodApproved(t *testing.T, ctx context.Context, st *kernel.Store, sc string) (string, string, string) {
	t.Helper()
	pid := dogfoodPrincipal(t, ctx, st)
	bid := dogfoodBelief(t, ctx, st, sc)
	tid := dogfoodTarget(t, ctx, st, pid)
	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)
	if err := st.Approve(ctx, tid, pid); err != nil {
		t.Fatalf("setup (approve): %v", err)
	}
	return pid, tid, bid
}

// --- DocTrust-shaped integration tests --------------------------------------

// DT-1: Exact tuple match → ALLOW.
func TestDocTrust_ExactTuple(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(1)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)
	result, err := st.Authorize(ctx, tid, dogfoodTuple(pid))

	ok := err == nil && result.Allowed
	rec.check(t, ok, Case{
		ID: "DT-1", Wave: "doctrust",
		Purpose:   "Exact tuple match yields ALLOW",
		Expected:  "Allowed=true",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q, err=%v", result.Allowed, result.Reason, err),
		Invariant: "Authorize allows on exact tuple match",
		Receipt:   "",
	})
}

// DT-2: Principal mismatch → DENY.
func TestDocTrust_PrincipalMismatch(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(2)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)
	wrongPID := dogfoodPrincipal(t, ctx, st)

	tuple := dogfoodTuple(pid)
	tuple.PrincipalID = wrongPID
	result, _ := st.Authorize(ctx, tid, tuple)

	ok := !result.Allowed
	rec.check(t, ok, Case{
		ID: "DT-2", Wave: "doctrust",
		Purpose:   "Principal mismatch yields DENY",
		Expected:  "Allowed=false",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q", result.Allowed, result.Reason),
		Invariant: "Authorize denies on principal mismatch",
		Receipt:   "",
	})
}

// DT-3: Resource mismatch → DENY.
func TestDocTrust_ResourceMismatch(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(3)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)

	tuple := dogfoodTuple(pid)
	tuple.ResourceType = "database"
	result, _ := st.Authorize(ctx, tid, tuple)

	ok := !result.Allowed
	rec.check(t, ok, Case{
		ID: "DT-3", Wave: "doctrust",
		Purpose:   "Resource type mismatch yields DENY",
		Expected:  "Allowed=false",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q", result.Allowed, result.Reason),
		Invariant: "Authorize denies on resource type mismatch",
		Receipt:   "",
	})
}

// DT-4: Scope mismatch → DENY.
func TestDocTrust_ScopeMismatch(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(4)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)

	tuple := dogfoodTuple(pid)
	tuple.Scope = "read"
	result, _ := st.Authorize(ctx, tid, tuple)

	ok := !result.Allowed
	rec.check(t, ok, Case{
		ID: "DT-4", Wave: "doctrust",
		Purpose:   "Scope mismatch yields DENY",
		Expected:  "Allowed=false",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q", result.Allowed, result.Reason),
		Invariant: "Authorize denies on scope mismatch",
		Receipt:   "",
	})
}

// DT-5: Action mismatch → DENY.
func TestDocTrust_ActionMismatch(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(5)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)

	tuple := dogfoodTuple(pid)
	tuple.ActionName = "rollback"
	result, _ := st.Authorize(ctx, tid, tuple)

	ok := !result.Allowed
	rec.check(t, ok, Case{
		ID: "DT-5", Wave: "doctrust",
		Purpose:   "Action name mismatch yields DENY",
		Expected:  "Allowed=false",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q", result.Allowed, result.Reason),
		Invariant: "Authorize denies on action name mismatch",
		Receipt:   "",
	})
}

// DT-6: Consequence mismatch → DENY.
func TestDocTrust_ConsequenceMismatch(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(6)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)

	tuple := dogfoodTuple(pid)
	tuple.ConsequenceType = "data_loss"
	result, _ := st.Authorize(ctx, tid, tuple)

	ok := !result.Allowed
	rec.check(t, ok, Case{
		ID: "DT-6", Wave: "doctrust",
		Purpose:   "Consequence type mismatch yields DENY",
		Expected:  "Allowed=false",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q", result.Allowed, result.Reason),
		Invariant: "Authorize denies on consequence type mismatch",
		Receipt:   "",
	})
}

// DT-7: Belief retraction → DENY. Retraction succeeds (FK CASCADE), Authorize denies.
func TestDocTrust_BeliefRetraction(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(7)

	pid, tid, bid := dogfoodApproved(t, ctx, st, sc)

	// Confirm ALLOW before retraction.
	pre, _ := st.Authorize(ctx, tid, dogfoodTuple(pid))

	// Retract. FK CASCADE propagates status to justification.
	retracted, retractErr := st.RetractCascade(ctx, sc, bid)

	// Confirm DENY after retraction.
	post, _ := st.Authorize(ctx, tid, dogfoodTuple(pid))

	ok := pre.Allowed &&
		retractErr == nil && retracted >= 1 &&
		!post.Allowed
	rec.check(t, ok, Case{
		ID: "DT-7", Wave: "doctrust",
		Purpose:   "Belief retraction causes Authorize DENY",
		Expected:  "ALLOW before, retraction succeeds, DENY after",
		Observed:  fmt.Sprintf("pre=%t, retracted=%d, post=%t", pre.Allowed, retracted, post.Allowed),
		Invariant: "Authority cannot survive a retracted belief",
		Receipt:   "",
	})
}

// DT-8: Re-promotion gap. Retract → re-promote → old justification may revive.
// ACCEPTED V0 GAP.
func TestDocTrust_RepromotionGap(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(8)

	pid, tid, bid := dogfoodApproved(t, ctx, st, sc)

	// Retract then re-promote.
	_, _ = st.RetractCascade(ctx, sc, bid)
	_ = st.Promote(ctx, bid)

	// Authorize may now ALLOW again — accepted v0 gap.
	result, err := st.Authorize(ctx, tid, dogfoodTuple(pid))

	ok := err == nil
	rec.check(t, ok, Case{
		ID: "DT-8", Wave: "doctrust",
		Purpose:   "ACCEPTED V0 GAP: re-promoted belief may revive old justification",
		Expected:  "Authorize completes (ALLOW or DENY depending on re-promotion timing)",
		Observed:  fmt.Sprintf("allowed=%t, reason=%q", result.Allowed, result.Reason),
		Invariant: "V0 ACCEPTED GAP: no promotion_epoch in v0",
		Receipt:   "ACCEPTED V0 GAP — no promotion_epoch in v0",
	})
}

// DT-9: Approval pin mutation. RequestAuth → add justification → Approve → DENY.
func TestDocTrust_ApprovalPinMutation(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(9)

	pid := dogfoodPrincipal(t, ctx, st)
	bid := dogfoodBelief(t, ctx, st, sc)
	bid2 := dogfoodBelief(t, ctx, st, dogfoodScenario(90))
	tid := dogfoodTarget(t, ctx, st, pid)

	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)

	// Mutation: add a second justification after pinning.
	_ = st.AttachJustification(ctx, tid, bid2, "promoted", pid)

	err := st.Approve(ctx, tid, pid)

	// Approval should fail (pin mismatch).
	ok := err != nil
	rec.check(t, ok, Case{
		ID: "DT-9", Wave: "doctrust",
		Purpose:   "Approval pin mutation yields rejection",
		Expected:  "Approve fails (pin mismatch)",
		Observed:  fmt.Sprintf("err=%v", err),
		Invariant: "Approval pin prevents silent authority expansion",
		Receipt:   receiptOf(err),
	})
}

// DT-10: Resurrection. Approve → RevokeTarget → attempt re-activation → failure.
func TestDocTrust_Resurrection(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(10)

	pid := dogfoodPrincipal(t, ctx, st)
	bid := dogfoodBelief(t, ctx, st, sc)
	tid := dogfoodTarget(t, ctx, st, pid)

	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)
	_ = st.Approve(ctx, tid, pid)
	_ = st.RevokeTarget(ctx, tid, pid, "test revocation")

	// Attempt re-approval on same target. Should fail (UNIQUE(target_id)).
	err := st.Approve(ctx, tid, pid)

	ok := err != nil
	rec.check(t, ok, Case{
		ID: "DT-10", Wave: "doctrust",
		Purpose:   "Resurrection impossible: second activation fails",
		Expected:  "Approve fails after revocation",
		Observed:  fmt.Sprintf("err=%v", err),
		Invariant: "UNIQUE(target_id) prevents re-activation",
		Receipt:   receiptOf(err),
	})
}

// DT-11: Duplicate discharge → reject. Second exact discharge is rejected.
func TestDocTrust_DuplicateDischarge(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(11)

	pid := dogfoodPrincipal(t, ctx, st)
	bid := dogfoodBelief(t, ctx, st, sc)

	err1 := st.Discharge(ctx, bid, "obligation-1", "instrument-1", pid)
	err2 := st.Discharge(ctx, bid, "obligation-1", "instrument-1", pid)

	ok := err1 == nil && err2 != nil
	rec.check(t, ok, Case{
		ID: "DT-11", Wave: "doctrust",
		Purpose:   "Duplicate discharge rejected",
		Expected:  "first succeeds, second rejects",
		Observed:  fmt.Sprintf("err1=%v, err2=%v", err1, err2),
		Invariant: "UNIQUE(belief_id, obligation_key, instrument_ref, discharged_by) rejects duplicate",
		Receipt:   receiptOf(err2),
	})
}

// DT-12: Revoked approver → DENY.
func TestDocTrust_RevokedApprover(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(12)

	pid := dogfoodPrincipal(t, ctx, st)
	bid := dogfoodBelief(t, ctx, st, sc)
	tid := dogfoodTarget(t, ctx, st, pid)

	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)
	_ = st.RequestAuthorization(ctx, tid, pid)

	// Revoke the approver before approval.
	_ = st.RevokePrincipal(ctx, pid)
	err := st.Approve(ctx, tid, pid)

	ok := err != nil
	rec.check(t, ok, Case{
		ID: "DT-12", Wave: "doctrust",
		Purpose:   "Revoked approver cannot Approve",
		Expected:  "Approve fails (revoked principal)",
		Observed:  fmt.Sprintf("err=%v", err),
		Invariant: "Revoked principal blocked from approval",
		Receipt:   receiptOf(err),
	})
}

// DT-13: Authorize is zero-write. Verify no new rows in authority tables.
func TestDocTrust_AuthorizeZeroWrite(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(13)

	pid, tid, _ := dogfoodApproved(t, ctx, st, sc)

	// Count before.
	var snapBefore, actBefore, revBefore, justBefore, disBefore int
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM target_snapshot`).Scan(&snapBefore)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM target_activation`).Scan(&actBefore)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM target_revocation`).Scan(&revBefore)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM justification`).Scan(&justBefore)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM debt_discharge`).Scan(&disBefore)

	// Call Authorize.
	_, _ = st.Authorize(ctx, tid, dogfoodTuple(pid))

	// Count after — must be unchanged.
	var snapAfter, actAfter, revAfter, justAfter, disAfter int
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM target_snapshot`).Scan(&snapAfter)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM target_activation`).Scan(&actAfter)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM target_revocation`).Scan(&revAfter)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM justification`).Scan(&justAfter)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM debt_discharge`).Scan(&disAfter)

	ok := snapBefore == snapAfter && actBefore == actAfter && revBefore == revAfter &&
		justBefore == justAfter && disBefore == disAfter
	rec.check(t, ok, Case{
		ID: "DT-13", Wave: "doctrust",
		Purpose:   "Authorize performs zero writes",
		Expected:  "all authority table counts unchanged",
		Observed:  fmt.Sprintf("snap=%d/%d, act=%d/%d, rev=%d/%d, just=%d/%d, dis=%d/%d", snapBefore, snapAfter, actBefore, actAfter, revBefore, revAfter, justBefore, justAfter, disBefore, disAfter),
		Invariant: "Authorize is read-only: no INSERT, no UPDATE, no DELETE",
		Receipt:   "",
	})
}

// DT-14: Full lifecycle. Complete production scenario with attack variants.
func TestDocTrust_FullLifecycle(t *testing.T) {
	rec.begin("doctrust")
	ctx := context.Background()
	st := kernel.New(shared)
	sc := dogfoodScenario(14)

	// 1. Create principal.
	pid := dogfoodPrincipal(t, ctx, st)

	// 2. Create/promote belief.
	bid := dogfoodBelief(t, ctx, st, sc)

	// 3. Create target.
	tid := dogfoodTarget(t, ctx, st, pid)

	// 4. Attach justification.
	_ = st.AttachJustification(ctx, tid, bid, "promoted", pid)

	// 5. Request authorization.
	_ = st.RequestAuthorization(ctx, tid, pid)

	// 6. Approve.
	approveErr := st.Approve(ctx, tid, pid)

	// 7. Authorize(ALLOW).
	result, _ := st.Authorize(ctx, tid, dogfoodTuple(pid))

	// 8. RevokeTarget.
	_ = st.RevokeTarget(ctx, tid, pid, "lifecycle test")

	// 9. Authorize(DENY).
	afterRevoke, _ := st.Authorize(ctx, tid, dogfoodTuple(pid))

	// Attack: wrong principal.
	wrongPID := dogfoodPrincipal(t, ctx, st)
	wrongTuple := dogfoodTuple(pid)
	wrongTuple.PrincipalID = wrongPID
	wrongPrincipal, _ := st.Authorize(ctx, tid, wrongTuple)

	// Attack: wrong resource.
	wrongResource := dogfoodTuple(pid)
	wrongResource.ResourceType = "database"
	wrongResResult, _ := st.Authorize(ctx, tid, wrongResource)

	ok := approveErr == nil &&
		result.Allowed &&
		!afterRevoke.Allowed &&
		!wrongPrincipal.Allowed &&
		!wrongResResult.Allowed
	rec.check(t, ok, Case{
		ID: "DT-14", Wave: "doctrust",
		Purpose:   "Full lifecycle: approve → allow → revoke → deny + attack variants",
		Expected:  "approve succeeds, authorize allows, revoke denies, wrong principal denies, wrong resource denies",
		Observed:  fmt.Sprintf("approve=%v, allow=%t, after_revoke=%t, wrong_principal=%t, wrong_resource=%t", approveErr, result.Allowed, afterRevoke.Allowed, wrongPrincipal.Allowed, wrongResResult.Allowed),
		Invariant: "Full authority lifecycle with attack variants",
		Receipt:   "",
	})
}
