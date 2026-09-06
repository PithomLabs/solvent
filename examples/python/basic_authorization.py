"""
Solvent Basic Authorization Flow

Demonstrates the authorization lifecycle against the frozen v1 API:
1. Authenticate (via API key)
2. Create principal
3. Enter belief
4. Retire debts
5. Promote belief
6. Create target
7. Attach justification
8. Request authorization
9. Verify authority (returns false — target not yet approved)

Note: Step 8 (approve) is skipped because the frozen v1 API does not
expose the approval pin hash in TargetResponse. This is a known v1
limitation, not introduced by Phase 4C. The approval credential must
be obtained through a separately secured channel.

This example is deliberately dumb — it orchestrates API calls in sequence.
It is NOT a workflow engine.

Usage:
    SOLVENT_URL=http://localhost:8080 SOLVENT_API_KEY=your-key python basic_authorization.py
"""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from client import SolventClient, SolventError


def main():
    base_url = os.environ.get("SOLVENT_URL", "http://localhost:8080")
    api_key = os.environ.get("SOLVENT_API_KEY", "")

    if not api_key:
        print("Set SOLVENT_API_KEY environment variable")
        sys.exit(1)

    client = SolventClient(base_url, api_key)

    try:
        # 1. Create principal
        principal = client.create_principal("service", "example-agent")
        principal_id = principal["principal_id"]
        print(f"1. Created principal: {principal_id}")

        # 2. Enter belief
        scenario_id = "00000000-0000-0000-0000-000000000001"
        belief = client.enter_belief(
            scenario_id=scenario_id,
            claim="etcd v3.5.0 is safe to deploy",
            claim_type="derived",
        )
        belief_id = belief["belief_id"]
        print(f"2. Entered belief: {belief_id}")

        # 3. Retire all debts
        debts = [
            "needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
            "needRollbackPlan", "needVersionPin", "needOperatorSignoff",
        ]
        for debt in debts:
            client.retire_debt(belief_id, debt, scenario_id=scenario_id)
        print(f"3. Retired {len(debts)} debts")

        # 4. Promote belief
        promoted = client.promote_belief(belief_id, scenario_id)
        status = promoted.get("status", "unknown")
        print(f"4. Belief status: {status}")

        # 5. Create target
        target = client.create_target(
            principal_id=principal_id,
            resource_type="scenario",
            resource_id=scenario_id,
            scope=f"belief:{belief_id}",
            action_namespace="solvent",
            action_name="deploy",
            consequence_type="execution",
            created_by=principal_id,
        )
        target_id = target["target_id"]
        print(f"5. Created target: {target_id}")

        # 6. Attach justification
        client.attach_justification(target_id, belief_id, "github-issue-12345")
        print("6. Attached justification")

        # 8. Request authorization
        # Note: The approval pin (hash) is computed server-side and stored
        # in authority_target.pinned_request_hash, but the API does not return
        # it in the TargetResponse. This is a known v1 limitation — the caller
        # cannot approve without knowing the hash. The credential must be
        # obtained through a separately secured channel.
        client.request_authorization(target_id)
        print("8. Requested authorization")

        # 9. Verify authority (before approval — target is in "requested" state)
        # The target is not approved, so verification returns allowed=False.
        # In a complete flow, approval would precede verification.
        result = client.verify_authorization(
            target_id=target_id,
            resource_type="scenario",
            resource_id=scenario_id,
            scope=f"belief:{belief_id}",
            action_namespace="solvent",
            action_name="deploy",
            consequence_type="execution",
        )
        print(f"9. Verify result: allowed={result.get('allowed')}")

        # 10. Authorize action
        # Note: This step requires the target to be approved. The approval
        # step (8) is skipped because the API does not expose the approval
        # pin. In a complete flow, step 8 would approve the target, and
        # this step would succeed.
        #
        # actor_id, when supplied, must equal the authenticated principal;
        # the API derives the effective principal from authentication (Phase 6.1).
        action_result = client.authorize_action(
            scenario_id=scenario_id,
            belief_id=belief_id,
            action="deploy etcd v3.5.0",
            target_id=target_id,
        )
        authority = action_result.get("authority", {})
        print(f"10. Action authorized: allowed={authority.get('allowed')}")
        if authority.get("allowed"):
            print(f"   Intent state: {action_result.get('intent_state')}")
        else:
            print(f"   Reason: {authority.get('reason')}")

    except SolventError as e:
        print(f"Error: {e}")
        print(f"Details: {e.body}")
        sys.exit(1)


if __name__ == "__main__":
    main()
