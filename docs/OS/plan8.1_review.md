Plan 8.1 is the right next step. It incorporates the corrections we just agreed on and, importantly, keeps the reconciliation record separate from implementation. 

I would approve it with **one substantive change** before execution:

### Change Part 6

Do **not** instruct the fresh reviewer:

> “Do not re-litigate F-02/F-04/F-05/F-07.” 

That creates a dangerous review blind spot. The reviewer should be given the reconciliation evidence and told that those findings were adjudicated, but remain free to challenge them if the current repository contradicts the recorded evidence.

Use:

> **“Plan 8 contains raw evidence adjudicating F-02/F-04/F-05/F-07 as closed. Treat those dispositions as established context, but verify any one of them if your inspection finds evidence that materially contradicts the recorded repository state. Do not spend review effort re-litigating them without such contradictory evidence. Focus primarily on new or previously undetected findings.”**

That preserves the benefit of the reconciliation without turning it into an untouchable assumption.

### The F-01 design is good

The `MCP_TRANSPORT` approach is better than file-descriptor probing. Plan 8.1 explicitly rejects pipe/socket probing and instead makes the supported transport an explicit runtime invariant, while retaining stdio as the only implemented transport. 

One minor refinement: the environment variable is technically **configuration validation**, not a security boundary by itself. The actual security property remains:

> No network transport implementation exists, and unsupported transport selections fail closed.

That is the property the fresh reviewer should test.

### Part 1 is appropriately concrete

The raw excerpts for both handlers now show the critical ordering directly rather than merely asserting it. 

That is exactly the missing evidence standard we identified.

### F-06 disposition is correctly calibrated

Keeping it **OPEN / LOW / accepted / non-blocking** is the right choice.  It shouldn't contaminate the architectural review.

### Verification is adequate

The sequence covers build, vet, tests, race-sensitive packages, Taskfile, I-7, and MCP verification.  The added acceptance checks for both `MCP_TRANSPORT=sse` rejection and normal stdio operation are especially useful because they test the new invariant rather than merely compiling it. 

## Verdict

**APPROVED — with the Part 6 wording change above.**

Then the sequence should be:

**Plan 8.1 implementation → full verification → fresh adversarial review → final GREEN/NO-GO.**

I would not make any further architectural changes before that review. The current plan has reached the right point of diminishing returns: small F-01 hardening, explicit evidence closure, and then an independent attack on HEAD.


## claude

The raw-evidence capture for F-02 (exact line numbers, `GetSnapshot` at 136/182 before `RetireDebt`/`Promote` at 142/188) is exactly what I asked for last turn, and it closes that question definitively — no further verification needed there. The wording correction in Part 4 is the right instinct too: "no current P0/P1 defect has been reproduced" is honestly a different and more defensible claim than "the system has no P0/P1 defects," and it's good that this plan caught the overclaim in its own predecessor rather than leaving it.

But I don't think the F-01 hardening actually closes F-01, and I want to walk through why carefully, because on the surface it looks like exactly the right kind of fix.

**The `MCP_TRANSPORT` check doesn't intercept the attack F-01 describes.** F-01's scenario is `ssh -R`, container port mapping, VS Code port forwarding — all of these are mechanisms that bridge a process's *existing* stdin/stdout to a remote network endpoint from *outside* the process, without the process itself ever being told to do anything different. The binary keeps calling `server.Run(ctx, &mcp.StdioTransport{})` exactly as before; it has no way of knowing its file descriptors are now fed by a tunnel rather than a local parent process. The new check only fires if someone sets `MCP_TRANSPORT=sse` — but per this same document, there's no SSE-transport code path wired up anywhere in the binary regardless, so that configuration value couldn't have done anything even before this change. The check is real, fail-closed, and well-implemented — it's just guarding a door that was never the one the attack walks through.

What this *does* do is legitimate future-proofing: if someone later adds real SSE/HTTP transport support (the SDK has it, per this document), this check ensures it can't be silently enabled without a deliberate override. That's worth keeping. But it's a different thing from what F-01 asked for, and calling it "F-01 Hardening" risks checking the box in tracking without closing the actual gap — which matters because the honest alternative conclusion might be that there isn't a cheap in-process fix for arbitrary external stdio-tunneling at all, and the right disposition is an operational one (a deployment-guide line: "never wrap this process's stdin/stdout in a network bridge; if remote access is needed, do X instead"), not a code check that creates the appearance of having addressed it. I'd rather see that stated plainly than have this pass as closed.

