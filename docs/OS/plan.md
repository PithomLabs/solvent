# Solvent Commercial MVP — Implementation Plan

**Date:** 2026-09-04  
**Status:** PLANNING  
**Goal:** Extend Solvent from a technically strong v0 kernel/demo into a minimum viable commercial product without enlarging the trusted kernel.

---

## 1. Architecture Map (Current State)

```
                    EXTERNAL SYSTEMS
                          |
                     ┌────┴────┐
                     │ADAPTERS │  (bedrock, agentjacking, mcp)
                     └────┬────┘
                          |
                     ┌────┴────┐
                     │SERVICE  │  (wizard, belief, pipeline, derive, normalize)
                     │  LAYER  │
                     └────┬────┘
                          |
                ┌─────────┴─────────┐
                │  SOLVENT KERNEL   │  (kernel/, authority.go, sql.go)
                │  Domain-agnostic  │
                │  DB-enforced      │
                └─────────┬─────────┘
                          |
                     COCKROACHDB
```

### Current Packages

| Package | Location | Purpose |
|---|---|---|
| `kernel` | `kernel/` | Belief lifecycle, authority lifecycle, DB invariants |
| `belief` | `internal/belief/` | EnsureBelief + AddEvidence + RetireDebt + Promote |
| `corpus` | `internal/corpus/` | Vector search, evidence storage |
| `derive` | `internal/derive/` | Belief derivation from normalized evidence |
| `normalize` | `internal/normalize/` | Evidence normalization from raw fixtures |
| `pipeline` | `internal/pipeline/` | Orchestration: normalize → derive → belief → intent |
| `intent` | `internal/intent/` | Intent audit & proposal |
| `wizard` | `internal/wizard/` | 3-screen demo: ASK, DISCHARGE, FALSIFY |
| `view` | `internal/view/` | Read-only projections |
| `agentjacking` | `internal/agentjacking/` | Agentjacking evidence adapter |
| `mcp` | `cmd/solvent-mcp/` | MCP server (16 tools, stdio) |
| `web` | `demo/cloud/web/` | Production web server |

### Current DB Schema (6 migrations)

| Migration | Tables | Purpose |
|---|---|---|
| `001_schema.sql` | `belief`, `belief_edge`, `evidence`, `action_intent` | Core ledger |
| `002_corpus.sql` | `corpus_issue`, `belief_corpus_citation` | Vector search |
| `003_wizard.sql` | `refusal_log` | Demo wizard |
| `004_debt_vocabulary.sql` | — | Updated debt items |
| `005_authority_mvp.sql` | `principal`, `authority_target`, `target_snapshot`, `target_activation`, `target_revocation`, `justification`, `debt_discharge` | Authority lifecycle |
| `006_authority_justification_cascade.sql` | — | ON UPDATE CASCADE |

---

## 2. Commercial MVP Scope

### Product Thesis

```
SOLVENT
A portable authority layer for autonomous systems.
```

### Core Distinction

```
Evidence is not authority.
Agent output is not authority.
Retrieval is not authority.
An action is allowed only when Solvent can establish the required authority.
```

### MVP Workflow

```
UNTRUSTED / AI-GENERATED INFORMATION
            |
            v
      Solvent review
            |
            v
    evidence + obligations
            |
            v
     human decision
            |
            v
      explicit authority
            |
            v
    exact action authorization
            |
            v
     external execution
            |
            v
       audit ledger
```

### Nine Questions the MVP Answers

1. What does the agent believe?
2. What evidence supports it?
3. What remains unresolved?
4. Who is allowed to approve it?
5. Which action is being requested?
6. Which target does that action affect?
7. Why was the action allowed or denied?
8. What authority existed at the time?
9. What happened afterwards?

---

## 3. Architectural Decision: Kernel Growth Gate

Before ANY kernel/schema change, require an ADR containing:

