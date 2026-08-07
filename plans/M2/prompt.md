You are implementing a frozen architecture.

M0 and M1 are complete and approved.

Before writing any code, produce the implementation plan for M2 only.

Inputs

Under /home/chaschel/Desktop/cockroach/hackathon/go
- IMPLEMENTATION_CONTRACT.md

Under /home/chaschel/Desktop/cockroach/hackathon/go/docs
- M0_REPORT.md
- M0_PREPARE.md
- M0_I7.md
- M1_REPORT.md
- M1_PREPARE.md
- M1_I7.md

Consult CockroachDB documentation via MCP where relevant.

Do NOT write code.

--------------------------------------------------

Objective

M2 proves kernel behavior.

It is the first behavioral milestone.

M2 must demonstrate that the kernel enforces the architectural invariants through execution.

--------------------------------------------------

The plan must contain

# 1. Objective

Exactly what behavioral guarantees M2 proves.

# 2. Scope

Exactly what M2 implements.

Exactly what M2 does NOT implement.

# 3. Behavioral Test Matrix

List every invariant to be executed.

For each:

- purpose
- expected result
- evidence produced

Cover at minimum:

- EnterBelief
- AddEvidence
- RetireDebt
- Promote
- IntentOnPromoted
- RetractCascade
- AuditLiveOnNonPromoted

Also include execution tests for:

- debt removal
- promotion refusal (23514)
- FK refusal (23503)
- []string → STRING[] encoding
- recursive cascade correctness
- cross-scenario isolation
- retry classification
- RowsAffected semantics

# 4. CockroachDB dependencies

For every behavioral assumption classify as

- Verified
- Assumption
- Risk
- Blocker
- Out of scope

using CockroachDB documentation where applicable.

# 5. Implementation order

Break M2 into the smallest executable stages.

Each stage should leave the repository passing tests.

# 6. Acceptance criteria

Provide objective acceptance criteria.

No subjective wording.

# 7. Risks

Only new M2 risks.

Do not repeat retired risks.

# 8. Escalation

If execution behavior differs from

- IMPLEMENTATION_CONTRACT.md
- M0 findings
- M1 findings

STOP.

Do not redesign.

Report receipts.

--------------------------------------------------

Constraints

Do NOT begin M3.

Do NOT implement concurrency harness.

Do NOT implement demo UI.

Do NOT implement Lambda.

Do NOT implement Bedrock.

Do NOT implement graph visualization.

Do NOT solve embedding.

--------------------------------------------------

Output

Only M2_IMPLEMENTATION_PLAN.md
