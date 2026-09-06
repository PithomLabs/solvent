"""
Solvent API Reference Client

Handwritten reference client for the Solvent Authorization Kernel API.
This is NOT an SDK — it demonstrates the minimum surface needed to
interact with the canonical v1 HTTP API.

Usage:
    client = SolventClient("http://localhost:8080", "your-api-key")
    principal = client.create_principal("service", "my-agent")
"""

import json
from urllib.request import Request, urlopen
from urllib.error import HTTPError


class SolventClient:
    """Minimal reference client for the Solvent API."""

    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key

    def _request(self, method: str, path: str, body=None):
        """Make an authenticated HTTP request."""
        url = f"{self.base_url}{path}"
        data = json.dumps(body).encode() if body else None
        req = Request(url, data=data, method=method)
        req.add_header("Authorization", f"Bearer {self.api_key}")
        if body is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with urlopen(req) as resp:
                return json.loads(resp.read())
        except HTTPError as e:
            error_body = json.loads(e.read())
            raise SolventError(e.code, error_body) from None

    # ── Principals ──────────────────────────────

    def create_principal(self, principal_type: str, issuer: str) -> dict:
        return self._request("POST", "/v1/principals", {
            "principal_type": principal_type,
            "issuer": issuer,
        })

    # ── Beliefs ─────────────────────────────────

    def enter_belief(self, scenario_id: str, claim: str, claim_type: str) -> dict:
        return self._request("POST", "/v1/beliefs", {
            "scenario_id": scenario_id,
            "claim": claim,
            "claim_type": claim_type,
        })

    def promote_belief(self, belief_id: str, scenario_id: str) -> dict:
        return self._request("POST", f"/v1/beliefs/{belief_id}/promote?scenario_id={scenario_id}")

    def retire_debt(self, belief_id: str, debt_item: str, scenario_id: str = "") -> dict:
        path = f"/v1/beliefs/{belief_id}/debt/retire"
        if scenario_id:
            path += f"?scenario_id={scenario_id}"
        return self._request("POST", path, {
            "debt_item": debt_item,
        })

    # ── Targets ─────────────────────────────────

    def create_target(self, principal_id: str, resource_type: str, resource_id: str,
                      scope: str, action_namespace: str, action_name: str,
                      consequence_type: str, created_by: str,
                      consequence_parameters=None) -> dict:
        body = {
            "principal_id": principal_id,
            "resource_type": resource_type,
            "resource_id": resource_id,
            "scope": scope,
            "action_namespace": action_namespace,
            "action_name": action_name,
            "consequence_type": consequence_type,
            "consequence_parameters": consequence_parameters or {},
            "created_by": created_by,
        }
        return self._request("POST", "/v1/targets", body)

    def attach_justification(self, target_id: str, belief_id: str, instrument_ref: str) -> dict:
        return self._request(
            "POST",
            f"/v1/targets/{target_id}/justifications?belief_id={belief_id}",
            {"instrument_ref": instrument_ref},
        )

    def request_authorization(self, target_id: str) -> dict:
        return self._request("POST", f"/v1/targets/{target_id}/request")

    def approve_target(self, target_id: str, approval_pin: str) -> dict:
        return self._request("POST", f"/v1/targets/{target_id}/approve", {
            "approval_pin": approval_pin,
        })

    # ── Authorizations ──────────────────────────

    def verify_authorization(self, target_id: str, resource_type: str, resource_id: str,
                             scope: str, action_namespace: str, action_name: str,
                             consequence_type: str, consequence_parameters=None) -> dict:
        body = {
            "target_id": target_id,
            "resource_type": resource_type,
            "resource_id": resource_id,
            "scope": scope,
            "action_namespace": action_namespace,
            "action_name": action_name,
            "consequence_type": consequence_type,
            "consequence_parameters": consequence_parameters or {},
        }
        return self._request("POST", "/v1/authorizations/verify", body)

    def authorize_action(self, scenario_id: str, belief_id: str, action: str,
                         target_id: str, action_source: str = "user_typed",
                         actor_id: str = None) -> dict:
        """Authorize an action against a promoted belief.

        actor_id, when supplied, must equal the authenticated principal;
        the API derives the effective principal from authentication (Phase 6.1).
        """
        body = {
            "scenario_id": scenario_id,
            "belief_id": belief_id,
            "action": action,
            "action_source": action_source,
            "target_id": target_id,
        }
        if actor_id is not None:
            body["actor_id"] = actor_id
        return self._request("POST", "/v1/authorizations/action", body)


class SolventError(Exception):
    """Error returned by the Solvent API."""

    def __init__(self, status_code: int, body: dict):
        self.status_code = status_code
        self.body = body
        super().__init__(f"Solvent API error {status_code}: {body.get('message', '')}")
