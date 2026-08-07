# M1 PREPARE Transcript

Every statement in `kernel.SQLCatalog()`, prepared against a schema-applied
database and immediately deallocated. **Nothing was executed; no row was written.**

This is a SQL compile check, not an invariant check: it proves relations, columns,
placeholder types and statement structure. It proves no semantics — 23514 and 23503
arise only at execution, which is M2's gate.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-08-07T00:13:09Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 8/8 statements prepared, none skipped.

## Statements

| statement | prepared | sqlstate | message | elapsed_ms |
|---|---|---|---|---|
| `verify_add_evidence` | PASS | — | — | 1 |
| `verify_audit_live_on_nonpromoted` | PASS | — | — | 0 |
| `verify_enter_belief` | PASS | — | — | 0 |
| `verify_intent_on_promoted` | PASS | — | — | 0 |
| `verify_promote` | PASS | — | — | 0 |
| `verify_retire_debt` | PASS | — | — | 0 |
| `verify_retract_cascade_cancel` | PASS | — | — | 1 |
| `verify_retract_cascade_retract` | PASS | — | — | 1 |

## Prepared SQL

### add_evidence

```sql
INSERT INTO evidence
  (scenario_id, belief_id, provenance_class, source_url, content_sha256)
VALUES ($1::UUID, $2::UUID, $3::STRING, $4::STRING, $5::STRING)
```

### audit_live_on_nonpromoted

```sql
SELECT count(*) FROM action_intent a
JOIN belief b ON b.id = a.belief_id
WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = $1::UUID
```

### enter_belief

```sql
INSERT INTO belief (scenario_id, claim, claim_type, debt)
VALUES ($1::UUID, $2::STRING, $3::STRING, $4::STRING[])
RETURNING id
```

### intent_on_promoted

```sql
INSERT INTO action_intent (scenario_id, belief_id, action)
VALUES ($1::UUID, $2::UUID, $3::STRING)
```

### promote

```sql
UPDATE belief SET status = 'promoted'
WHERE id = $1::UUID
```

### retire_debt

```sql
UPDATE belief SET debt = array_remove(debt, $2::STRING)
WHERE id = $1::UUID
```

### retract_cascade_cancel

```sql
WITH RECURSIVE d(id) AS (
    SELECT b.id FROM belief b
     WHERE b.id = $1::UUID AND b.scenario_id = $2::UUID
  UNION
    SELECT e.child_id
      FROM belief_edge e
      JOIN d         ON e.parent_id = d.id
      JOIN belief cb ON cb.id = e.child_id AND cb.scenario_id = $2::UUID
)
UPDATE action_intent SET state = 'cancelled'
 WHERE state = 'live'
   AND scenario_id = $2::UUID
   AND belief_id IN (SELECT id FROM d)
```

### retract_cascade_retract

```sql
WITH RECURSIVE d(id) AS (
    SELECT b.id FROM belief b
     WHERE b.id = $1::UUID AND b.scenario_id = $2::UUID
  UNION
    SELECT e.child_id
      FROM belief_edge e
      JOIN d         ON e.parent_id = d.id
      JOIN belief cb ON cb.id = e.child_id AND cb.scenario_id = $2::UUID
)
UPDATE belief SET status = 'retracted'
 WHERE status = 'promoted'
   AND scenario_id = $2::UUID
   AND id IN (SELECT id FROM d)
```