- Problem
- Why product/service layer cannot solve it
- Why adapter cannot solve it
- Why policy/configuration cannot solve it
- New durable security fact
- New atomic transition
- Required DB invariant
- Impact on existing authority semantics
- Impact on Lean model
- Migration strategy
- Backward compatibility
- Security argument

**Default: DO NOT MODIFY THE KERNEL.**

---

## 4. Package Structure (Target State)

```
solvent/
├── kernel/                          # FROZEN — invariant enforcement
│   ├── kernel.go                    # Belief lifecycle
│   ├── authority.go                 # Authority lifecycle
│   ├── contract.go                  # Interface contract
│   ├── errors.go                    # Sentinel errors
│   ├── sql.go                       # SQL statements
│   └── doc.go                       # Package documentation
│
├── service/                         # NEW — business logic layer
│   ├── workflow/
│   │   ├── machine.go               # Workflow state machine
│   │   ├── transition.go            # Transition table
│   │   └── machine_test.go
│   ├── policy/
│   │   ├── registry.go              # Tool/actor policy registry
│   │   ├── evaluate.go              # Policy evaluation
│   │   └── registry_test.go
│   ├── evidence/
│   │   ├── quality.go               # Evidence quality projection
│   │   ├── scoring.go               # Transparent scoring
│   │   └── quality_test.go
│   ├── audit/
│   │   ├── ledger.go                # Activity ledger
│   │   ├── query.go                 # Ledger queries
│   │   └── ledger_test.go
│   ├── authority/
│   │   ├── service.go               # Authority service
│   │   └── service_test.go
│   └── executor/
│       ├── executor.go              # Executor interface
│       └── mock.go                  # Mock executor
│
├── adapter/                         # NEW — pluggable adapters
│   ├── port/
│   │   ├── evidence.go              # EvidenceSource interface
│   │   ├── document.go              # DocumentGenerator interface
│   │   ├── signature.go             # SignatureProvider interface
│   │   ├── executor.go              # Executor interface
│   │   └── ledger.go                # ActivityLedger interface
│   ├── bedrock/                     # Amazon Bedrock adapter
│   ├── agentjacking/                # Agentjacking adapter (existing)
│   ├── github/                      # GitHub adapter (first real integration)
│   └── mock/                        # Mock adapters for testing
│
├── web/                             # NEW — operational console
│   ├── server.go                    # HTTP server
│   ├── handlers/
│   │   ├── overview.go              # Dashboard
│   │   ├── review.go                # Review queue
│   │   ├── authority.go             # Authority detail
│   │   ├── audit.go                 # Audit trail
│   │   ├── integrations.go          # Adapter status
│   │   └── settings.go              # Configuration
│   ├── middleware/
│   │   ├── auth.go                  # Authentication boundary
│   │   └── logging.go               # Request logging
│   ├── templates/                   # HTML templates
│   └── static/                      # CSS, JS
│
├── scenario/                        # NEW — demo platform
│   ├── framework.go                 # Scenario runner
│   ├── agentjacking.go              # Scenario A
│   ├── lying_agent.go               # Scenario B
│   ├── stale_auth.go                # Scenario C
│   ├── confused_deputy.go           # Scenario D
│   ├── legitimate.go                # Scenario E
│   └── inject.go                    # Failure injection
│
├── db/                              # EXISTING — migrations
│   ├── 001_schema.sql
│   ├── 002_corpus.sql
│   ├── 003_wizard.sql
│   ├── 004_debt_vocabulary.sql
│   ├── 005_authority_mvp.sql
│   ├── 006_authority_justification_cascade.sql
│   └── 007_workflow.sql             # NEW — workflow tables (if needed)
│
├── cmd/
│   ├── solvent/                     # CLI entry point
│   ├── solvent-mcp/                 # MCP server (existing)
│   └── solvent-web/                 # Web server
│
├── internal/                        # EXISTING — internal packages
│   ├── belief/
│   ├── corpus/
│   ├── derive/
│   ├── normalize/
│   ├── pipeline/
│   ├── intent/
│   ├── wizard/
│   ├── view/
│   └── testdb/
│
├── formal/lean/                     # EXISTING — formal verification
├── proof/                           # EXISTING — control experiments
├── docs/                            # Documentation
├── examples/                        # NEW — example adapters
├── demos/                           # NEW — demo scenarios
└── test/                            # NEW — end-to-end tests
    ├── challenges/                  # Security challenge suite
    └── scenarios/                   # Scenario tests
```

