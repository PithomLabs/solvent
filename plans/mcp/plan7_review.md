I reviewed Plan 7. **APPROVE WITH MINOR CORRECTIONS**. The scope is appropriately narrow and the fixes correspond directly to A-1 through A-4. 

### One correction I would make before implementation

**A-4's wording is slightly confusing.**

It says:

> `"I-7 6 adversarial injections" → "I-7 5 adversarial injections"` because the table has 5 malicious + 1 clean. 

But the actual Plan 7 test matrix now has **8 malicious cases + 1 clean case**:

* 2–7 = six malicious injections
* 8–9 = two missing-directory failures
* 1 = clean PASS 

So if `plan6_imp.md` is intended to describe the **full adversarial I-7 suite**, the correct wording should be:

> **“I-7 8 adversarial cases”**
> or, even better, **“I-7: 8 negative cases + 1 clean case.”**

That is clearer and avoids this counting problem recurring.

### A-3 is otherwise sound

The corrected regex now catches:

* `INSERT INTO`
* `UPDATE ... SET`
* `DELETE FROM`
* `CREATE TABLE/INDEX/DATABASE`
* `DROP TABLE/INDEX/DATABASE`

and `-i` closes the lowercase bypass. 

The important adversarial `QueryRowContext(... "update belief set ...")` case is explicitly included. 

I would **not** complicate this further with a SQL parser. For a hackathon boundary tripwire, this is appropriately mechanical.

### A-1/A-2 are exactly the right fixes

The authorize guard now mirrors the already-proven promotion/retirement ownership pattern, while leaving the database as the authority for the actual M2 gate. 

Likewise, falsify gets the same identity protection and the `retracted == 0 → NO-OP` correction, which prevents a misleading success response. 

### One small verification correction

The verification table says:

> `task test` → 107 PASS, 0 FAIL, 0 SKIP. 

I'd retain the previously established self-contained invocation:

```text
unset FABLE_TEST_DSN && task test
```

That matters because we specifically established that the test suite must not silently depend on the MCP/demo container.

### Verdict

**Approve after these two tiny edits:**

1. Change A-4 to **“8 adversarial cases + 1 clean case”** if that is what the report is meant to count.
2. Change verification to:
   `unset FABLE_TEST_DSN && task test`.

Then **send Plan 7 directly to the coding agent**. No more planning review is warranted. The remaining work is mechanical, and the frozen-core boundary is explicitly preserved. 
