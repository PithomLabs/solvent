# DeepWiki Q&A with Code Context for Repository: TusharTechs/aegisflow
## Q1
how can Pithom Labs Solvent take advantage of the insights below with respect to workflow and architecture of aegisflow? # Competitive Analysis: Solvent vs AegisFlow

**Date:** 2026-09-04  
**Purpose:** Identify gaps and limitations of Solvent relative to AegisFlow to determine what Solvent needs to become a competitive, market-ready product.

---

## Executive Summary

**Solvent** is a transactional belief ledger for autonomous agents, built on CockroachDB with database-enforced invariants, formal verification in Lean 4, and an MCP server with 16 tools. Its core thesis is "retrieval is not authority" — the database, not the LLM, determines whether an action is allowed.

**AegisFlow** is a full-stack TypeScript/Next.js application for AI-powered incident response in critical procurement. It features a rich UI, 7 sponsor API integrations, a workflow state machine with human-in-the-loop guards, a risk scoring engine, document generation, and e-signature capabilities.

**Bottom line:** Solvent is a stronger *kernel* but a weaker *product*. AegisFlow demonstrates what the market actually needs: an end-to-end workflow with UI, integrations, human approval gates, and document automation. Solvent's database-enforced invariants are technically superior, but they remain invisible to the user without the surrounding product layer.

---

## 1. Architecture Comparison

| Dimension | Solvent | AegisFlow |
|---|---|---|
| **Language** | Go 1.25 | TypeScript 5 (strict) |
| **Framework** | Standard library HTTP + MCP SDK | Next.js 16 (App Router, Turbopack) |
| **Database** | CockroachDB Cloud Serverless (v26.2.5) | Xano (free-tier) + in-memory fallback |
| **UI** | 3-screen embedded demo wizard | Full dashboard with 8+ pages |
| **Deployment** | AWS App Runner | Vercel |
| **Vector Search** | CockroachDB native VECTOR(1024) | None (web search via SerpApi) |
| **Formal Verification** | Lean 4 + Mathlib (zero sorry) | None |
| **Agent Integration** | MCP server (16 tools, stdio) | None (Next.js server actions) |

**Gap:** Solvent has no modern web UI. AegisFlow's dashboard includes incident overview, audit trail, evidence panels, supplier comparison, document viewer, integration status, and approval queue — all things a non-technical user expects.

---

## 2. Feature-by-Feature Gap Analysis

### 2.1 Workflow State Machine

**AegisFlow:** 8-state FSM (`INVESTIGATING → RECOMMENDATION_READY → HUMAN_REVIEW → APPROVED → DOCUMENT_PREPARED → SIGNATURE_REQUIRED → SIGNED / REJECTED`). Transitions validated in code. Three states are `HUMAN_ONLY` — the AI orchestrator cannot cross them. `requiresHuman()` and `assertHumanMaySign()` enforce this structurally.

**Solvent:** Belief lifecycle has 3 states (`entered → promoted → retracted`). `action_intent` has 3 states (`live → cancelled → executed`). No explicit workflow FSM — the "workflow" is the promotion gate (debt must be empty) and intent gate (belief must be promoted).

**Gap:** Solvent has no concept of a *workflow* with ordered stages and human approval gates. The belief lifecycle is a state machine, but it is not exposed as a configurable workflow. A customer cannot define "Stage 1: AI investigates, Stage 2: Human reviews, Stage 3: Human approves" without building that logic themselves.

**Market need:** Enterprises need configurable approval workflows where AI does the work and humans sign off. Solvent's kernel *enforces* that a belief must be debt-free before promotion, but it does not provide the UI or workflow layer to make that enforcement usable.

### 2.2 Human-in-the-Loop Authorization

**AegisFlow:**
- `HUMAN_ONLY_TARGETS = ["APPROVED", "REJECTED", "SIGNED"]` — enforced in `machine.ts`
- `assertHumanMaySign(actor, state)` — throws `AgentAuthorizationError` if actor is not HUMAN
- `AGENT_TOOLS` registry classifies tools by risk (`REVERSIBLE` / `IRREVERSIBLE`) and maps each to allowed actors and required states
- `assertToolAllowed(toolId, actor, state)` — blocks AI from reaching irreversible operations
- Guards are called on the *path to the operation*, not as convention

**Solvent:**
- `action_intent` composite FK gate: `belief_status` must be `'promoted'` for `state = 'live'`
- `RetractCascade` cancels live intents before retracting (order enforced by schema)
- Authority lifecycle: `Approve` is the sole authority-creating operation, requires hash pin verification
- `Authorize` is read-only verification against snapshot
- `RevokeTarget` is append-only

