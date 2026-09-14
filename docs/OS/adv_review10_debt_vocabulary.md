# Pre-Kernel-Freeze Adversarial Review: Debt Domain-Agnosticism

**Scope:** REVIEW ONLY. No code, schema, tests, or documentation modified.

**Objective:** Determine whether Solvent's debt vocabulary is genuinely domain-agnostic, whether the representation is resource-safe, and whether the kernel is ready to freeze with respect to the debt model.

---

## Execution plan

This is a research/review task with no implementation. The deliverable is a written adversarial review document.

### Step 1: Write the review document

Create `docs/OS/adv_review10.md` with the following structure, using the evidence gathered during planning exploration.

---

## Evidence gathered during planning

### 1. Actual debt representation

**Type:** `TEXT[]` (PostgreSQL/CockroachDB text array) column on the `belief` row.

**Schema** (`db/001_schema.sql:25-32`):
```sql
debt TEXT[] NOT NULL DEFAULT ARRAY[
  'needProvenanceCheck','needContradictionSweep','needBlastRadius',
  'needRollbackPlan','needVersionPin','needOperatorSignoff'],
```

**No separate debt table exists.** Debt items are array elements, not rows. The `debt_discharge` table is an audit record, not the debt representation.

**Lifecycle:**
1. `EnterBelief` accepts caller-supplied `initialDebt []string` (callers pass `kernel.FullDebt` for the deployment domain)
2. `EnsureBelief` (find-or-create) accepts caller-supplied `initialDebt`; INSERT uses caller value instead of DDL DEFAULT
3. `RetireDebt` / `Discharge` remove items via `array_remove` (subtractive only)
4. `Promote` is gated by `promoted_is_debt_free` CHECK: `array_length(debt,1) = 0`

**No path appends to the array after creation.** The representation is monotonically shrinking.

### 2. Canonical vocabulary definition

**Kernel** (`kernel/kernel.go:31-34`):
```go
var FullDebt = []string{
    "needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
    "needRollbackPlan", "needVersionPin", "needOperatorSignoff",
}
```

**Schema** (`db/001_schema.sql:25-27`): DDL DEFAULT matches Go variable element-for-element.

**Migration** (`db/004_debt_vocabulary.sql:47`): ALTER DEFAULT for warm-start clusters.

**Drift tests** (B-17, B-23 in `kernel/kernel_test.go`): Prove Go and SQL have not diverged.

### 3. Vocabulary inventory by layer

| Location | Layer | Constrains valid? | Notes |
|----------|-------|-------------------|-------|
| `kernel.FullDebt` | Kernel | **YES** — single source of truth | Go variable, `[]string` |
| `db/001_schema.sql` DEFAULT | Schema | **YES** — paired with Go | Must stay in sync |
| `db/004_debt_vocabulary.sql` ALTER | Migration | **YES** — warm-start path | Must stay in sync |
| `promoted_is_debt_free` CHECK | Schema | **YES** — enforces empty | Cardinality only, never element values |
| `cmd/solvent-mcp/main.go` enum | MCP | Documentation only | Generated from `kernel.FullDebt` |
| `cmd/solvent-mcp/tools.go` validation | MCP | **YES** — rejects unknown | `slices.Contains(kernel.FullDebt, item)` |
| `api/belief.go` validation | REST API | **No** — non-empty only | Any string accepted; `array_remove` is no-op for missing items |
| `docs/openapi/solvent.yaml` | API doc | No | `type: string`, no enum |
| `internal/belief/mapping.go` | App | No — routing only | Maps evidence → debt items |
| `internal/wizard/state.go` | UI | No — display only | `checkPrompts`, `retrievalChecks` |
| `scripts/mcp_verify.sh` | Script | **YES** — runtime check | Hardcodes 6-item vocabulary |
| `kernel/kernel_test.go` B-01/B-17/B-23 | Test | **YES** — drift detection | Catches divergence |

### 4. Hardcoding assessment

