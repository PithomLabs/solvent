#!/usr/bin/env bash
# Deploy preflight: verify the cluster we are about to deploy against is the one we think
# it is, using ccloud's structured output. Fails closed.
#
# This is the CockroachDB Cloud CLI doing real work in the pipeline rather than appearing
# in a list of tools. `task deploy` runs it first and aborts on any mismatch.
#
# The check that earns its place is the region one. App Runner's region and the cluster's
# primary region must be equal, or every request in the demo path pays a cross-region
# round trip -- and nothing else in the pipeline would notice. The service would come up,
# serve correct pages, pass its smoke test, and simply be slow for reasons no log
# mentions.
#
# Two things about ccloud that a naive script gets wrong:
#
#   1. It writes a progress spinner to STDERR and JSON to STDOUT. `2>&1 | jq` therefore
#      feeds jq the spinner and fails. Stderr is discarded below, deliberately.
#   2. The org has more than one cluster (great-goat on AWS us-west-2, silver-fish on GCP
#      asia-southeast1). Taking the first element of the array would pass today and
#      silently check the wrong cluster the moment their order changed. The cluster is
#      selected by id.
#
# Region naming: ccloud reports the cloud provider's own region name (`us-west-2`), while
# `SHOW REGIONS FROM DATABASE` reports CockroachDB's prefixed form (`aws-us-west-2`).
# Comparing those two strings directly would report a mismatch that does not exist, so
# this compares provider-form to provider-form.
set -uo pipefail

EXPECT_CLUSTER="${EXPECT_CLUSTER:-great-goat}"
EXPECT_CLUSTER_ID="${EXPECT_CLUSTER_ID:-c995cb24-e07b-4470-bfb3-344c44ce0de1}"
EXPECT_CLOUD="${EXPECT_CLOUD:-AWS}"
EXPECT_VERSION="${EXPECT_VERSION:-v26.2.5}"
APPRUNNER_REGION="${APPRUNNER_REGION:-us-west-2}"

echo "== deploy preflight: ccloud =="

command -v ccloud >/dev/null 2>&1 || {
  echo "PREFLIGHT BLOCKED: ccloud not on PATH"
  echo "  install: https://www.cockroachlabs.com/docs/cockroachcloud/ccloud-get-started"
  exit 1
}

WHO="$(ccloud auth whoami 2>/dev/null)" || {
  echo "PREFLIGHT BLOCKED: ccloud is not authenticated"
  echo "  run: ccloud auth login"
  exit 1
}
echo "  identity : $WHO"

# JSON on stdout, spinner on stderr — keep them apart.
CLUSTERS="$(ccloud cluster list --output json 2>/dev/null)" || {
  echo "PREFLIGHT BLOCKED: ccloud cluster list failed"
  exit 1
}

echo "$CLUSTERS" | EXPECT_CLUSTER="$EXPECT_CLUSTER" EXPECT_CLUSTER_ID="$EXPECT_CLUSTER_ID" \
  EXPECT_CLOUD="$EXPECT_CLOUD" EXPECT_VERSION="$EXPECT_VERSION" \
  APPRUNNER_REGION="$APPRUNNER_REGION" python3 -c '
import json, os, sys

want_id   = os.environ["EXPECT_CLUSTER_ID"]
want_name = os.environ["EXPECT_CLUSTER"]
want_cloud= os.environ["EXPECT_CLOUD"]
want_ver  = os.environ["EXPECT_VERSION"]
want_reg  = os.environ["APPRUNNER_REGION"]

try:
    clusters = json.load(sys.stdin)
except json.JSONDecodeError as e:
    print("  PREFLIGHT BLOCKED: cluster list was not JSON (%s)" % e)
    print("  ccloud writes its spinner to stderr; do not merge streams")
    sys.exit(1)

# Selected by id, not by position: the org holds more than one cluster.
match = [c for c in clusters if c.get("id") == want_id]
if not match:
    print("  PREFLIGHT BLOCKED: no cluster with id %s" % want_id)
    print("  visible: %s" % ", ".join("%s (%s)" % (c.get("name"), c.get("id")) for c in clusters))
    sys.exit(1)
c = match[0]

regions = c.get("regions") or []
primary = next((r.get("name") for r in regions if r.get("primary")), None)
observed = {
    "cluster name"  : c.get("name"),
    "cloud provider": c.get("cloud_provider"),
    "version"       : c.get("cockroach_version"),
    "primary region": primary,
    "state"         : c.get("state"),
}
expected = {
    "cluster name"  : want_name,
    "cloud provider": want_cloud,
    "version"       : want_ver,
    "primary region": want_reg,
    "state"         : "CREATED",
}

fails = []
for k in expected:
    ok = observed[k] == expected[k]
    print("  %-15s %-22s %s" % (k, observed[k], "ok" if ok else "MISMATCH — want %s" % expected[k]))
    if not ok:
        fails.append(k)

routing = (c.get("config") or {}).get("serverless", {}).get("routing_id")
print("  %-15s %s" % ("routing id", routing))

if fails:
    print()
    print("PREFLIGHT BLOCKED — %d mismatch(es): %s" % (len(fails), ", ".join(fails)))
    if "primary region" in fails:
        print("  A region mismatch adds a cross-region round trip to every request in the")
        print("  demo path. The service would still come up and still pass its smoke test.")
    sys.exit(1)

print()
print("PREFLIGHT GREEN — %s (%s) %s, primary region %s == App Runner %s" % (
    observed["cluster name"], want_id[:8], observed["version"], primary, want_reg))
'
