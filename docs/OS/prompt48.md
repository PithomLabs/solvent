STOP. DO NOT IMPLEMENT ANY FIXES YET.

We have conflicting independent review claims that must first be resolved
against the ACTUAL CURRENT REPOSITORY.

This is an evidence-reconciliation task, not a remediation task.

==================================================
1. RESOLVE CI-9 PROVIDEROUTCOME CONTRADICTION
==================================================

Do NOT rely on:
- adv_review_phase4d.md
- adv_review5.md
- phase4d_plan4.md
- previous summaries

Inspect the actual source tree.

Run:

    grep -Rnw --include='*.go' 'type ProviderOutcome' .
    grep -Rnw --include='*.go' 'type ProviderError' .
    grep -Rnw --include='*.go' 'ProviderOutcome' adapter service kernel
    grep -Rnw --include='*.go' 'ProviderError' adapter service kernel

Also inspect the actual imports and declarations.

Determine definitively:

1. Which package declares ProviderOutcome?
2. Which package declares ProviderError?
3. Which package performs GitHub-specific classification?
4. Which package consumes the generic classification?
5. Does service/authority import adapter/github directly?
6. Does service/authority inspect GitHub HTTP status codes?
7. Does kernel import either GitHub or provider-classification code?

Report exact file paths and symbols.

Do not call this resolved from documentation.

==================================================
2. RESOLVE R-3 / F-02 REGRESSION
==================================================

The historical evidence conflicts:

Earlier MCP review:
    R-3 found cross-scenario mutation-before-validation.

Later plan5 review:
    R-3 explicitly reported CLOSED and verified with SQL before/after.

Latest cumulative review:
    F-02 claims the same defect is present again.

Determine whether the current repository has actually regressed.

Inspect:

    cmd/solvent-mcp/tools.go

Specifically:

    handleSolventRetireDebt
    handleSolventPromote

Determine the exact ordering.

Required safe ordering:

    validate belief belongs to requested scenario
        ↓
    only then call kernel mutation

Unsafe ordering:

    kernel mutation
        ↓
    scenario validation

Do NOT infer from comments.

==================================================
3. BUILD A REPRODUCTION TEST
==================================================

For current code, create NO changes.

Instead, use the existing test infrastructure or an isolated temporary
reproduction outside the repository to establish the current behavior.

For:

    scenario A
    belief belonging to scenario B

test:

    solvent_retire_debt
    solvent_promote

Measure authoritative state BEFORE and AFTER.

The required question is binary:

    Does cross-scenario request cause ZERO state mutation?

Expected secure behavior:

    error
    +
    zero mutation

If mutation occurs, F-02 is genuinely OPEN.

If zero mutation occurs, F-02 is CLOSED and the cumulative review is stale or
otherwise incorrect.

Do not accept logging-only evidence.

==================================================
4. INVESTIGATE HOW REGRESSION COULD HAVE OCCURRED
==================================================

Inspect git history for the relevant files:

    git log --oneline --follow -- cmd/solvent-mcp/tools.go
    git log -p -- cmd/solvent-mcp/tools.go

Identify:

- commit where R-3 was fixed
- later commits touching the same handlers
- whether the fix was preserved
- whether another refactor reintroduced the old ordering

If the fix was lost, identify the commit that caused the regression.

If the fix is still present, document why the latest review's claim is false.

==================================================
5. RECONCILE THE EARLIER MCP REVIEW HISTORY
==================================================

Locate the actual earlier MCP review and later plan5 review.

Known evidence includes:

- R-3 was originally reported as cross-scenario mutation.
- plan5_review.md later says R-3 CLOSED and reports SQL before/after proving
  zero mutation.

Verify the actual repository revision associated with those claims.

Build a timeline:

    original finding
        ↓
    fix
        ↓
    independent verification
        ↓
    later Phase 4D changes
        ↓
    later Phase 4E changes
        ↓
    current state

Identify whether there was an actual regression.

Do not assume later review > earlier review merely because it is newer.

==================================================
6. VERIFY TASKFILE CONTRADICTION
==================================================

The latest cumulative review also claims:
- Taskfile syntax failure
- inert I-7 gate

But later Phase 4E verification reportedly showed:
    task test
    check_i7
    mcp_verify

passing.

Do not accept either report.

Run directly:

    task --list
    task test
    bash scripts/check_i7.sh
    bash scripts/mcp_verify.sh

Inspect the actual Taskfile.

Determine:
- does Taskfile parse?
- does I-7 actually execute?
- can I-7 detect an injected raw write?
- does it fail on missing directories?

Use a temporary copy/scratch directory for any injection test.
Do not modify the repository.

==================================================
7. DO NOT FIX ANYTHING
==================================================

This entire pass is evidence reconciliation only.

Do NOT:
- modify code
- modify tests
- modify Taskfile
- modify docs
- add configuration flags
- remove workflow_token
- change MCP transport
- fix R-3
- fix I-7

The goal is to establish which findings are REAL in the CURRENT repository.

==================================================
FINAL REPORT
==================================================

Return:

A. CI-9 actual source declaration

    ProviderOutcome:
    ProviderError:
    GitHub classifier:
    Service consumer:
    Kernel dependency:

B. F-02 current status

    OPEN / CLOSED

with actual reproduction evidence.

C. R-3 historical timeline

    finding → fix → verification → later changes → current state

D. Taskfile/I-7 current status

    PASS / FAIL

with actual command evidence.

E. Review reconciliation table:

    Finding | Earlier Evidence | Later Evidence | Current Reality

Include at minimum:
    CI-9
    R-3/F-02
    R-1/F-07
    R-2/F-04
    R-6/F-05
    R-7/F-06

F. Final conclusion:

    Which findings are genuinely actionable NOW?

STOP.

Do not implement remediation until this evidence reconciliation is complete.