---

## 5. Service Layer Design

### 5.1 Workflow Service

```go
// service/workflow/machine.go

type WorkflowState string

const (
    StateInvestigating    WorkflowState = "INVESTIGATING"
    StateEvidenceReview   WorkflowState = "EVIDENCE_REVIEW"
    StateHumanReview      WorkflowState = "HUMAN_REVIEW"
    StateApproved         WorkflowState = "APPROVED"
    StateAuthReady        WorkflowState = "AUTHORIZATION_READY"
    StateExecution        WorkflowState = "EXECUTION"
    StateCompleted        WorkflowState = "COMPLETED"
    StateRejected         WorkflowState = "REJECTED"
    StateCancelled        WorkflowState = "CANCELLED"
)

type ActorType string

const (
    ActorAgent  ActorType = "AGENT"
    ActorHuman  ActorType = "HUMAN"
    ActorSystem ActorType = "SYSTEM"
)

type TransitionRequest struct {
    From       WorkflowState
    To         WorkflowState
    Actor      ActorType
    BeliefID   string
    ScenarioID string
    Context    map[string]interface{}
}

type TransitionResult struct {
    Allowed    bool
    Reason     string
    Findings   []Finding
    NewState   WorkflowState
}

// CanTransition checks if a transition is allowed.
func CanTransition(req TransitionRequest) TransitionResult {
    // Centralized transition table
    // Actor restrictions enforced here
    // Policy evaluation called here
}
```

### 5.2 Policy Registry

```go
// service/policy/registry.go

type RiskClass string

const (
    RiskReadOnly     RiskClass = "READ_ONLY"
    RiskReversible   RiskClass = "REVERSIBLE"
    RiskIrreversible RiskClass = "IRREVERSIBLE"
)

type ToolPolicy struct {
    ID                     string
    RiskClass              RiskClass
    AllowedActors          []ActorType
    RequiredStage          WorkflowState
    RequiresHumanConfirm   bool
    RequiredAuthority      bool
}

type PolicyRegistry struct {
    tools map[string]ToolPolicy
}

// Register adds a tool policy.
func (r *PolicyRegistry) Register(policy ToolPolicy)

// Evaluate checks if an action is allowed.
func (r *PolicyRegistry) Evaluate(toolID string, actor ActorType, stage WorkflowState) EvaluationResult
```

### 5.3 Evidence Quality Projection

```go
// service/evidence/quality.go

type EvidenceStatus string

const (
    StatusVerified   EvidenceStatus = "VERIFIED"
    StatusUnverified EvidenceStatus = "UNVERIFIED"
    StatusConflict   EvidenceStatus = "CONFLICT"
    StatusStale      EvidenceStatus = "STALE"
    StatusMissing    EvidenceStatus = "MISSING"
)

type EvidenceQuality struct {
    BeliefID       string
    Status         EvidenceStatus
    Confidence     float64
    Debt           []string
    Contradictions []string
    Provenance     []ProvenanceRef
    Freshness      time.Duration
    Findings       []Finding
}

// Evaluate computes evidence quality from kernel state.
func Evaluate(ctx context.Context, store *kernel.Store, beliefID string) (EvidenceQuality, error)
```

### 5.4 Activity Ledger