**Gap:** Solvent's authority model is cryptographically stronger (hash pins, snapshot immutability), but it lacks *actor classification*. There is no concept of "this operation is AI-only, this one requires a human." The MCP server exposes 16 tools but does not classify them by risk or restrict them by actor type.

**Market need:** Customers need to know *who* (human vs AI) can perform *which* actions, and the system must enforce it. Solvent enforces that a belief must be promoted before an intent can cite it, but it does not enforce that a human must approve the promotion.

### 2.3 Risk Scoring and Decision Support

**AegisFlow:**
- 6-dimension risk engine: compliance (25), delivery (20), evidence (20), reliability (15), cost (10), compatibility (10)
- Each dimension scores 0-100 with cited reasons from evidence
- `INTEGRITY_CAP = 49` — supplier with unresolved CONFLICT capped at 49/100 regardless of weighting
- `evaluateSupplier()` and `evaluateAll()` produce ranked recommendations
- Transparent scoring: every dimension shows its evidence sources

**Solvent:**
- Beliefs have a `debt` array (6 items at entry)
- Debt items are retired one at a time via `RetireDebt`
- Promotion requires empty debt
- No scoring, no ranking, no multi-dimensional assessment
- Contradictions exist in `belief_edge` but do not affect a score

**Gap:** Solvent tracks *whether* a belief is ready for promotion, but it does not quantify *how ready* it is or *how risky* it is. There is no way to compare two beliefs by confidence, evidence quality, or risk.

**Market need:** Decision support requires scoring, ranking, and transparent reasoning. Solvent provides the atomic guarantee (no promotion with open debt), but customers need a risk engine on top of that guarantee.

### 2.4 External API Integrations

**AegisFlow:** 7 sponsor integrations, each with live/fallback paths, recorded in an Activity Ledger:
- **SerpApi** — web intelligence (5 concurrent queries)
- **Nutrient DWS** — PDF extraction + watermarking
- **Doctavian** — document generation from templates
- **Foxit eSign** — electronic signature
- **name.com** — domain availability check
- **Gemini** — LLM for analysis and decision narratives
- **Xano** — persistent storage with rate-limit-aware fallback

Every API call logged with `LIVE`/`LOCAL`/`DEMO SEEDED` tags, real request/response, timing, and status.

**Solvent:** 1 integration — Amazon Bedrock (Titan v2 embeddings). MCP server is a *tool surface*, not an integration layer. No Activity Ledger equivalent.

**Gap:** Solvent has no integration ecosystem. It cannot call external APIs, generate documents, send signatures, or interact with third-party services.

**Market need:** Real-world workflows require integration with document management, e-signature, CRM, ERP, and communication systems. Solvent's kernel is strong but isolated.

### 2.5 Document Generation and E-Signature

**AegisFlow:**
- Generates Emergency Supplier Transition Agreement via Doctavian
- Zod-validated contract payload with structured fields
- Watermarks PDF with "PENDING HUMAN SIGNATURE" via Nutrient
- Creates Foxit eSign folder with `sendNow: false`
- Entire flow guarded: only HUMAN actor from `SIGNATURE_REQUIRED` state can trigger signing

**Solvent:** No document generation. No e-signature. No contract payload.

**Gap:** This is the most visible gap for enterprise customers. Solvent can *decide* that a belief is authoritative and an action is permitted, but it cannot *produce the document* that records that decision or *route it for signature*.

**Market need:** Decisions need to become documents. Documents need signatures. Solvent's authority model is strong, but without document generation, the "action" that follows an authorized intent must be implemented by the customer.

### 2.6 User Interface

**AegisFlow:** Full React dashboard with:
- Incident overview cards, main incident console
- Audit trail (append-only event history)
- Evidence panel with external sources
- Supplier comparison and ranking
- Document viewer, integration activity ledger
- Approval queue, risk model with live re-weighting
- Demo controls with failure injection

**Solvent:** 3-screen embedded wizard:
1. ASK — search etcd issues, select evidence, attempt promotion
2. DISCHARGE — record review obligations, retry promotion
3. FALSIFY — introduce falsifier, observe cascade

**Gap:** Solvent's wizard is a *demo* — it proves the thesis but does not serve as a product UI. No dashboard, no audit trail view, no evidence browser, no approval queue, no settings page.

**Market need:** Non-technical users need a visual interface to review beliefs, approve actions, trace evidence, and manage the lifecycle. Solvent's MCP server serves *agents*, but agents are not the only consumers.

