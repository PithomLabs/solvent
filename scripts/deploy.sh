#!/usr/bin/env bash
# Deploy the cloud demo to AWS App Runner, in six steps, failing closed at each.
#
# Until now this sequence existed only as prose in a phase report, which meant every
# deploy was hand-assembled from a document. Two of the steps below exist specifically
# because doing it by hand went wrong:
#
#   Step 4 re-asserts the instance role and FABLE_DSN because `update-service` takes a
#   --source-configuration that REPLACES rather than merges. Omit the environment block
#   and the service starts, then fails on database connection -- which reads like an
#   outage, not a configuration error. It is re-read afterwards rather than assumed.
#
#   Step 1 is the ccloud preflight, because a region mismatch between App Runner and the
#   cluster's primary region silently adds a cross-region round trip to every request and
#   nothing downstream notices: the service comes up, serves correct pages, and passes
#   its own smoke test.
#
# Nothing here creates or reads a credential. The AWS profile and the ccloud session are
# the operator's, supplied from the environment.
set -uo pipefail
cd "$(dirname "$0")/.."

REGION="${AWS_REGION:-us-west-2}"
ACCOUNT="${AWS_ACCOUNT:-705029439164}"
REPO_NAME="${ECR_REPO:-solvent-demo}"
REPO="$ACCOUNT.dkr.ecr.$REGION.amazonaws.com/$REPO_NAME"
SERVICE_ARN="${APPRUNNER_SERVICE_ARN:-arn:aws:apprunner:$REGION:$ACCOUNT:service/solvent-demo/c1d9bd1c3dc94ced8f0bd4776433aa23}"
ROLE_ARN="${APPRUNNER_INSTANCE_ROLE_ARN:-arn:aws:iam::$ACCOUNT:role/solvent-demo-apprunner-instance-role}"
SERVICE_URL="${SERVICE_URL:-https://byb43s8nh2.us-west-2.awsapprunner.com}"
TAG="${DEPLOY_TAG:-$(git rev-parse --short HEAD)}"

die() { echo; echo "DEPLOY BLOCKED at step $1: $2"; exit 1; }

echo "=============================================================="
echo " deploy $REPO_NAME:$TAG  ->  App Runner $REGION"
echo "=============================================================="

# ---- 1. preflight -------------------------------------------------------------
./scripts/ccloud_preflight.sh || die 1 "ccloud preflight failed; the cluster is not the expected one"

# The cluster's primary region must also be the region we are deploying into.
APPRUNNER_REGION="$REGION" ./scripts/ccloud_preflight.sh >/dev/null 2>&1 \
  || die 1 "cluster primary region does not match the App Runner region ($REGION)"

# ---- 2. build + push ----------------------------------------------------------
echo
echo "== 2. build + push =="
aws ecr get-login-password --region "$REGION" 2>/dev/null \
  | docker login --username AWS --password-stdin "$ACCOUNT.dkr.ecr.$REGION.amazonaws.com" >/dev/null 2>&1 \
  || die 2 "ECR login failed"
docker build -q -t "$REPO:$TAG" . >/dev/null || die 2 "docker build failed"
echo "  built  $REPO:$TAG"
docker push -q "$REPO:$TAG" >/dev/null || die 2 "docker push failed"
echo "  pushed"

# ---- 3. update the service ----------------------------------------------------
echo
echo "== 3. update-service =="
aws apprunner update-service --service-arn "$SERVICE_ARN" --region "$REGION" \
  --source-configuration "ImageRepository={ImageIdentifier=$REPO:$TAG,ImageRepositoryType=ECR,ImageConfiguration={Port=8080}}" \
  >/dev/null 2>&1 || die 3 "update-service failed"
echo "  image -> $REPO:$TAG"

# ---- 4. re-assert configuration, then verify it ------------------------------
echo
echo "== 4. re-assert InstanceRoleArn + FABLE_DSN =="
aws apprunner update-service --service-arn "$SERVICE_ARN" --region "$REGION" \
  --instance-configuration "InstanceRoleArn=$ROLE_ARN" >/dev/null 2>&1 || true

aws apprunner describe-service --service-arn "$SERVICE_ARN" --region "$REGION" --output json 2>/dev/null \
  | ROLE_ARN="$ROLE_ARN" python3 -c '
import json, os, sys
s = json.load(sys.stdin)["Service"]
role = (s.get("InstanceConfiguration") or {}).get("InstanceRoleArn")
env  = ((s["SourceConfiguration"].get("ImageRepository") or {}).get("ImageConfiguration") or {}).get("RuntimeEnvironmentVariables") or {}
bad = []
if role != os.environ["ROLE_ARN"]:
    bad.append("InstanceRoleArn is %r, want %r" % (role, os.environ["ROLE_ARN"]))
    print("  role   MISSING — /demo search will fail with an explicit Bedrock permission error")
else:
    print("  role   %s" % role.split("/")[-1])
if "FABLE_DSN" not in env:
    bad.append("FABLE_DSN absent from RuntimeEnvironmentVariables")
    print("  env    FABLE_DSN MISSING — the service will start and fail on database connection")
else:
    print("  env    %s" % sorted(env))
sys.exit(1 if bad else 0)
' || die 4 "configuration did not survive update-service (it REPLACES, it does not merge)"

# ---- 5. wait for RUNNING ------------------------------------------------------
echo
echo "== 5. wait for RUNNING =="
for _ in $(seq 1 60); do
  st="$(aws apprunner describe-service --service-arn "$SERVICE_ARN" --region "$REGION" \
        --query 'Service.Status' --output text 2>/dev/null || echo QUERY_FAILED)"
  case "$st" in
    RUNNING) echo "  RUNNING"; break ;;
    OPERATION_IN_PROGRESS) sleep 20 ;;
    *) die 5 "unexpected service status: $st" ;;
  esac
done
[ "${st:-}" = "RUNNING" ] || die 5 "service did not reach RUNNING"

# ---- 6. cloud smoke tests -----------------------------------------------------
echo
echo "== 6. cloud smoke =="
smoke_fail=0
for p in /health /demo /proof /ledger; do
  code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 45 "$SERVICE_URL$p")"
  printf "  %-8s HTTP=%s\n" "$p" "$code"
  [ "$code" = "200" ] || smoke_fail=1
done

# The one smoke check that proves the instance role works: a real Bedrock embedding.
echo "  live Bedrock search:"
curl -s --max-time 60 -c /dev/null -H 'Content-Type: application/json' \
  -d '{"query":"is etcd v3.5.x safe to deploy?"}' "$SERVICE_URL/demo/api/search" \
  | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    print("    FAIL: search returned no JSON"); sys.exit(1)
if d.get("error"):
    print("    FAIL:", str(d["error"])[:200]); sys.exit(1)
hits = d.get("hits") or []
if not hits:
    print("    FAIL: no hits"); sys.exit(1)
print("    model %s, %d tokens, %d hits, nearest #%d at %.6f" % (
    d.get("model"), d.get("query_tokens", 0), len(hits),
    hits[0]["issue_number"], hits[0]["distance"]))
' || smoke_fail=1

echo
if [ "$smoke_fail" -ne 0 ]; then
  echo "DEPLOY BLOCKED at step 6: smoke tests failed. The new image is live but not verified."
  exit 1
fi
echo "DEPLOY GREEN — $REPO_NAME:$TAG live at $SERVICE_URL/demo"
echo
echo "Note: the smoke search above created a scenario on great-goat, because a POST is"
echo "an action. Remove it if you are restoring the cloud baseline before a recording."