```go
// service/audit/ledger.go

type Event struct {
    ID          string
    ScenarioID  string
    Timestamp   time.Time
    Actor       ActorType
    Operation   string
    Service     string
    BeliefID    *string
    IntentID    *string
    Target      *string
    Result      string
    Authority   *AuthorityRef
    Evidence    *EvidenceRef
    External    *ExternalRef
    Metadata    map[string]interface{}
}

type AuthorityRef struct {
    TargetID   string
    SnapshotID string
    ApprovedAt time.Time
}

type ExternalRef struct {
    Provider  string
    Operation string
    Request   interface{}
    Response  interface{}
    Mode      string // "LIVE", "LOCAL", "DEMO"
    Status    string // "ok", "fallback", "error"
    Duration  time.Duration
}

type Ledger struct {
    store *sql.DB
}

// Record appends an event.
func (l *Ledger) Record(ctx context.Context, event Event) error

// Query retrieves events with filters.
func (l *Ledger) Query(ctx context.Context, filter QueryFilter) ([]Event, error)
```

### 5.5 Executor Interface

```go
// service/executor/executor.go

type ExecutionRequest struct {
    AuthorizationID string
    BeliefID        string
    Action          string
    Target          string
    Authority       kernel.AuthorityTuple
    Parameters      map[string]interface{}
}

type ExecutionResult struct {
    Success  bool
    Output   interface{}
    Error    error
    Receipt  string // Transaction receipt for audit
}

type Executor interface {
    Name() string
    Execute(ctx context.Context, req ExecutionRequest) (ExecutionResult, error)
    Validate(ctx context.Context, req ExecutionRequest) ([]Finding, error)
}
```

---

## 6. Adapter Framework

### 6.1 Port Interfaces

```go
// adapter/port/evidence.go

type EvidenceSource interface {
    Name() string
    Fetch(ctx context.Context, query EvidenceQuery) ([]EvidenceCandidate, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}

type EvidenceQuery struct {
    Text        string
    MaxHits     int
    MaxDistance float64
}

type EvidenceCandidate struct {
    SourceID  string
    Title     string
    Body      string
    URL       string
    Distance  float64
    Metadata  map[string]string
}
```

```go
// adapter/port/document.go

type DocumentGenerator interface {
    Generate(ctx context.Context, request GenerateRequest) (GeneratedDocument, error)
    Validate(ctx context.Context, request GenerateRequest) ([]Finding, error)
}

type GenerateRequest struct {
    TemplateID string
    Data       map[string]interface{}
    Format     string
}

type GeneratedDocument struct {
    ID     string
    URI    string
    Format string
    Size   int64
    Hash   string
}
```

```go
// adapter/port/signature.go

type SignatureProvider interface {
    CreateDraft(ctx context.Context, request DraftRequest) (SignatureEnvelope, error)
    Send(ctx context.Context, envelopeID string) error
    Status(ctx context.Context, envelopeID string) (SignatureStatus, error)
    Download(ctx context.Context, envelopeID string, documentID string) ([]byte, error)
}
```

### 6.2 Adapter Registration

```go
// adapter/registry.go

type AdapterRegistry struct {
    evidence   map[string]EvidenceSource
    documents  map[string]DocumentGenerator
    signatures map[string]SignatureProvider
    executors  map[string]Executor
    ledgers    map[string]ActivityLedger
}

func NewRegistry() *AdapterRegistry

func (r *AdapterRegistry) RegisterEvidence(name string, src EvidenceSource)
func (r *AdapterRegistry) RegisterDocument(name string, gen DocumentGenerator)
func (r *AdapterRegistry) RegisterSignature(name string, sig SignatureProvider)
func (r *AdapterRegistry) RegisterExecutor(name string, exec Executor)
func (r *AdapterRegistry) RegisterLedger(name string, led ActivityLedger)

func (r *AdapterRegistry) GetEvidence(name string) (EvidenceSource, bool)
func (r *AdapterRegistry) GetDocument(name string) (DocumentGenerator, bool)
func (r *AdapterRegistry) GetSignature(name string) (SignatureProvider, bool)
func (r *AdapterRegistry) GetExecutor(name string) (Executor, bool)
func (r *AdapterRegistry) GetLedger(name string) (ActivityLedger, bool)
```

---

## 7. Workflow Token (HMAC-Sealed)