### 2.7 Demo Controls and Failure Injection

**AegisFlow:**
- Per-sponsor failure injection toggles
- One-click `Reset demo` button
- `DEMO SEEDED` mode for offline operation
- Every integration degrades gracefully with honest fallback

**Solvent:**
- Named scenarios (`SOLVENT_SCENARIO_1`, `SOLVENT_SCENARIO_2`)
- Single demo dataset (etcd issues)
- Deploy-time assertion of measured values
- No failure injection, no graceful degradation controls

**Gap:** Solvent's demo is tightly coupled to one scenario. There is no way to inject failures, toggle modes, or reset state from the UI.

**Market need:** Sales engineers need controllable demos that show both happy and failure paths. AegisFlow's demo controls make this trivial; Solvent's do not.

### 2.8 Evidence Status Tracking

**AegisFlow:** 5 evidence statuses: `VERIFIED`, `UNVERIFIED`, `CONFLICT`, `STALE`, `MISSING`. Each claim carries status, confidence score, conflict reason, and optional document evidence with verification rule name. Rules are *computed*, not scripted.

**Solvent:** Beliefs have status (`entered`/`promoted`/`retracted`). Evidence recorded with `provenance_class` and `content_sha256`. No concept of "conflicting" or "unverified" per claim. Contradictions exist in `belief_edge` but do not affect claim status.

**Gap:** Solvent tracks evidence *existence* but not evidence *quality*. A customer cannot see "3 of 5 claims are verified, 1 is conflicting, 1 is unverified."

**Market need:** Users need to understand the strength of evidence behind a belief. Solvent's debt mechanism partially addresses this, but it does not provide per-claim confidence or conflict tracking.

---

## 3. Where Solvent Is Stronger

### 3.1 Database-Enforced Invariants
Solvent's schema-level invariants (`promoted_is_debt_free`, `gate`, `live_requires_promoted`) are enforced by CockroachDB CHECK constraints and composite foreign keys. AegisFlow's guards are application-level — a code change could bypass them. In regulated industries (finance, healthcare, defense), this is a critical differentiator.

### 3.2 Formal Verification
Solvent's Lean 4 model proves 8 state-machine properties with zero `sorry` or `admit`. AegisFlow has none. Mathematical certainty is a differentiator for customers who need provable correctness.

### 3.3 MCP Server
Solvent's 16-tool MCP server exposes the ledger to any MCP-compatible agent. AegisFlow has no agent integration surface. As MCP adoption grows, Solvent is positioned as a backend that any agent can query.

### 3.4 Transactional Authority Model
Solvent's authority lifecycle (propose → justify → pin → approve → authorize → revoke) with hash-pin verification and snapshot immutability is cryptographically stronger than AegisFlow's application-level guards.

### 3.5 Vector Search with Real Embeddings
7,239 real etcd issues with genuine Titan v2 embeddings, served by CockroachDB's native vector index. AegisFlow uses web search with no local vector store.

---

## 4. Market Gaps Solvent Must Close

### Priority 1: Web UI (Critical)
Solvent needs a production dashboard: belief browser with status/debt/evidence/relationships, approval queue, audit trail with filtering, evidence viewer with provenance, scenario management, integration status.

### Priority 2: Configurable Workflow Engine (Critical)
Workflow layer on top of belief lifecycle: configurable stages, per-stage actor restrictions, per-stage tool restrictions, transition guards referencing evidence status.

### Priority 3: Risk Scoring Engine (High)
Multi-dimensional scoring with configurable weights, per-dimension evidence citation, integrity caps for conflicts, ranked recommendations, transparent reasoning.

### Priority 4: External API Integration Layer (High)
Pluggable adapter pattern, Activity Ledger recording every outbound call, live/fallback routing per integration, demo-seeded mode.

### Priority 5: Document Generation (Medium)
Template-based document generation, payload validation, PDF watermarking, e-signature integration.

### Priority 6: Multi-Scenario Support (Medium)
Scenario creation/management UI, configurable evidence sources, per-scenario vector indexes, scenario comparison.

### Priority 7: Demo Controls (Low)
Failure injection toggles, one-click reset, pre-seeded scenarios, narrative walkthrough.

---

## 5. Recommended Strategy

Solvent should not become AegisFlow. The kernel is the differentiator. The strategy is to **add the product layer that makes the kernel visible and usable**, while preserving the architectural guarantees.

