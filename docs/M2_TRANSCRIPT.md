# M2 Behavioral Transcript

Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.
Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,
never from substring matching.

M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.

<!-- volatile:begin -->

| run fact | value |
|---|---|
| generated_at | 2026-09-05T03:47:17Z |
| host | linux |
| dsn | postgresql://root@localhost:26260/fable_kernel_test?sslmode=disable |

<!-- volatile:end -->

## Verdict

**GREEN** — 4/4 cases passed.

## Cases

| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |
|---|---|---|---|---|---|---|---|---|---|
| T-C10 | concurrency | PASS | Old two-step sequence creates stale-authority intent (negative control) | stale live intent exists after old two-step sequence | intent_count=1, err=<nil> | — | — | Old sequence: Authorize commits → RevokeTarget commits → IntentOnPromoted commits → stale intent persists | 145 |
| T-C9A | concurrency | PASS | FOR UPDATE lock serializes authorize-first against concurrent revoke | intent exists, revocation exists; Conn2 blocked on lock until Conn1 committed | intents=1, revocations=1 | — | — | FOR UPDATE lock on authority_target ensures authorize-first ordering is respected | 95 |
| T-C9A2 | concurrency | PASS | FOR UPDATE lock serializes revoke-first: authority denied after revocation | authority denied, no intent, revocation exists | authority_denied=true, intents=0, revocations=1 | — | — | FOR UPDATE lock on authority_target ensures revoke-first ordering is respected | 87 |
| T-C9B | concurrency | PASS | Concurrent AuthorizeAndCreateIntent + RevokeTarget: no stale intent | outcome A (authorize wins, intent valid) or B (revoke wins, denied); never C (stale intent) | authorize_allowed=false, authorize_err=<nil>, revoke_err=<nil>, intents=0, revocations=1, stale=false, outcome=authorization_denied | — | — | No committed live intent created on superseded authority | 94 |

## Receipts

_No case produced a raw receipt._