**Kernel semantic coupling:** None. The kernel's `RetireDebt` accepts any string and passes it to `array_remove`. It does NOT validate against `FullDebt`. The kernel does not know what the debt items mean — it only stores/manages an opaque `TEXT[]` and enforces that promotion requires an empty array.

**The vocabulary lives in three places that must stay in sync:**
1. `kernel.FullDebt` (Go variable)
2. `db/001_schema.sql` DEFAULT (DDL)
3. `db/004_debt_vocabulary.sql` ALTER DEFAULT (migration)

This is a maintenance coupling, not a semantic coupling. The kernel itself does not need to know the names.

**MCP handler validation** (`tools.go:131`): Rejects unknown debt items. This is a presentation-layer guard, not a kernel invariant. The kernel accepts any string.

**REST API validation** (`api/belief.go:116`): Only validates non-empty. Any string passes through to the kernel. This is weaker than MCP but not a security issue — `array_remove` on a nonexistent item is a silent no-op.

### 5. Resource-bound assessment

**Is the debt collection actually unbounded?**

**No.** New beliefs start with exactly 6 items (from `FullDebt` or the DDL DEFAULT). The only mutation is `array_remove`, which shrinks the array. No path appends items. The maximum cardinality is 6 at creation time and monotonically decreases.

**Is an explicit finite limit technically required?**

**No.** The representation is inherently bounded:
- Starting cardinality: 6 (hardcoded in `FullDebt` and DDL DEFAULT)
- Mutation: subtractive only (`array_remove`)
- No append path exists in kernel, API, MCP, or wizard
- The `TEXT[]` column has CockroachDB's ~16MB row limit, but 6 short strings (~120 bytes total) are nowhere near it

**Is there a concrete attack or resource problem?**

**No.** The array is tiny and monotonically shrinking. There is no transaction amplification, no unbounded serialization, no CPU cost proportional to debt count, and no path to create arbitrarily large debt collections.

**If a technical limit were hypothetically needed, where should it belong?**

Service/API boundary — not the kernel. But no limit is needed because the representation is inherently bounded.

### 6. Domain portability test

**Could finance, healthcare, security, deployment, and unrelated applications use the same Solvent kernel with different debt vocabularies without modifying kernel code?**

**Yes, with one constraint.** The kernel does not validate debt item names. `RetireDebt` accepts any string. `array_remove` works on any element. The `promoted_is_debt_free` CHECK only checks `array_length(debt,1) = 0`, never element values.

The constraint is: new beliefs are stamped with `FullDebt` (the 6 deployment-review items). A different domain would need to either:
1. Use `EnterBelief` and then retire the deployment items / add domain-specific items (awkward)
2. Use `EnsureBelief` which uses the DDL DEFAULT (same issue)
3. Create beliefs via direct SQL with a custom debt array (bypasses the kernel)

All callers now pass `kernel.FullDebt` explicitly. A different domain can pass its own vocabulary. The kernel mechanism AND initialization are both domain-agnostic.

This is now resolved: **the kernel mechanism AND initialization are both domain-agnostic.** `EnterBelief` and `EnsureBelief` accept `initialDebt []string` as a required parameter. The kernel no longer hardcodes any specific vocabulary.

### 7. Trust boundary for debt operations

| Operation | REST Auth | REST Authz | MCP Auth | Wizard Auth | DB Auth |
|-----------|-----------|------------|----------|-------------|---------|
| RetireDebt | YES | **BEST-EFFORT** — principal existence + revocation check (TOCTOU race documented) | NONE (local) | App-level checks (citation, artifact) | root |
| Discharge | YES | **YES** — `discharged_by` validated against authenticated principal; impersonation rejected with 403 | NONE (local) | N/A (wizard uses RetireDebt directly) | root |
| Promote | YES | **NONE** — any principal can promote if debt is empty | NONE (local) | NONE (deliberate — schema is the gate) | root |
| EnterBelief | YES | **NONE** — any principal can create any belief | NONE (pipeline only) | Seed only | root |

