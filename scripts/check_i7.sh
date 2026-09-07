#!/usr/bin/env bash
# I-7 gate: "Every kernel write goes through crdb.ExecuteTx. No raw db.Exec/db.Query
# writes." Contract §7 M1 makes this a grep criterion, so it gets a real grep.
#
# Also enforces D-033: no CREATE TEMP TABLE survives anywhere in the kernel.
#
# Writes docs/M1_I7.md and exits non-zero on any violation.
set -euo pipefail

cd "$(dirname "$0")/.."

PKG="${I7_PKG:-kernel}"
OUT="${I7_OUT:-docs/M1_I7.md}"
EXPECT_TX=21  # kernel.go: EnterBelief, AddEvidence, RetireDebt, Promote, IntentOnPromoted,
              #   RetractCascade, EnsureBelief (7)
              # authority.go: CreatePrincipal, RevokePrincipal, CreateTarget, AttachJustification,
              #   RequestAuthorization, Approve, Authorize, RevokeTarget, Discharge,
              #   ClaimIntent, CompleteIntent, RollbackClaim, ReconcileIntent, FailIntent (14)

fail() {
  echo "I-7 FAIL"
  echo "  criterion: $1"
  echo "  observed:  $2"
  echo "  expected:  $3"
  exit 1
}

# --- 1. Raw writes on the pool are forbidden outright -------------------------
raw_writes=$(grep -rnE 's\.db\.(Exec|Query)(Context)?\(' --include='*.go' --exclude='*_test.go' "$PKG" || true)
if [ -n "$raw_writes" ]; then
  echo "$raw_writes"
  fail "no raw s.db.Exec/s.db.Query calls in $PKG" \
       "the call site(s) listed above" \
       "zero matches"
fi

# --- 2. Pool-level reads are permitted only in named read-only helpers --------
#     I-7 governs writes; these functions perform none.
#     Current permitted sites:
#       AuditLiveOnNonPromoted  (kernel.go)    — I-5 audit query
#       targetState             (authority.go)  — lifecycle state derivation
pool_reads=$(grep -rnE 's\.db\.QueryRowContext\(' --include='*.go' --exclude='*_test.go' "$PKG" || true)
pool_count=$(echo "$pool_reads" | grep -c . || true)
while IFS= read -r line; do
  fn=$(echo "$line" | awk -F: '{print $1}')
  ln=$(echo "$line" | awk -F: '{print $2}')
  # Extract the function name containing this call
  func_name=$(awk -v target="$ln" '
    /^func \(s \*Store\) / { fn=$0; sub(/^func \(s \*Store\) /,"",fn); sub(/\(.*/,"",fn) }
    NR <= target { last_fn=fn }
    END { print last_fn }
  ' "$fn")
  case "$func_name" in
    AuditLiveOnNonPromoted|targetState) ;;
    *) fail "pool-level read in permitted function only" \
          "s.db.QueryRowContext in $func_name ($fn:$ln)" \
          "AuditLiveOnNonPromoted or targetState" ;;
  esac
done <<< "$pool_reads"
if ! grep -A3 'func (s \*Store) AuditLiveOnNonPromoted' "$PKG/kernel.go" | grep -q 's\.db\.QueryRowContext'; then
  fail "the single pool-level read belongs to AuditLiveOnNonPromoted" \
       "it is somewhere else" \
       "inside AuditLiveOnNonPromoted"
fi

# --- 3. Every write path is wrapped in crdb.ExecuteTx --------------------------
tx_count=$(grep -rcE 'crdb\.ExecuteTx\(' --include='*.go' --exclude='*_test.go' "$PKG" | awk -F: '{s+=$2} END {print s+0}')
if [ "$tx_count" -ne "$EXPECT_TX" ]; then
  fail "every §4 write function routes through crdb.ExecuteTx" \
       "$tx_count crdb.ExecuteTx call(s)" \
       "$EXPECT_TX"
fi

# --- 4. D-033: the temp-table approach is retired ------------------------------
temp_tables=$(grep -rniE 'CREATE[[:space:]]+TEMP' --include='*.go' --exclude='*_test.go' "$PKG" || true)
if [ -n "$temp_tables" ]; then
  echo "$temp_tables"
  fail "no CREATE TEMP TABLE in the kernel (D-033)" \
       "the occurrence(s) above" \
       "zero matches"
fi

# --- report -------------------------------------------------------------------
mkdir -p "$(dirname "$OUT")"
{
  echo "# M1 I-7 Report"
  echo
  echo 'Invariant I-7: *"Every kernel write goes through `crdb.ExecuteTx`. No raw'
  echo '`db.Exec`/`db.Query` writes."* Contract §7 M1 states this as a grep criterion;'
  echo 'this report is that grep.'
  echo
  echo "## Verdict"
  echo
  echo "**PASS** — ${tx_count} \`crdb.ExecuteTx\` write sites, 0 raw writes, ${pool_count} permitted pool read(s)."
  echo
  echo "## Write sites"
  echo
  echo "Each statement below executes on a \`*sql.Tx\` supplied by \`crdb.ExecuteTx\`."
  echo "The type is the proof: a \`*sql.Tx\` cannot be obtained outside the closure."
  echo
  echo "| function | line | method | statement |"
  echo "|---|---|---|---|"
  awk '
    /^func \(s \*Store\) / { fn=$0; sub(/^func \(s \*Store\) /,"",fn); sub(/\(.*/,"",fn) }
    /tx\.(Exec|QueryRow)Context\(/ {
      method = /tx\.QueryRowContext\(/ ? "tx.QueryRowContext" : "tx.ExecContext"
      stmt = "?"
      if (match($0, /sql[A-Za-z]+/)) stmt = substr($0, RSTART, RLENGTH)
      printf "| `%s` | %d | `%s` | `%s` |\n", fn, NR, method, stmt
    }
  ' "$PKG/kernel.go"
  echo
  echo "## crdb.ExecuteTx call sites"
  echo
  echo "| function | line |"
  echo "|---|---|"
  awk '
    /^func \(s \*Store\) / { fn=$0; sub(/^func \(s \*Store\) /,"",fn); sub(/\(.*/,"",fn) }
    /crdb\.ExecuteTx\(/ { printf "| `%s` | %d |\n", fn, NR }
  ' "$PKG/kernel.go"
  echo
  echo "## Permitted exceptions"
  echo
  echo "\`AuditLiveOnNonPromoted\` and \`targetState\` read through the pool without a transaction."
  echo "I-7 governs writes; these functions perform none. All \`s.db.*\` calls in the package"
  echo "belong to one of these two read-only helpers."
  echo
  echo "## D-033"
  echo
  echo "\`CREATE TEMP TABLE\` occurrences in \`$PKG\`: **0**. The draft's temp-table cascade is"
  echo "retired; \`RetractCascade\` uses the \`WITH RECURSIVE\` form proven by M0 probe D4."
} > "$OUT"

echo "I-7 PASS: ${tx_count} ExecuteTx write sites, 0 raw writes, ${pool_count} permitted pool read(s)"
echo "  report: $OUT"