```go
// service/workflow/token.go

type WorkflowToken struct {
    Version    int            `json:"v"`
    ScenarioID string         `json:"sid"`
    BeliefID   string         `json:"bid"`
    Stage      WorkflowState  `json:"stage"`
    Evidence   []EvidenceRef  `json:"evidence,omitempty"`
    Debt       []string       `json:"debt,omitempty"`
    Authority  *AuthorityRef  `json:"authority,omitempty"`
    CreatedAt  time.Time      `json:"cat"`
    ExpiresAt  *time.Time     `json:"eat,omitempty"`
}

type EvidenceRef struct {
    ID       string  `json:"id"`
    Source   string  `json:"source"`
    Distance float64 `json:"distance"`
}

type TokenService struct {
    secret []byte
}

func NewTokenService(secret string) *TokenService

// Seal encodes and signs the workflow state.
func (s *TokenService) Seal(state WorkflowToken) (string, error)

// Open validates and decodes the token.
func (s *TokenService) Open(token string) (WorkflowToken, error)

// Refresh creates a new token with updated state.
func (s *TokenService) Refresh(oldToken string, updates map[string]interface{}) (string, error)
```

---

## 8. Preparation Boundary

```go
// service/preparation/prepare.go

type PreparationResult struct {
    Status     string // "ready", "review", "blocked"
    CanProceed bool
    Belief     kernel.Belief
    Quality    EvidenceQuality
    Findings   []Finding
}

// PrepareForAction re-validates before external operations.
func PrepareForAction(ctx context.Context, store *kernel.Store, scenarioID, beliefID string) (PreparationResult, error) {
    // 1. Read the belief
    // 2. Evaluate evidence quality
    // 3. Check debt status
    // 4. Check contradictions
    // 5. Verify authority
    // 6. Run policy evaluation
    // 7. Return preparation result
}
```

---

## 9. Web UI (5 Views)

### 9.1 Overview Dashboard

```
┌─────────────────────────────────────────────────────────┐
│ SOLVENT — Portable Authority Layer                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Active Workflows: 12    Pending Reviews: 3             │
│  Blocked Actions: 1      Recent Decisions: 8            │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ INVESTIGATING│  │ HUMAN_REVIEW │  │ APPROVED     │  │
│  │      5       │  │      3       │  │      4       │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│                                                         │
│  Recent Activity                                        │
│  ─────────────────────────────────────────────────────  │
│  14:32  AGENT    evidence.ingested    INC-1042         │
│  14:31  HUMAN    belief.promoted      INC-1042         │
│  14:30  SYSTEM   action.intent        INC-1042         │
│  14:29  HUMAN    authority.approved   INC-1042         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 9.2 Review Queue

```
┌─────────────────────────────────────────────────────────┐
│ REVIEW QUEUE                                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │ INC-1042 — Deploy etcd v3.5.x to production     │    │
│  │ Status: HUMAN_REVIEW                            │    │
│  │ Evidence: 3 verified, 1 conflict, 2 pending     │    │
│  │ Open Debt: needOperatorSignoff                  │    │
│  │ Requested Action: kubectl apply -f etcd.yaml    │    │
│  │ Target: cluster/production/etcd                 │    │
│  │                                                 │    │
│  │ [REVIEW] [APPROVE] [REJECT]                     │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │ INC-1043 — Update nginx config                  │    │
│  │ Status: EVIDENCE_REVIEW                         │    │
│  │ Evidence: 1 verified, 0 conflict, 4 pending     │    │
│  │ Open Debt: needProvenanceCheck, needBlastRadius │    │
│  │                                                 │    │
│  │ [VIEW EVIDENCE] [ADD EVIDENCE]                  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 9.3 Authority Detail

