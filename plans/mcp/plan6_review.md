I would **not send Plan 6 to the coding agent yet**. There is one substantive flaw in the proposed I-7 gate that should be corrected first.

### The problem: F-1's Stage 2 does not actually catch the claimed `UPDATE`/`DELETE` cases

The proposed regex is:

```text
(INSERT|UPDATE|DELETE|CREATE|DROP)[[:space:]]+(INTO|TABLE|FROM)
```

But ordinary SQL is:

```sql
UPDATE belief SET ...
DELETE FROM belief ...
INSERT INTO belief ...
CREATE TABLE ...
DROP TABLE ...
```

So:

* `INSERT INTO` → caught
* `DELETE FROM` → caught
* `CREATE TABLE` → caught
* `DROP TABLE` → caught
* **`UPDATE belief SET` → NOT caught**

More importantly, Plan 6 explicitly requires this adversarial test:

```text
db.QueryRowContext(ctx, "UPDATE ... RETURNING id")
```

Yet Stage 1 doesn't detect `QueryRowContext`, and Stage 2 expects `UPDATE` to be followed by `INTO|TABLE|FROM`, so **the exact test that Plan 6 says must FAIL can PASS**. That's a real internal contradiction. 

The const-hoisted test is similarly misleading: it will be caught because the call is `ExecContext`, but that's Stage 1—not proof that Stage 2 catches const-hoisted SQL. 

### How I'd fix the plan

Keep the two-stage design, but make Stage 2 actually recognize SQL write statements rather than trying to encode their syntax incorrectly.

For example, conceptually:

```sh
if grep -rnE --include='*.go' \
  '\b(INSERT|UPDATE|DELETE|CREATE|DROP)\b' \
  cmd/solvent-mcp internal/view; then
    echo "FAIL: write SQL text in MCP/view"
    exit 1
fi
```

But there's an important caveat: **this will also match comments and prose**, so the coding agent should implement a reasonable Go-source-aware filtering approach rather than blindly copying that regex.

Even better, the gate's primary invariant should be:

> **MCP/view must not directly invoke database write-capable APIs.**

So Stage 1 should be the strong guard:

```text
Exec
ExecContext
Query
QueryContext
QueryRow
QueryRowContext
Prepare
PrepareContext
Begin
BeginTx
```

and then Stage 2 should be a supplementary SQL-text tripwire.

I would explicitly include `Query`, `QueryContext`, `QueryRow`, and `QueryRowContext` in the prohibited direct-DB API list. A write can be expressed through `UPDATE ... RETURNING`, and relying exclusively on SQL-text matching is fragile.

### One other subtlety

Plan 6 says:

> "MCP/view may read. MCP/view may call the Solvent kernel. MCP/view may NOT directly write or open write transactions." 

But the proposed Stage 1 rejects **all** `Prepare`, `Begin`, and `BeginTx`, including potentially read-only transaction usage. That's probably acceptable for this repository because the MCP/view layer shouldn't need those APIs, but the wording should be:

> **For this implementation, MCP/view must not directly use database transaction/write APIs; all DB access is through the existing read-only view or Solvent kernel/pipeline.**

That makes the rule deliberately restrictive rather than pretending `Begin` is intrinsically a write.

## Verdict

**Plan 6: APPROVE WITH ONE CORRECTION — do not implement yet.**

The F-2/F-3 validation changes are good and the verification matrix is good.  The problem is specifically the F-1 gate specification.

I would amend F-1 so that:

1. `Query`, `QueryContext`, `QueryRow`, and `QueryRowContext` are included in the direct-DB API tripwire.
2. The SQL-text test actually recognizes ordinary `UPDATE ... SET` syntax.
3. The mandatory `QueryRowContext("UPDATE ... RETURNING")` test is guaranteed to fail.
4. The coding agent must demonstrate **clean → PASS and every malicious case → FAIL**, rather than merely asserting that the eight tests were run.

After that, **no more Claude plan review is necessary**. This is a small correction to the final hardening plan, not an architectural issue.
