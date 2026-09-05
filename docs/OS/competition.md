# Competitive Analysis: Solvent vs AegisFlow

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
