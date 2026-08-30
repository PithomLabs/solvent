Choose Option 1.

The actual repository's internal/view/view.go legitimately uses
db.QueryRowContext for four SELECT operations. Therefore the I-7 gate MUST
NOT reject Query/QueryContext/QueryRow/QueryRowContext indiscriminately.

Use this boundary:

Stage 1 — reject direct mutation/transaction entry points:

    Exec
    ExecContext
    Prepare
    PrepareContext
    Begin
    BeginTx

Stage 2 — supplementary SQL-text tripwire:

    INSERT
    UPDATE
    DELETE
    CREATE
    DROP

The clean repository MUST PASS.

Do NOT reject QueryRowContext merely because it is a database API.

The existing QueryRowContext SELECT calls in internal/view are legitimate
and must remain untouched.

The purpose of Stage 2 is specifically to catch write SQL passed through
otherwise read-capable query APIs, e.g.:

    db.QueryRowContext(ctx,
        "UPDATE belief SET ... RETURNING id")

and:

    db.QueryContext(ctx,
        "DELETE FROM belief WHERE ...")

The Stage 2 matcher must correctly recognize ordinary SQL syntax,
including:

    UPDATE ... SET
    DELETE FROM ...
    INSERT INTO ...
    CREATE TABLE ...
    DROP TABLE ...

Do NOT use the previous flawed pattern requiring UPDATE to be followed by
INTO/TABLE/FROM.

Keep the gate conservative but simple. It is a mechanical boundary guard,
not a full static SQL analyzer.

Required tests remain:

    clean repository → PASS
    ExecContext write → FAIL
    tx.ExecContext write → FAIL
    gofmt-wrapped ExecContext → FAIL
    QueryRowContext + UPDATE RETURNING → FAIL
    const-hoisted UPDATE + ExecContext → FAIL
    missing cmd/solvent-mcp → FAIL
    missing internal/view → FAIL

Also explicitly verify that the four legitimate QueryRowContext SELECTs
in internal/view continue to pass the clean gate.

Do not modify internal/view or the frozen core to satisfy the gate.