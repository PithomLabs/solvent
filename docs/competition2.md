# Competitive Analysis: Solvent vs DealForge — The Operating System Kernel Design Pattern

**Date:** 2026-09-04  
**Purpose:** Expand on `competition.md` findings by analyzing DealForge's port/adapter architecture as a blueprint for designing Solvent as an operating system kernel with extension mechanisms for enterprise workflows.

---

## Executive Summary

DealForge is not a competitor to Solvent — it is a **blueprint**. DealForge demonstrates exactly how to separate a trust kernel from provider execution, and that separation is the missing design pattern in Solvent.

Solvent's kernel already *is* an operating system kernel: it enforces invariants through CockroachDB constraints, governs authority through cryptographic hash pins, and refuses to let agents act on beliefs that are no longer true. But an OS kernel without system calls is a trapped algorithm. DealForge shows what Solvent needs: **port interfaces** that let customers plug in their own evidence sources, workflow stages, document generators, and signature providers without modifying the kernel.

The insight: Solvent does not need to become a product. It needs to become a **platform** — an invariant layer that enterprise workflows extend through standardized interfaces.

---

## 1. DealForge's Architecture: The Pattern to Copy

### 1.1 The Trust Kernel Separation

DealForge's architecture answers one question: **Which component is allowed to make a fact authoritative or cause an external side effect?**

```
Untrusted Source → AI Extraction → Candidate Facts → Provenance Validation
    → Product Catalog → Canonical Manifest → Deterministic Validation
    → Readiness Decision → Generation → Signature → Completion
```

Every boundary is explicit:
- The AI extracts *candidates*, never facts
- The catalog assigns *authoritative* product codes, discarding AI-invented ones
- Financial totals are *computed* by DealForge, never by the AI
- Generation requires `READY` status, re-validated at every provider mutation
- Signature sending requires explicit human `SEND` confirmation
- Completed state is *never regressed* by stale provider reads

**Solvent's kernel already does this.** The database enforces invariants. The authority lifecycle requires hash pins. The composite FK gate refuses intents on non-promoted beliefs. But Solvent stops at the kernel boundary — there is no standardized way to extend it.

### 1.2 The Port/Adapter Pattern

DealForge defines three port interfaces:

```typescript
// AI extraction — provider-agnostic
interface DealExtractionModel {
  extractDeal(input: DealExtractionModelInput): Promise<unknown>;
}

// Document generation — provider-agnostic
interface DoctavianGenerationPort {
  uploadTemplate(input: { fileName: string; bytes: Uint8Array }): Promise<DoctavianUploadedFile>;
  uploadData(data: unknown): Promise<DoctavianUploadedFile>;
  generateDocument(request: DoctavianGenerateRequest): Promise<DoctavianGeneratedDocument>;
}

// E-signature — provider-agnostic
interface DoctavianSignaturePort {
  createEnvelope(request: DoctavianEnvelopeCreateRequest): Promise<DoctavianCreatedEnvelope>;
  sendEnvelope(envelopeId: string): Promise<DoctavianSentEnvelope>;
  getEnvelope(envelopeId: string): Promise<DoctavianEnvelopeStatus>;
  downloadEnvelopeDocument(envelopeId: string, documentId: string): Promise<Uint8Array>;
}
```

Adapters implement these ports:
- `GoogleDealModel` implements `DealExtractionModel`
- `OpenAIDealModel` implements `DealExtractionModel`
- `DoctavianClient` implements both `DoctavianGenerationPort` and `DoctavianSignaturePort`

The decorator `CatalogBackedDealExtractionModel` wraps any `DealExtractionModel` and applies authoritative product code resolution after extraction — proving that adapters can be composed.

**Solvent has no port interfaces.** The kernel package exposes concrete functions (`EnterBelief`, `Promote`, etc.) but no abstraction layer for external services. The MCP server is a *tool surface*, not an extension mechanism. There is no way to plug in a new evidence source, document generator, or signature provider without writing a new adapter that calls kernel functions directly.

### 1.3 The Domain Layer

DealForge's domain layer has zero provider dependencies:

