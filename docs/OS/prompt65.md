I would **not approve the corrective plan exactly as shown**. F-1 and F-2 are correct; F-7 is worthwhile. But the proposed addition of **005–008** to `resetAndSeed` needs one verification step before execution.

### F-1: definitely fix

`resetAndSeed` must apply `010_debt_opaque.sql` after `004`. The adversarial review established that this is a real inconsistency and can restore the retired default after a reset. 

### F-2: definitely fix

Replace the wizard's remaining debt literals with the named `belief.Need*` constants. That completes the single-source-of-truth design. 

### F-7: fix

The stale comments should be corrected because they now falsely say the six debts came from the DDL default. The six debts now come from `belief.WizardDebt()`. 

### 005–008: do not add automatically

The screenshot's plan says:

> add `debtOpaquePath` **and the other missing schemas 005–008** to `resetAndSeed`

The adversarial finding only establishes that **010 is missing**. Adding 005–008 is an additional behavioral change.

Before adding them, the agent should inspect what `resetAndSeed` actually needs. The fact that the normal `main()` paths apply 005–008 does **not by itself prove** that `resetAndSeed` should. This is precisely where "consistency" can turn into unnecessary scope expansion.

I would tell the coding agent:

```text
Proceed with the corrective pass, but resolve F-1 narrowly.

1. Add db/010_debt_opaque.sql to resetAndSeed after 004.

2. Before adding 005-008, inspect resetAndSeed's actual purpose and every object
   it creates/uses. Determine whether 005-008 are required for that reset path
   to represent the same supported database state as the normal cold-start path.

3. If 005-008 are genuinely required dependencies, add them in migration order
   before 010.

4. If resetAndSeed intentionally uses a reduced schema and does not require
   005-008, do NOT add them. Document that distinction and add only 010.

5. Fix F-2: replace all wizard debt string literals in internal/wizard/state.go
   and related wizard-domain locations with the named belief.Need* constants.

6. Fix F-7: update stale comments in:
      kernel/kernel.go
      internal/wizard/seed.go
      service/ledger/ledger_test.go
      kernel/example_test.go
      IMPLEMENTATION_CONTRACT.md
      internal/wizard/discharge.go
   so they describe caller-supplied/domain-owned debt rather than a DDL
   six-item default.

7. Do NOT:
   - modify or delete historical migrations 001 or 004;
   - rename FullDebtNames();
   - refactor internal/belief.Process;
   - perform unrelated cleanup.

8. Run:
      targeted affected tests
      task test
      ./scripts/mcp_verify.sh
      git diff --check

9. Repeat the adversarial grep/search for:
      FullDebt
      old six-item literals
      stale "DDL default" references

10. Do not commit or tag.

Report:
- whether 005-008 were actually required by resetAndSeed;
- files changed;
- tests/results;
- remaining findings;
- git status.
```

The key point is: **fix the real F-1 defect without turning it into a broader schema-reset refactor**. Once that passes, the remaining work is small enough that you should be at the final human diff review before your commit.