```
┌─────────────────────────────────────────────────────────┐
│ AUTHORITY DETAIL — INC-1042                             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  BELIEF                                                 │
│  Claim: "etcd v3.5.x is safe to deploy"                │
│  Status: PROMOTED                                       │
│  Debt: EMPTY                                            │
│  Evidence: 3 verified, 1 conflict                       │
│                                                         │
│  EVIDENCE                                               │
│  ┌─────────────────────────────────────────────────┐    │
│  │ #19220 · 0.372424 — Deployment safety analysis  │    │
│  │ Status: VERIFIED · Distance: 0.372424           │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │ #13766 · 0.594920 — Known stability issue       │    │
│  │ Status: CONFLICT · Distance: 0.594920           │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  AUTHORITY                                              │
│  Target: cluster/production/etcd                        │
│  Action: kubectl apply                                  │
│  Status: ACTIVE                                         │
│  Approved: 2026-09-04T14:30:00Z                        │
│  Approver: admin@pithomlabs.com                         │
│                                                         │
│  DECISION                                               │
│  Result: AUTHORIZED                                     │
│  Reason: Belief promoted, authority active, target match│
│                                                         │
│  [EXECUTE] [REVOKE] [VIEW AUDIT]                        │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 9.4 Audit Trail

```
┌─────────────────────────────────────────────────────────┐
│ AUDIT TRAIL                                             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Filters: [Actor ▼] [Operation ▼] [Result ▼] [Date ▼]  │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │ 14:32:15  AGENT     evidence.ingested    OK     │    │
│  │           Source: bedrock-titan                  │    │
│  │           Belief: #19220                        │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ 14:31:42  HUMAN     belief.promoted      OK     │    │
│  │           Belief: #19220                        │    │
│  │           Debt: 0/6 items remaining             │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ 14:30:58  SYSTEM    action.intent        OK     │    │
│  │           Action: kubectl apply                 │    │
│  │           Target: cluster/production/etcd       │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ 14:30:01  HUMAN     authority.approved   OK     │    │
│  │           Target: cluster/production/etcd       │    │
│  │           Snapshot: snap-abc123                 │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ 14:29:45  SYSTEM    authority.created    OK     │    │
│  │           Target: cluster/production/etcd       │    │
│  │           Principal: admin@pithomlabs.com       │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 9.5 Integrations