```typescript
// Canonical domain schema (Zod-validated)
DealManifest — { customer, signer, commercialTerms, lineItems, requirements }

// Preparation pipeline (validate → calculate → evaluate)
prepareDealForGeneration(manifest) → Invalid | Blocked | Review | Ready

// Validation rules with severity levels
evaluateDealForGeneration(manifest) → { status, canGenerate, findings[] }

// Financial calculations (BigInt integer money)
calculateDealTotals(manifest) → DealTotals
```

The domain layer is the **single source of truth** for what a "deal" is. It does not know about Gemini, Doctavian, or Foxit. It knows about commercial terms, line items, and readiness rules.

**Solvent's kernel is the domain layer** — but it is also the persistence layer. There is no separation between "what a belief is" and "how a belief is stored." The `kernel.Contract` interface mixes domain operations (`EnterBelief`, `Promote`) with persistence concerns (`crdb.ExecuteTx`). This makes it hard to add domain-specific validation or preparation logic without modifying the kernel.

### 1.4 Workflow Tokens

DealForge uses HMAC-sealed tokens for stateless browser continuity:

```typescript
// Seal: state → base64url(payload).base64url(signature)
sealDealWorkflowState(state, secret) → string

// Open: token → validated state (fails on tamper)
openDealWorkflowToken(token, secret) → DealWorkflowState
```

Key properties:
- The browser never carries mutable deal objects between steps
- Server reconstructs authoritative state from the manifest before every provider mutation
- Tokens are tamper-evident (HMAC-SHA256 with timingSafeEqual)
- Identity is asserted at every boundary (`dealId` must match manifest)

**Solvent has no workflow token mechanism.** The MCP server passes belief IDs and scenario IDs as plain strings. There is no tamper-evident state continuity. A browser or agent could pass arbitrary strings — the kernel validates them against the database, but there is no cryptographic binding between steps.

### 1.5 The Preparation Boundary

DealForge re-runs domain preparation before every provider mutation:

```typescript
function getGenerationReadyDealFromWorkflow(state: DealWorkflowState): GenerationReadyDeal {
  // Never rely solely on the fact that the state was previously signed.
  // Re-run the current authoritative domain preparation before every provider mutation.
  const preparation = prepareDealForGeneration(state.manifest);
  if (preparation.status !== "ready") {
    throw new Error(`Workflow is no longer generation-ready; current status is ${preparation.status}.`);
  }
  return { manifest: preparation.manifest, totals: preparation.totals };
}
```

This means a future policy change can invalidate an older workflow token. The preparation boundary is the **gate** that prevents stale or manipulated state from reaching providers.

**Solvent has no preparation boundary.** The `Promote` function checks the schema gate (debt must be empty), but there is no equivalent of "re-validate before every provider mutation." The kernel enforces invariants at write time, but there is no mechanism to re-check domain-specific rules before external operations.

### 1.6 Observability

DealForge's Trusted Execution Trace records *what happened*, not chain-of-thought:

```text
USER → AI → DEALFORGE → CATALOG → FINANCE → HUMAN → DOCTAVIAN
```

Every trace event is created from successful observed application/provider transitions. The trace explains the path from intake to completion without exposing AI reasoning.

**Solvent has no equivalent.** The `evidence` table records provenance, and `action_intent` records actions, but there is no unified execution trace that shows the path from evidence ingestion through belief promotion to action execution.

---

## 2. Solvent as an Operating System Kernel

### 2.1 The Metaphor

An operating system kernel provides:

| OS Concept | Solvent Equivalent | What's Missing |
|---|---|---|
| **System calls** | `kernel.Contract` interface | Standardized port interfaces for external interaction |
| **Device drivers** | MCP server (16 tools) | Adapters for evidence sources, document generators, signature providers |
| **Process context** | Scenario ID | Tamper-evident workflow tokens |
| **Memory management** | CockroachDB transactions | Preparation boundary (re-validate before mutations) |
| **Audit logging** | `evidence` + `action_intent` tables | Unified execution trace |
| **File system** | Corpus (vector search) | Pluggable storage backends |
| **IPC** | MCP server (stdio) | Event bus for workflow stage transitions |

The kernel is already strong. What's missing is the **extension mechanism** — the system calls that let enterprise workflows interact with the kernel without modifying it.

