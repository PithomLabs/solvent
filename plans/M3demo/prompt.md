M0, M1 and M2 are complete.

Do NOT design M3 from the perspective of engineering.

Design it from the perspective of winning the CockroachDB × AWS hackathon.

Assume the judges have exactly three minutes.

Current facts

- Database semantics proven.
- Kernel behavior proven.
- Three tables.
- Two agents.
- One evidence feed.
- One graph.

Question

What is the SMALLEST possible M3 that creates an unforgettable demonstration?

Requirements

1. The demo must revolve around one live concurrency race.

2. The race must visibly change agent behavior.

3. CockroachDB must visibly prevent corruption.

4. The graph must visibly change.

5. The audience must immediately understand why.

Deliverables

Produce only

M3_DEMO_PLAN.md

Include

- demo narrative
- race timeline
- actors
- expected visualization
- expected receipts
- exactly what the judges see
- exactly what the narrator says
- implementation boundary

Do NOT discuss implementation.

Do NOT discuss Go.

Do NOT discuss packages.

Do NOT discuss tests.

Optimize purely for maximum demo impact.

End by answering:

"If we built ONLY this M3, would it be enough to make the database the hero?"