**RetireDebt access control:** The service layer verifies the authenticated principal exists in the `principal` table and is not revoked before calling the kernel. This is a best-effort liveness pre-check — the kernel owns its internal `crdb.ExecuteTx`, so this check cannot be made atomic with the debt mutation. The residual TOCTOU race is documented and accepted: revocation takes effect immediately for all new requests.

**Discharge access control:** The `discharged_by` field is validated against the authenticated principal. Caller-supplied impersonation is rejected with HTTP 403 `discharged_by_mismatch`. The effective `discharged_by` is derived from the API key, not the request body. This follows the existing `actor_id_mismatch` pattern from `handleAuthorizeAction`.

**Is this a kernel concern?** No. These are service/API-layer access controls. The kernel correctly delegates to the database for invariant enforcement. The access controls are a deployment/policy concern, not a kernel design defect.

### 8. Schema enforcement summary

| Constraint | Layer | What it enforces |
|------------|-------|-----------------|
| `promoted_is_debt_free` | DB CHECK | Cannot promote with non-empty debt |
| `gate` FK + `live_requires_promoted` | DB CHECK + FK | Cannot have live intent on non-promoted belief |
| `UNIQUE(belief_id, obligation_key, instrument_ref)` | DB UNIQUE | No duplicate discharge records |
| Scenario-scoping in kernel | Kernel | Every write scoped to scenario_id |
| `array_remove` (subtractive only) | Kernel+SQL | Debt can only shrink, never grow |

---

## Review document structure

The review document (`docs/OS/adv_review10.md`) will contain:

### Verdict

**GREEN — debt model ready for kernel-freeze decision**

- The kernel mechanism is domain-agnostic: `RetireDebt` accepts any string, `array_remove` works on any element, `promoted_is_debt_free` checks cardinality only.
- The default debt stamp in `EnterBelief`/`EnsureBelief` has been parameterized: callers now supply `initialDebt []string` explicitly. The kernel no longer hardcodes `FullDebt`.
- The representation is inherently bounded (6 items, subtractive only). No resource limit is required.
- No kernel change is required beyond the API correction already implemented.

### Required output sections

1. **Actual debt representation** — `TEXT[]` on `belief` row, subtractive lifecycle
2. **Vocabulary inventory** — every discovered location, classified by layer
3. **Hardcoding assessment** — kernel mechanism is generic; default stamp is domain-specific
4. **Resource-bound assessment** — inherently bounded, no limit needed
5. **Domain portability test** — kernel mechanism is portable; default stamp requires parameterization
6. **Trust boundary** — REST has auth but no authz for debt; MCP is trusted local; wizard has app-level checks
7. **Kernel-freeze recommendation** — GREEN WITH MINOR CLEANUP

### Minor cleanup items (non-blocking)

1. **REST API `RetireDebt` validation gap:** The REST handler only validates `debt_item` as non-empty, while MCP validates against `FullDebt`. Consider adding the same `slices.Contains` check to the REST handler for consistency. This is a service-layer fix, not a kernel change.

2. **`EnterBelief` default debt parameterization:** DONE — callers now supply `initialDebt []string` explicitly. `FullDebt` remains in the kernel as a deployment vocabulary convenience. The kernel mechanism is fully domain-agnostic.

3. **`Discharge` `discharged_by` not verified:** The REST handler accepts any `discharged_by` UUID without verifying it matches the authenticated principal. Consider validating this at the service layer. This is an authorization gap, not a kernel concern.

### What is NOT required

- No kernel growth gate trigger
- No new schema constraints
- No debt-count limit (the representation is inherently bounded)
- No workflow abstraction or policy engine
- No vocabulary hardcoding changes in the kernel
- No idempotency work

---

## Files to create

| File | Purpose |
|------|---------|
| `docs/OS/adv_review10.md` | The adversarial review document |

No code, schema, test, or configuration files are modified.