### 2.2 What the Kernel Provides

Solvent's kernel already provides:

1. **Invariant enforcement** — `promoted_is_debt_free`, `gate`, `live_requires_promoted` (CockroachDB constraints)
2. **Transactional authority** — hash-pin verification, snapshot immutability, append-only revocation
3. **Belief lifecycle** — `EnterBelief → AddEvidence → RetireDebt → Promote → IntentOnPromoted`
4. **Cascade retraction** — `RetractCascade` cancels intents before retracting beliefs
5. **Vector search** — CockroachDB native VECTOR(1024) with cosine distance
6. **Formal verification** — Lean 4 model proves 8 state-machine properties

These are the **invariant guarantees** that no extension can violate. They are the kernel's contract with the world.

### 2.3 What the Extension Layer Provides

DealForge shows that the extension layer provides:

1. **Port interfaces** — standardized contracts for external services
2. **Adapter implementations** — pluggable providers that implement ports
3. **Domain schemas** — Zod-like validation of business objects
4. **Preparation pipelines** — validate → calculate → evaluate before mutations
5. **Workflow tokens** — tamper-evident state continuity across boundaries
6. **Activity ledgers** — record every external call with provenance
7. **Validation rules** — severity-level findings (ready/review/blocked)
8. **Observability traces** — execution path recording

Solvent needs all of these — but as an **extension layer** that wraps the kernel, not as modifications to the kernel itself.

---

## 3. The Port Interfaces Solvent Needs

Based on DealForge's pattern, Solvent should define these port interfaces:

### 3.1 Evidence Source Port

```go
// EvidenceSource is the port interface for external evidence providers.
// Adapters implement this to connect Solvent to specific evidence feeds.
type EvidenceSource interface {
    // Name returns the source identifier (e.g., "github-issues", "jira", "confluence").
    Name() string
    
    // Fetch retrieves evidence matching the given query within the scenario.
    Fetch(ctx context.Context, scenarioID string, query EvidenceQuery) ([]EvidenceCandidate, error)
    
    // Embed converts raw text into a vector embedding.
    Embed(ctx context.Context, text string) ([]float32, error)
}

type EvidenceQuery struct {
    Text      string
    MaxHits   int
    MaxDistance float64
}

type EvidenceCandidate struct {
    SourceID    string
    Title       string
    Body        string
    URL         string
    Distance    float64
    Metadata    map[string]string
}
```

**Why:** Solvent currently hardcodes Amazon Bedrock Titan for embeddings and etcd issues for the corpus. An `EvidenceSource` port lets customers plug in their own evidence feeds (Jira, Confluence, GitHub, internal databases) without modifying the kernel.

### 3.2 Workflow Stage Port

```go
// WorkflowStage defines a named stage in an enterprise workflow.
// The kernel does not enforce workflow order — extensions do.
type WorkflowStage interface {
    Name() string
    
    // Evaluate checks whether the belief/intent can advance from this stage.
    // Returns findings with severity levels (ready/review/blocked).
    Evaluate(ctx context.Context, state WorkflowState) (StageEvaluation, error)
    
    // Advance transitions the workflow to the next stage.
    // The kernel enforces invariants; the stage enforces domain rules.
    Advance(ctx context.Context, state WorkflowState) (WorkflowState, error)
}

type StageEvaluation struct {
    Status     string // "ready", "review", "blocked"
    CanAdvance bool
    Findings   []Finding
}

type Finding struct {
    Code     string
    Severity string // "review", "blocking"
    Path     string
    Message  string
}
```

**Why:** Solvent's belief lifecycle has 3 states (`entered → promoted → retracted`). Enterprises need workflows with ordered stages, per-stage actor restrictions, and severity-level findings. A `WorkflowStage` port lets customers define their own workflow stages without modifying the kernel.

### 3.3 Document Generator Port

