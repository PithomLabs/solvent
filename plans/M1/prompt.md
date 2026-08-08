You are implementing a frozen architecture.

M0 is complete and approved.

Before writing any code, produce an implementation plan for M1 only.

Inputs:

Under /home/chaschel/Desktop/cockroach/hackathon/go
- IMPLEMENTATION_CONTRACT.md

Under /home/chaschel/Desktop/cockroach/hackathon/go/docs
- M0_REPORT.md
- M0_TRANSCRIPT.md

Consult CockroachDB documentation via MCP where relevant.

Do NOT write code.

Your implementation plan must contain:

# 1. Objective

State exactly what M1 accomplishes.

# 2. Scope

List every artifact M1 will implement.

List every artifact explicitly out of scope.

# 3. API Surface

List the kernel APIs to be implemented in M1.

Include signatures and behavioral contracts only.

No implementation.

# 4. CockroachDB considerations

Identify any database behaviors that M1 depends upon.

Verify them against CockroachDB documentation where possible.

If documentation is insufficient,

mark the item as:

- Verified
- Assumption
- Risk
- Blocker
- Out of scope

# 5. Implementation order

Break M1 into the smallest compilable steps.

Each step should leave the repository in a buildable state.

# 6. Acceptance criteria

Provide measurable acceptance criteria for each step.

# 7. Risks

List only risks introduced by M1.

Do not repeat already retired risks from M0.

# 8. Escalations

If M1 conflicts with:

- IMPLEMENTATION_CONTRACT.md
- M0 findings
- CockroachDB behavior

STOP.

Describe the conflict.

Do not redesign.

Wait for architectural review.

Constraints:

- Do not begin M2.
- Do not implement concurrency.
- Do not implement Bedrock.
- Do not implement Lambda.
- Do not implement graph UI.
- Do not implement agents beyond what M1 explicitly requires.
- Do not solve the embedding (R2); it remains deferred until M4 unless M1 cannot proceed without it.

Output only the implementation plan.