```
┌─────────────────────────────────────────────────────────┐
│ INTEGRATIONS                                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │ bedrock-titan    LIVE    12ms avg    99.9% up   │    │
│  │ Last: 14:32:15  Embeddings: 1,234               │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │ github-actions   LIVE    245ms avg   100% up    │    │
│  │ Last: 14:30:01  Deploys: 3                      │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │ cockroachdb      LIVE    8ms avg     100% up    │    │
│  │ Last: 14:32:15  Queries: 45,678                 │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  Recent Calls                                           │
│  ─────────────────────────────────────────────────────  │
│  14:32:15  bedrock-titan  POST /embed   200  12ms     │
│  14:30:01  github-actions POST /deploy  200  245ms    │
│  14:29:45  cockroachdb    INSERT        200  8ms      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 10. Demo Scenarios

### Scenario A: Agentjacking

```
Agent submits poisoned evidence.
Solvent enters belief with full debt.
Agent attempts to promote — DENIED (debt not empty).
Human reviews evidence — finds poisoning.
Human retracts belief — cascade retraction.
```

### Scenario B: Lying Agent

```
Agent claims "etcd is safe" without evidence.
Solvent enters belief — status: ENTERED.
Agent attempts to promote — DENIED (no evidence, full debt).
Human inspects — no supporting evidence found.
Human rejects — belief retracted.
```

### Scenario C: Stale Authorization

```
Authority approved for deploy.
Time passes — authority expires or is revoked.
Agent attempts to execute — DENIED (authority revoked).
Human inspects — sees revoked authority in audit trail.
Human re-approves — new authority created.
```

### Scenario D: Confused Deputy

```
Authority exists for cluster A.
Agent attempts to deploy to cluster B — DENIED (target mismatch).
Human inspects — sees correct authority, wrong target.
Agent corrects target — now matches authority.
Human approves — execution proceeds.
```

### Scenario E: Legitimate Workflow

```
Agent submits evidence.
Human reviews — evidence verified.
Human retires debt items.
Agent promotes belief — SUCCESS (debt empty).
Human approves authority.
Agent executes action — SUCCESS.
Audit trail shows complete flow.
```

---

## 11. Implementation Phases

### Phase 1: Service Boundaries (Week 1-2)

- [ ] Define service interfaces (WorkflowService, PolicyService, EvidenceService, AuditService, ExecutorService)
- [ ] Implement workflow state machine with transition table
- [ ] Implement tool/actor policy registry
- [ ] Implement evidence quality projection
- [ ] Implement activity ledger
- [ ] Write service-level tests

### Phase 2: Adapter Framework (Week 2-3)

- [ ] Define port interfaces (EvidenceSource, DocumentGenerator, SignatureProvider, Executor)
- [ ] Implement adapter registry
- [ ] Refactor existing Bedrock integration as adapter
- [ ] Refactor existing Agentjacking integration as adapter
- [ ] Implement mock adapters for testing
- [ ] Write adapter-level tests

### Phase 3: Web UI (Week 3-4)

- [ ] Implement HTTP server with middleware
- [ ] Implement Overview dashboard
- [ ] Implement Review queue
- [ ] Implement Authority detail view
- [ ] Implement Audit trail view
- [ ] Implement Integrations view
- [ ] Write UI-level tests

### Phase 4: Human Authorization Boundary (Week 4-5)

- [ ] Implement preparation boundary
- [ ] Implement workflow tokens (HMAC-sealed)
- [ ] Implement human confirmation flow
- [ ] Implement authority re-validation before execution
- [ ] Write security-sensitive tests

### Phase 5: Executor Framework (Week 5-6)

- [ ] Implement executor interface
- [ ] Implement GitHub Actions executor (first real integration)
- [ ] Implement mock executor
- [ ] Write executor tests
- [ ] Write end-to-end scenario tests

### Phase 6: Demo Platform (Week 6-7)

- [ ] Implement scenario framework
- [ ] Implement Scenario A: Agentjacking
- [ ] Implement Scenario B: Lying Agent
- [ ] Implement Scenario C: Stale Authorization
- [ ] Implement Scenario D: Confused Deputy
- [ ] Implement Scenario E: Legitimate Workflow
- [ ] Implement failure injection
- [ ] Write scenario tests

### Phase 7: Open Source Polish (Week 7-8)

- [ ] Write developer documentation
- [ ] Write adapter development guide
- [ ] Write security challenge suite
- [ ] Write architecture/security docs
- [ ] Write commercial demo walkthrough
- [ ] Write quickstart guide

---

## 12. Testing Strategy

### 12.1 Kernel Tests (Existing)

Preserve all existing kernel tests. These verify:
- Belief lifecycle invariants
- Authority lifecycle invariants
- DB-enforced constraints
- Cascade retraction correctness

### 12.2 Service Tests (New)

```go
// service/workflow/machine_test.go
func TestCanTransition(t *testing.T) {
    // Test valid transitions
    // Test invalid transitions
    // Test actor restrictions
    // Test stage requirements
}

// service/policy/registry_test.go
func TestEvaluate(t *testing.T) {
    // Test allowed actions
    // Test blocked actions
    // Test human-only actions
    // Test irreversible actions
}

// service/evidence/quality_test.go
func TestEvaluate(t *testing.T) {
    // Test verified evidence
    // Test conflicting evidence
    // Test stale evidence
    // Test missing evidence
}
```

### 12.3 Adapter Tests (New)

```go
// adapter/bedrock/adapter_test.go
func TestFetch(t *testing.T) {
    // Test successful fetch
    // Test provider failure
    // Test malformed response
    // Test timeout
}
```

### 12.4 Security Tests (New)

```go
// test/challenges/bypass_test.go
func TestAgentCannotPromote(t *testing.T) {
    // Agent attempts to promote without retiring debt
    // Expected: DENIED
}