```go
// DocumentGenerator produces artifacts from promoted beliefs.
// Adapters implement this to connect Solvent to document generation services.
type DocumentGenerator interface {
    // Generate produces a document from the given template and data.
    Generate(ctx context.Context, request GenerateRequest) (GeneratedDocument, error)
    
    // Validate checks that the template and data are consistent.
    Validate(ctx context.Context, request GenerateRequest) ([]Finding, error)
}

type GenerateRequest struct {
    TemplateID string
    Data       map[string]interface{}
    Format     string // "pdf", "docx", "html"
}

type GeneratedDocument struct {
    ID       string
    URI      string
    Format   string
    Size     int64
    Hash     string // SHA-256 of content
}
```

**Why:** Solvent can *decide* that a belief is authoritative, but it cannot *produce the document* that records that decision. A `DocumentGenerator` port lets customers connect Doctavian, Nutrient, or any document generation service.

### 3.4 Signature Provider Port

```go
// SignatureProvider handles electronic signature workflows.
// Adapters implement this to connect Solvent to e-signature services.
type SignatureProvider interface {
    // CreateDraft creates a signature envelope without sending it.
    CreateDraft(ctx context.Context, request DraftRequest) (SignatureEnvelope, error)
    
    // Send explicitly authorizes sending the envelope to signers.
    Send(ctx context.Context, envelopeID string) error
    
    // Status retrieves the current signature status.
    Status(ctx context.Context, envelopeID string) (SignatureStatus, error)
    
    // Download retrieves the signed document.
    Download(ctx context.Context, envelopeID string, documentID string) ([]byte, error)
}

type SignatureEnvelope struct {
    ID          string
    Status      string // "draft", "in_progress", "completed"
    DocumentID  string
    RecipientID string
}

type SignatureStatus struct {
    EnvelopeStatus  string
    RecipientStatus string
    Completed       bool
}
```

**Why:** DealForge's signature workflow (Draft → Human SEND → In Progress → Completed) is exactly the pattern enterprises need. A `SignatureProvider` port lets customers connect Foxit, DocuSign, or any e-signature service.

### 3.5 Activity Ledger Port

```go
// ActivityLedger records every external call with provenance.
// Adapters implement this to connect Solvent to audit/observability systems.
type ActivityLedger interface {
    // Record stores an activity event.
    Record(ctx context.Context, event ActivityEvent) error
    
    // Query retrieves activity events for a scenario.
    Query(ctx context.Context, scenarioID string, filter ActivityFilter) ([]ActivityEvent, error)
}

type ActivityEvent struct {
    ID          string
    ScenarioID  string
    Timestamp   time.Time
    Actor       string // "system", "ai", "human"
    Operation   string
    Service     string
    Request     interface{}
    Response    interface{}
    Mode        string // "live", "local", "demo_seeded"
    Status      string // "ok", "fallback", "error"
    Duration    time.Duration
    BeliefID    *string
    IntentID    *string
}

type ActivityFilter struct {
    Actor     string
    Service   string
    Status    string
    After     time.Time
    Before    time.Time
}
```

**Why:** AegisFlow's Activity Ledger records every API call with `LIVE`/`LOCAL`/`DEMO SEEDED` tags. Enterprises need this for compliance, debugging, and audit. An `ActivityLedger` port lets customers connect their own observability systems.

---

## 4. The Workflow Token Pattern

DealForge's HMAC-sealed tokens are a critical pattern Solvent should adopt.

### 4.1 Current Solvent Pattern

```
Browser → MCP tool call (beliefID, scenarioID) → Kernel → Database
```

The browser passes plain identifiers. The kernel validates them against the database. There is no tamper-evident state continuity.

### 4.2 DealForge's Pattern

```
Browser → HMAC-sealed token → Server → Open + Validate → Provider → New Token → Browser
```

The browser carries a sealed token that:
- Contains the workflow state (stage, manifest, document reference, envelope reference)
- Is tamper-evident (HMAC-SHA256)
- Is re-validated at every boundary
- Carries identity assertions (dealId must match manifest)

### 4.3 Solvent's Workflow Token

Solvent should define a workflow token that carries:

```go
type WorkflowToken struct {
    Version    int
    ScenarioID string
    BeliefID   string
    Stage      string
    Evidence   []EvidenceRef
    Debt       []string
    Authority  *AuthorityRef
    CreatedAt  time.Time
    ExpiresAt  time.Time
}

type EvidenceRef struct {
    ID       string
    Source   string
    Distance float64
}

type AuthorityRef struct {
    TargetID    string
    SnapshotID  string
    ApprovedAt  time.Time
}
```