One smaller thing on Part 6: telling the fresh reviewer "do not re-litigate F-02/F-04/F-05/F-07" risks discouraging them from noticing something *else* wrong in that same code, distinct from the original claims. I'd rephrase it as "these specific claims are considered resolved with the evidence in Plan 8 — verify independently if you'd like, but don't need to re-derive them from scratch; anything else you find in that code is fair game." Same effect (don't waste the reviewer's time re-proving settled facts), without foreclosing scrutiny of the surrounding code for unrelated issues — this is the same anti-anchoring discipline that made the SDAR review's rejected-leads table credible rather than performative.

Net: F-02/F-04/F-05/F-07 closures are solid, ship them as-is. F-06's disposition is honest and fine. F-01 needs either a real fix or an honest downgrade to "accepted architectural limitation, mitigated operationally, not by code" — the current framing overstates what the `MCP_TRANSPORT` check accomplishes.


## final

Agreed. That is the correct distinction.

The `MCP_TRANSPORT` change should **not be represented as closing F-01**. Plan 8.1 itself establishes that the process remains hard-coded to `StdioTransport`; the proposed check only rejects a configuration value for transports that are not actually implemented. 

So the clean disposition is:

### F-01

**Status: ACCEPTED ARCHITECTURAL LIMITATION / OPERATIONALLY MITIGATED / NOT CLOSED BY CODE**

The actual limitation is:

> `actor_id` is attribution, not authentication, because MCP is intentionally a trusted local stdio surface.

The process cannot reliably distinguish ordinary stdin/stdout from stdin/stdout that an external mechanism has tunneled over SSH, a container runtime, or an IDE. Therefore an in-process “stdio is local” check would be fundamentally unreliable.

The `MCP_TRANSPORT` guard is still worth implementing, but its correct classification is:

> **Future-transport hardening**, not F-01 remediation.

That is a useful defense against a future developer adding SSE/HTTP support and accidentally making it selectable, but it does not address arbitrary external stdio tunneling. The plan currently describes exactly the distinction that should be made. 

I would change Part 2 to say explicitly:

```text
F-01 is not closed by an in-process security check.

The threat is external stdio bridging. The MCP process cannot reliably
detect whether its stdio file descriptors are connected through SSH
reverse forwarding, container forwarding, IDE forwarding, or an ordinary
local parent process.

Therefore the security boundary is operational/deployment-level:

- MCP is stdio-only.
- Do not expose or bridge its stdio over an untrusted network boundary.
- Remote access must use an authenticated/authorized deployment boundary
  rather than treating actor_id as authentication.
- The binary itself provides no remote MCP transport.

Separately, MCP_TRANSPORT is retained as future-transport hardening:
unsupported transport values fail closed, preventing accidental activation
if additional transport implementations are introduced later.
```

Then the deployment documentation should carry the actual mitigation:

> **MCP security assumption:** `cmd/solvent-mcp` is a trusted local administrative surface. Do not expose its stdio transport through SSH forwarding, container port/stdio bridging, IDE remote forwarding, or another untrusted network boundary. `actor_id` provides attribution only and is not an authentication mechanism.

That is much more intellectually honest than claiming a code-level fix exists where it does not.

### Part 6

I agree with your wording. I would use essentially this:

> **F-02/F-04/F-05/F-07 are considered resolved based on the raw repository evidence captured in Plan 8. Reviewers need not independently re-derive those specific historical claims from scratch, but may verify them if convenient. Any new or unrelated issue discovered in the same code remains fully in scope.**

That preserves the efficiency benefit without creating an anchoring prohibition.

### Resulting status before fresh review

| Finding         | Disposition                                                    |
| --------------- | -------------------------------------------------------------- |
| F-02            | **CLOSED** — raw source/history evidence                       |
| F-04            | **CLOSED** — current I-7 gate verified                         |
| F-05            | **CLOSED** — refusal-path audit verified                       |
| F-07            | **CLOSED** — Taskfile parsing verified                         |
| F-06            | **OPEN / LOW / accepted**                                      |
| F-01            | **ACCEPTED ARCHITECTURAL LIMITATION / operational mitigation** |
| `MCP_TRANSPORT` | **Future-transport hardening, not F-01 closure**               |

That is the stronger position. It avoids both failure modes: pretending F-01 is solved when it isn't, and rejecting a useful future-proofing check merely because it doesn't solve the original threat.

The fresh adversarial review should therefore enter with **F-01 explicitly disclosed as an accepted trust-boundary assumption**, rather than as a supposedly closed vulnerability.