- **Phase 1:** Make the kernel visible — web UI, scenario management, integration dashboard
- **Phase 2:** Make the kernel actionable — workflow engine, risk scoring, actor/tool restrictions
- **Phase 3:** Make the kernel productive — external integrations, document generation, e-signature
- **Phase 4:** Make the kernel demonstrable — demo controls, failure injection, narrative walkthrough

---

## 6. Conclusion

Solvent has the stronger foundation. Database-enforced invariants, formal verification, and transactional authority model are technically superior. But foundation is not product. The market does not buy invariants — it buys **workflows that are provably correct**. Solvent needs to wrap its kernel in the UI, workflow, and integration layers that make those invariants visible to the humans who depend on them. The gap is not technical depth — it is product breadth.
### Answer
Based on AegisFlow's actual implementation, here's concretely how Pithom Labs Solvent could translate these patterns onto its own kernel (Solvent isn't in this codebase, so this is a conceptual mapping of AegisFlow's mechanisms that Solvent could adopt):

## 1. Formalize a workflow layer on top of the belief lifecycle

AegisFlow's FSM is deliberately thin: an 8-state enum, a `VALID_TRANSITIONS` adjacency map, and a single `canTransition(from, to)` guard function aegisflow:4-13 , enforced centrally in `transitionIncident()` before any state mutation is persisted aegisflow:341-359 . Solvent's belief lifecycle (`entered → promoted → retracted`) and intent lifecycle (`live → cancelled → executed`) are currently enforced only as database-level gates (debt-empty check, composite FK), not as an explicit, named FSM with a transition table. Solvent could wrap these DB invariants in an equivalent `canTransition()`-style function and a `VALID_TRANSITIONS` map that names the *stages* customers actually care about (e.g., `AI_INVESTIGATING → EVIDENCE_COMPLETE → HUMAN_REVIEW → PROMOTED → AUTHORIZED → EXECUTED`), while keeping the CockroachDB CHECK constraints as the ultimate backstop. This gives Solvent both the database-enforced guarantee (its differentiator) and a legible, demoable workflow abstraction (AegisFlow's strength) — the CHECK constraints become the "structural, not a prompt" guarantee that AegisFlow can only offer at the application layer.

## 2. Add actor classification and a tool/action risk registry

AegisFlow's real HITL enforcement isn't the FSM alone — it's two additional layers: `HUMAN_ONLY_TARGETS` (`APPROVED`, `REJECTED`, `SIGNED`) checked via `requiresHuman()` aegisflow:350-355 , and a separate `AGENT_TOOLS` registry that classifies every operation by risk (`REVERSIBLE` vs `IRREVERSIBLE`) and by which actor (`SYSTEM`/`AI`/`HUMAN`) may invoke it in which state, enforced via `assertToolAllowed()` and `assertHumanMaySign()` which throw a dedicated `AgentAuthorizationError` on violation. Solvent's MCP server exposes 16 tools with no actor distinction at all — any MCP client can invoke `Approve`, `Authorize`, `RevokeTarget`, etc. Solvent should add an equivalent registry mapping each MCP tool to a risk class and allowed-actor set (e.g., `Propose`/`Justify` = REVERSIBLE, any-actor; `Approve`/`RevokeTarget` = IRREVERSIBLE, HUMAN-only), and enforce it at the MCP tool-dispatch layer the same way AegisFlow enforces it in `signAgreement()` and `approve()` in `lib/orchestration/actions.ts`. This turns Solvent's hash-pin/snapshot cryptographic guarantees into something that also blocks the *wrong actor* from ever reaching the authority-creating call, not just the wrong *data state*.

## 3. Surface the workflow and guards in a UI, reusing the audit-trail pattern

AegisFlow appends every transition to an audit log with `actor` and `timestamp`, tested directly in `tests/workflow.test.ts` aegisflow:23-30 , and renders it as a stepper/console in the dashboard. Solvent's belief/intent state changes are transactionally correct but invisible outside the demo wizard. Adopting an append-only audit event table keyed to belief/intent transitions (mirroring `appendAudit()`) and a corresponding UI view would let Solvent show, per belief, exactly which actor (human or agent) promoted it, retired which debt item, or approved which intent — turning invariant enforcement into a legible product feature rather than a backend guarantee nobody sees.

## 4. Add a scoring/decision layer without touching the kernel

AegisFlow's risk engine sits *above* its FSM as a pure, cited scoring function (`evaluateSupplier()`) with an `INTEGRITY_CAP` that overrides weighting when evidence conflicts aegisflow:265-274 . Solvent's belief `debt` array and `belief_edge` contradictions are structurally analogous to AegisFlow's claim `EvidenceStatus` (`VERIFIED`/`CONFLICT`/etc.) aegisflow:15-38  — Solvent already tracks *whether* debt is retired but not *how much confidence* each retired-debt item deserves. A scoring layer that reads belief debt/edge data and produces a transparent, per-belief confidence score (with an integrity cap when unresolved contradictions exist in `belief_edge`) would give Solvent AegisFlow-style decision support without weakening the underlying promotion gate — the cap logic and the DB CHECK constraint can coexist, the same way AegisFlow's risk gate and its FSM guard are independent, layered controls.

**Net recommendation:** Solvent doesn't need to abandon its DB-enforced, formally-verified kernel to close this gap — it needs to add three thin layers on top of it, each modeled directly on an AegisFlow mechanism: (1) a named workflow/FSM abstraction over the existing lifecycle states, (2) an actor+risk classification registry gating MCP tool calls the way `AGENT_TOOLS`/`assertToolAllowed` gate AegisFlow's operations, and (3) an audit-trail + scoring UI that makes the invariants visible. This preserves Solvent's structural advantage (CHECK constraints can't be bypassed by a code change, unlike AegisFlow's `assertHumanMaySign`) while adopting the parts of AegisFlow's design that make a workflow *legible and demoable* to non-technical buyers.

### Citations
**File:** schemas/core.ts (L4-13)
```typescript
export const WorkflowState = z.enum([
  "INVESTIGATING",
  "RECOMMENDATION_READY",
  "HUMAN_REVIEW",
  "APPROVED",
  "DOCUMENT_PREPARED",
  "SIGNATURE_REQUIRED",
  "SIGNED",
  "REJECTED"
]);
```
**File:** schemas/core.ts (L15-38)
```typescript
export const EvidenceStatus = z.enum([
  "VERIFIED",
  "UNVERIFIED",
  "CONFLICT",
  "STALE",
  "MISSING"
]);

export const ClaimSchema = z.object({
  id: z.string(),
  text: z.string(),
  source: z.string(),
  timestamp: z.string(),
  confidence: z.number().min(0).max(100),
  status: EvidenceStatus,
  conflictReason: z.string().optional(),
  documentEvidence: z.object({
    documentId: z.string(),
    field: z.string(),
    mode: z.enum(["LIVE", "LOCAL"]),
    /** The verification rule that produced this verdict. */
    rule: z.string().optional(),
  }).optional(),
});
```
**File:** lib/incidents/repository.ts (L341-359)
```typescript
export async function transitionIncident(
  id: string,
  to: WorkflowStateType,
  actor: "SYSTEM" | "AI" | "HUMAN",
  event?: string
): Promise<void> {
  const repository = getRepository();
  const incident = await repository.getIncident(id);
  if (!incident) throw new Error(`Incident ${id} not found`);
  if (!canTransition(incident.state, to)) {
    throw new Error(`Invalid transition: ${incident.state} -> ${to}`);
  }
  if (requiresHuman(to) && actor !== "HUMAN") {
    throw new Error(`Blocked: ${actor} may not perform the human-only transition -> ${to}`);
  }
  incident.state = to;
  await repository.saveIncident(incident);
  await repository.appendAudit(id, event ?? `State transition: ${to}`, actor);
}
```
**File:** tests/workflow.test.ts (L23-30)
```typescript
  it("records audit events with actor and timestamp", async () => {
    await appendAudit("INC-1042", "test event", "HUMAN");
    const incident = await getIncident("INC-1042");
    const last = incident!.auditLog[incident!.auditLog.length - 1];
    expect(last.event).toBe("test event");
    expect(last.actor).toBe("HUMAN");
    expect(last.timestamp).toBeTruthy();
  });
```
**File:** README.md (L265-274)
```markdown
### The integrity gate: you can't weight your way to a bad supplier

The risk model is fully transparent and the weights are live-adjustable on the
*Why this recommendation?* screen. But a supplier carrying an **unresolved
evidence conflict** is capped at `INTEGRITY_CAP = 49` regardless of weighting
([`weights.ts:35`](lib/risk/weights.ts#L35), [`engine.ts:126`](lib/risk/engine.ts#L126)).
Drag the cost weight to maximum and the conflicted cheapest supplier still loses —
its pre-gate score is shown alongside the capped one, so the gate is visible, not
hidden. A test pins this: the raw score *would* win, the gated score does not
([`risk.test.ts`](tests/risk.test.ts)).
```