func TestAgentCannotApprove(t *testing.T) {
    // Agent attempts to approve authority
    // Expected: DENIED
}

func TestWrongTargetDenied(t *testing.T) {
    // Authority for target A, action on target B
    // Expected: DENIED
}

func TestRevokedAuthorityDenied(t *testing.T) {
    // Execute after authority revoked
    // Expected: DENIED
}

func TestStaleTokenRejected(t *testing.T) {
    // Submit expired workflow token
    // Expected: REJECTED
}
```

### 12.5 End-to-End Tests (New)

```go
// test/scenarios/agentjacking_test.go
func TestAgentjackingScenario(t *testing.T) {
    // Full scenario: agent submits poisoned evidence
    // Human reviews, finds poisoning, retracts
    // Verify audit trail
}
```

---

## 13. Acceptance Criteria

### Architecture

- [ ] Kernel remains small and domain-generic
- [ ] Product behavior lives outside the kernel
- [ ] MCP remains thin
- [ ] External systems are adapters
- [ ] Execution is outside the kernel
- [ ] Service layer owns orchestration/policy

### Security

- [ ] Actor/tool restrictions are explicit
- [ ] Human-only actions are mechanically gated
- [ ] Agent claims cannot become authority
- [ ] Wrong target cannot use correct authority
- [ ] Revoked/stale authority cannot authorize
- [ ] Browser/workflow state cannot become authority
- [ ] External execution cannot create authority

### Product

- [ ] Operator can view pending decisions
- [ ] Operator can inspect evidence
- [ ] Operator can see why something is blocked
- [ ] Operator can approve legitimate actions
- [ ] Operator can inspect authority
- [ ] Operator can inspect audit history
- [ ] Operator can see external execution status

### Demo

- [ ] Agentjacking demo works
- [ ] Lying Agent demo works
- [ ] Stale Authorization demo works
- [ ] Confused Deputy demo works
- [ ] Legitimate approval demo works
- [ ] All scenarios reset independently

### Open Source

- [ ] New developer can add an adapter without touching kernel code
- [ ] New customer policy does not require kernel modification
- [ ] New executor does not require kernel modification
- [ ] Security challenges are reproducible

### Quality

- [ ] Existing tests continue to pass
- [ ] New security invariants have bypass tests
- [ ] Lean verification remains clean
- [ ] No unexplained kernel/schema growth
- [ ] Documentation reflects actual implementation

---

## 14. Deliverables

1. Implementation (code)
2. Architecture diagram
3. ADRs for any kernel/schema changes
4. Commercial MVP README
5. Security model
6. Adapter development guide
7. Workflow documentation
8. Demo guide
9. Test report
10. "What is Solvent?" explanation for buyers
11. "Why is Solvent different?" explanation for security engineers
12. Implementation summary (files added/changed, migrations, kernel changes, service additions, adapter additions, UI additions, test coverage, remaining limitations)

---

## 15. Guiding Principles

1. **Small trusted core** — Keep the kernel frozen
2. **Clear boundaries** — Adapters → Service/Policy → Kernel → DB
3. **Commercially useful workflow** — One coherent end-to-end flow
4. **Visible security guarantees** — Show the user WHY something is allowed/denied
5. **Easy integration** — Clean port interfaces
6. **Excellent demos** — Five deterministic, resettable scenarios
7. **Maintainability** — Service layer owns business logic
8. **Open-source extensibility** — Adapter SDK, examples, challenge suite

---

## 16. What NOT to Build Now

Explicitly defer:

- Full multi-tenancy in kernel
- Generalized policy programming language
- Universal workflow DSL
- Dozens of integrations
- Deep IAM system
- Custom identity provider
- Cryptographic attestation infrastructure
- Global replay-prevention machinery
- Execution-effect verification framework
- Complex distributed scheduler
- Elaborate document/signature platform
- Large analytics platform
- Generic AI risk-scoring laboratory

These may become future products/features. They are not requirements for the MVP.