The token would be HMAC-sealed and carried by the browser/agent. Every kernel mutation would:
1. Open and validate the token
2. Assert the token's state matches the database state
3. Perform the mutation
4. Seal a new token with the updated state
5. Return the new token to the caller

This prevents:
- Stale state from reaching the kernel
- Tampered state from passing validation
- Replay attacks (token includes timestamp and expiry)

---

## 5. The Preparation Boundary

DealForge's preparation boundary is the pattern Solvent needs:

```typescript
// DealForge: re-run before every provider mutation
function getGenerationReadyDealFromWorkflow(state: DealWorkflowState): GenerationReadyDeal {
  const preparation = prepareDealForGeneration(state.manifest);
  if (preparation.status !== "ready") {
    throw new Error(`Workflow is no longer generation-ready; current status is ${preparation.status}.`);
  }
  return { manifest: preparation.manifest, totals: preparation.totals };
}
```

Solvent's equivalent would be:

```go
// Solvent: re-validate before every external operation
func (s *Store) PrepareForAction(ctx context.Context, scenarioID, beliefID string) (ActionReadyBelief, error) {
    // 1. Read the belief
    belief, err := s.GetBelief(ctx, scenarioID, beliefID)
    if err != nil {
        return ActionReadyBelief{}, err
    }
    
    // 2. Re-validate domain rules (extensible via WorkflowStage ports)
    evaluation := s.evaluateBeliefForAction(belief)
    if evaluation.Status != "ready" {
        return ActionReadyBelief{}, ErrNotReadyForAction
    }
    
    // 3. Assert no live intents on retracted beliefs (I-5)
    count, err := s.AuditLiveOnNonPromoted(ctx, scenarioID)
    if err != nil {
        return ActionReadyBelief{}, err
    }
    if count != 0 {
        return ActionReadyBelief{}, ErrInvariantViolation
    }
    
    return ActionReadyBelief{Belief: belief, Evaluation: evaluation}, nil
}
```

This means:
- A future policy change can invalidate an older workflow token
- Domain-specific validation runs before every external operation
- The kernel's invariants are re-checked at the preparation boundary

---

## 6. The Activity Ledger

DealForge's execution trace maps directly to Solvent's needs:

```text
DealForge: USER → AI → DEALFORGE → CATALOG → FINANCE → HUMAN → DOCTAVIAN
Solvent:   AGENT → EVIDENCE → KERNEL → CORPUS → PROMOTE → HUMAN → ACTION
```

Solvent's Activity Ledger should record:

| Event | Source | Description |
|---|---|---|
| `evidence.ingested` | EvidenceSource | Raw evidence fetched from external feed |
| `evidence.embedded` | EvidenceSource | Vector embedding generated |
| `belief.entered` | Kernel | New belief created in the ledger |
| `debt.retired` | Kernel | Debt item retired via evidence |
| `belief.promoted` | Kernel | Belief promoted (debt-free, schema gate passed) |
| `action.intent` | Kernel | Live action intent recorded |
| `action.executed` | Extension | External action completed |
| `document.generated` | DocumentGenerator | Document produced from belief |
| `signature.drafted` | SignatureProvider | Signature envelope created |
| `signature.sent` | SignatureProvider | Envelope sent to signer |
| `signature.completed` | SignatureProvider | Signing completed |

Each event carries: timestamp, actor, operation, service, mode (live/local/demo), status, duration, and references to belief/intent IDs.

---

## 7. The Validation Rules Engine

DealForge's validation rules have severity levels:

```typescript
type DealFindingSeverity = "review" | "blocking";

interface DealFinding {
  code: string;
  severity: DealFindingSeverity;
  path: string;
  message: string;
}

interface DealGenerationEvaluation {
  status: "ready" | "review" | "blocked";
  canGenerate: boolean;
  findings: DealFinding[];
}
```

Solvent should adopt the same pattern for belief validation:

```go
type FindingSeverity string

const (
    SeverityReview   FindingSeverity = "review"
    SeverityBlocking FindingSeverity = "blocking"
)

type Finding struct {
    Code     string
    Severity FindingSeverity
    Path     string
    Message  string
}

type BeliefEvaluation struct {
    Status      string // "ready", "review", "blocked"
    CanPromote  bool
    CanAction   bool
    Findings    []Finding
}
```

This lets customers define domain-specific validation rules:
- "Discount > 20% requires review" → `discount_requires_review`
- "Payment terms > 60 days requires review" → `payment_terms_require_review`
- "Contradictory evidence blocks promotion" → `contradiction_blocks_promotion`
- "Missing provenance blocks action" → `missing_provenance_blocks_action`

---

## 8. The Extension Architecture

### 8.1 Package Structure

```
kernel/                    # FROZEN — invariant enforcement
├── kernel.go              # Belief lifecycle
├── authority.go           # Authority lifecycle
├── contract.go            # Interface contract
├── errors.go              # Sentinel errors
├── sql.go                 # SQL statements
└── schema/                # Database schema

extension/                 # NEW — extension mechanism
├── port/                  # Port interfaces
│   ├── evidence.go        # EvidenceSource interface
│   ├── workflow.go        # WorkflowStage interface
│   ├── document.go        # DocumentGenerator interface
│   ├── signature.go       # SignatureProvider interface
│   └── ledger.go          # ActivityLedger interface
├── token/                 # Workflow tokens
│   ├── seal.go            # HMAC sealing
│   └── open.go            # HMAC validation
├── prepare/               # Preparation boundary
│   └── prepare.go         # Re-validate before mutations
├── evaluate/              # Validation rules engine
│   └── evaluate.go        # Finding severity levels
└── trace/                 # Execution trace
    └── trace.go           # Activity recording

adapter/                   # NEW — pluggable adapters
├── bedrock/               # Amazon Bedrock adapter (evidence source)
├── doctavian/             # Doctavian adapter (document generation)
├── foxit/                 # Foxit adapter (signature provider)
├── xano/                  # Xano adapter (activity ledger)
└── ...                    # Customer-provided adapters
```

### 8.2 The Dependency Injection Pattern

DealForge uses constructor injection:

```typescript
interface DealWorkflowRuntimeDependencies {
  model: DealExtractionModel;
  doctavian: DoctavianWorkflowPort;
  signingSecret: string;
  templateFileName: string;
  templateBytes: Uint8Array;
  sender: { name: string; email: string };
  signatureField: SignatureFieldPlacement;
}
```

Solvent should use the same pattern:

```go
type ExtensionDependencies struct {
    EvidenceSources   map[string]EvidenceSource
    WorkflowStages    []WorkflowStage
    DocumentGenerator DocumentGenerator
    SignatureProvider SignatureProvider
    ActivityLedger    ActivityLedger
    SigningSecret     string
}

type ExtensionRuntime struct {
    kernel  *kernel.Store
    deps    ExtensionDependencies
    token   *TokenService
    prepare *PreparationService
    trace   *TraceService
}
```

### 8.3 The Lazy Initialization Pattern

DealForge's `createLazyDoctavianPort()` defers credential resolution until first use. Solvent should do the same:

```go
func NewLazyEvidenceSource(factory func() EvidenceSource) EvidenceSource {
    var (
        source EvidenceSource
        once   sync.Once
        err    error
    )
    return &lazyEvidenceSource{
        resolve: func() (EvidenceSource, error) {
            once.Do(func() {
                source, err = factory()
            })
            return source, err
        },
    }
}
```

This allows:
- The kernel to start without all adapters configured
- Adapters to be resolved on first use
- Graceful degradation when adapters are unavailable

---

## 9. What Solvent Should NOT Copy

### 9.1 DealForge's Stateless Persistence

DealForge has no database — it uses HMAC tokens for state continuity. This works for a demo but not for production. Solvent's CockroachDB-backed persistence is stronger and should be preserved.

### 9.2 DealForge's Single-Process Idempotency

DealForge uses `Map<string, Promise>` for same-process deduplication. This does not work across processes or serverless instances. Solvent's database-backed transactions are the correct idempotency mechanism.

