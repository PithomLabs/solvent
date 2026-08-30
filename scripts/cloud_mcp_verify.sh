#!/usr/bin/env bash
# Verify the CockroachDB Cloud Managed MCP Server end to end, with one read-only query.
#
# This is the single step between "additionally configured" and a claimed, verified tool.
# It is a separate script from scripts/mcp_verify.sh because the two servers are different
# things: that one is ours, stdio, six domain tools; this one is Cockroach Labs' own hosted
# HTTP endpoint that connects an agent directly to the cluster for read-only SQL.
#
# The credential is read from the environment and nowhere else. It is never written to a
# file, never echoed, and never added to .mcp.json -- which is tracked, and would put a
# bearer token in git. This follows the policy already stated in scripts/demo/config.env.
#
# The mcp-cluster-id header value has never been confirmed: the Cloud Console may expect
# the cluster UUID or the SQL-DNS routing prefix. Rather than guess, both are tried and the
# script reports which one the endpoint accepted, so the README can then state a fact.
set -uo pipefail

ENDPOINT="${CC_MCP_ENDPOINT:-https://cockroachlabs.cloud/mcp}"
CLUSTER_UUID="${CC_MCP_CLUSTER_UUID:-c995cb24-e07b-4470-bfb3-344c44ce0de1}"
CLUSTER_ROUTING="${CC_MCP_CLUSTER_ROUTING:-great-goat-30894}"
KEY="${CC_MCP_API_KEY:-}"

if [ -z "$KEY" ]; then
  cat <<'MSG'
CLOUD MCP VERIFY BLOCKED: no credential.

  export CC_MCP_API_KEY='<service-account-key>'
  scripts/cloud_mcp_verify.sh

Issue the key in the CockroachDB Cloud Console under the cluster's MCP page (or complete
the interactive OAuth 2.1 flow, which works for a human at a terminal but not for an
autonomous agent). Creating credentials is deliberately not automated here.

Until this script passes, README.md must keep the Cloud Managed MCP Server listed as
"additionally configured, verification pending" and MUST NOT count it as a claimed tool.
MSG
  exit 1
fi

echo "== Cloud Managed MCP: unauthenticated request must be refused =="
code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 30 -X POST "$ENDPOINT" \
        -H 'Content-Type: application/json' \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}')"
if [ "$code" = "401" ] || [ "$code" = "403" ]; then
  echo "  ok   HTTP $code without a credential"
else
  echo "  WARN HTTP $code without a credential (expected 401/403)"
fi

# Try both header forms. One of them is right; the docs do not say which.
verified=""
for form in uuid routing; do
  case "$form" in
    uuid)    cid="$CLUSTER_UUID" ;;
    routing) cid="$CLUSTER_ROUTING" ;;
  esac

  echo
  echo "== authenticated, mcp-cluster-id = <$form> =="
  body="$(curl -s --max-time 60 -X POST "$ENDPOINT" \
          -H 'Content-Type: application/json' \
          -H 'Accept: application/json, text/event-stream' \
          -H "mcp-cluster-id: $cid" \
          -H "Authorization: Bearer $KEY" \
          -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cloud_mcp_verify","version":"1"}}}')"

  if printf '%s' "$body" | grep -qi '"result"'; then
    echo "  ok   initialize accepted"
    printf '%s' "$body" | python3 -c '
import json,sys,re
raw = sys.stdin.read()
# The endpoint may answer as SSE; take the first JSON object in the body.
m = re.search(r"\{.*\}", raw, re.S)
try:
    d = json.loads(m.group(0)) if m else {}
except Exception:
    d = {}
info = (d.get("result") or {}).get("serverInfo") or {}
if info:
    print("  server: %s %s" % (info.get("name"), info.get("version")))
' 2>/dev/null
    verified="$form"
    break
  fi

  echo "  no   rejected"
  printf '%s' "$body" | head -c 300 | sed 's/^/       /'
  echo
done

echo
if [ -z "$verified" ]; then
  cat <<'MSG'
CLOUD MCP VERIFY BLOCKED: neither mcp-cluster-id form was accepted.

Take the exact header value from the Cloud Console's MCP page snippet and re-run with:
  CC_MCP_CLUSTER_UUID='<value-from-the-console>' scripts/cloud_mcp_verify.sh

Do NOT promote the Managed MCP Server into README.md's claimed-tools table on the strength
of a configured endpoint. A configured endpoint is not a verified tool.
MSG
  exit 1
fi

echo "== read-only query through the endpoint =="
echo "  The remaining step is agent-side: point Claude Code at the endpoint and run one of"
echo "  the three read-only questions in README.md, e.g. the corpus_issue vector-index one."
echo
echo "  claude mcp add cockroachdb-cloud $ENDPOINT \\"
echo "    --transport http \\"
case "$verified" in
  uuid)    echo "    --header \"mcp-cluster-id: $CLUSTER_UUID\" \\" ;;
  routing) echo "    --header \"mcp-cluster-id: $CLUSTER_ROUTING\" \\" ;;
esac
echo "    --header \"Authorization: Bearer \$CC_MCP_API_KEY\""
echo
echo "CLOUD MCP VERIFY GREEN — endpoint authenticated with mcp-cluster-id = <$verified>."
echo "README.md may now record that header form as confirmed. Promote the Managed MCP"
echo "Server into the claimed-tools table only after a real query returns real rows."