### 9.3 DealForge's Provider State Reconciliation

DealForge reconciles provider state because Doctavian's read model can be stale. Solvent's kernel is the source of truth — there is no external provider state to reconcile. The Activity Ledger records external calls, but the kernel does not depend on external state.

### 9.4 DealForge's Financial Calculations

DealForge's BigInt integer money and basis-point discounts are domain-specific. Solvent is domain-agnostic — it should not include financial calculation logic. That belongs in customer extensions.

---

## 10. The Operating System Kernel Design Principles

Based on DealForge's patterns and Solvent's existing architecture, the design principles are:

### Principle 1: The Kernel Is Frozen

The kernel's invariants, transactions, and authority model are non-negotiable. Extensions cannot violate them. This is the OS kernel's contract with the world.

### Principle 2: Extensions Are Pluggable

Every external interaction goes through a port interface. Adapters implement ports. Customers provide their own adapters. The kernel does not know about Gemini, Doctavian, or Foxit.

### Principle 3: The Preparation Boundary Is Mandatory

Before every external mutation, the preparation boundary re-validates domain rules. A future policy change can invalidate an older workflow token. The kernel's invariants are re-checked at this boundary.

### Principle 4: Tokens Are Tamper-Evident

Workflow tokens are HMAC-sealed. The browser/agent carries sealed state. The server opens and validates at every boundary. Tampered or stale tokens are rejected.

### Principle 5: The Activity Ledger Is Append-Only

Every external call is recorded with provenance. The ledger is append-only. It explains *what happened*, not chain-of-thought. It is the audit trail for compliance and debugging.

### Principle 6: Validation Has Severity Levels

Domain rules produce findings with severity: `ready`, `review`, or `blocking`. `ready` permits advancement. `review` requires human attention. `blocking` prevents advancement. This maps directly to DealForge's pattern.

### Principle 7: The Kernel Does Not Own Domain Logic

The kernel enforces invariants. Domain logic (financial calculations, validation rules, workflow stages) belongs in extensions. The kernel is domain-agnostic by design.

---

## 11. Recommended Strategy (Updated)

### Phase 1: Define Port Interfaces
Define the five port interfaces: `EvidenceSource`, `WorkflowStage`, `DocumentGenerator`, `SignatureProvider`, `ActivityLedger`. These are the system calls that extensions use to interact with the kernel.

### Phase 2: Implement Workflow Tokens
Implement HMAC-sealed workflow tokens that carry belief/intent context across browser/agent boundaries. Re-validate at every kernel mutation.

### Phase 3: Build the Preparation Boundary
Implement the preparation boundary that re-validates domain rules before every external mutation. Integrate with `WorkflowStage` ports for extensible validation.

### Phase 4: Implement the Activity Ledger
Implement the append-only Activity Ledger that records every external call with provenance. Integrate with all port interfaces.

### Phase 5: Build Reference Adapters
Build reference adapters for Bedrock (evidence), a document generator, a signature provider, and an activity ledger. These prove the port interfaces work.

### Phase 6: Expose via MCP
Expose the extension layer through the existing MCP server. New tools for workflow tokens, preparation boundaries, and activity queries.

---

## 12. Conclusion

DealForge is not a competitor — it is a **design reference**. It shows how to separate a trust kernel from provider execution, and that separation is exactly what Solvent needs.

Solvent's kernel is already stronger than anything DealForge or AegisFlow provides. Database-enforced invariants, formal verification, and transactional authority model are technically superior. But the kernel is trapped — there is no standardized way to extend it.

The operating system kernel design pattern solves this:
- **Port interfaces** = system calls (standardized contracts for external interaction)
- **Adapters** = device drivers (plug in specific providers)
- **Workflow tokens** = process context (tamper-evident state across boundaries)
- **Preparation boundary** = memory management (re-validate before mutations)
- **Activity ledger** = audit log (observability into every operation)

Solvent does not need to become a product. It needs to become a **platform** — an invariant layer that enterprise workflows extend through standardized interfaces. DealForge shows how. The kernel is the foundation. The ports are the extension mechanism. The adapters are the pluggable providers. Together, they make Solvent the operating system for autonomous agent authority.
