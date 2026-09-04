// Package policy implements the PolicyService: tool registry, actor registry,
// belief/tool compatibility, and action authorization policy.
//
// Policy decisions are constraints, NOT final authorization. The final
// authorization decision requires both policy constraints AND kernel.Authorize.
//
// Security invariant: policy may constrain authority but may never manufacture
// authority. policy ALLOW + authority ABSENT = DENIED.
package policy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ToolClass represents the three classes of tools.
type ToolClass string

const (
	ClassRead        ToolClass = "read"
	ClassMutate      ToolClass = "mutate"
	ClassOrchestrate ToolClass = "orchestrate"
)

// Tool represents a registered tool in the system.
type Tool struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Class           ToolClass `json:"class"`
	RequiredBeliefs []string  `json:"required_beliefs"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
}

// Actor represents a registered agent or user.
type Actor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Roles     []string  `json:"roles"`
	MaxClass  ToolClass `json:"max_class"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// PolicyConstraints is the output of policy evaluation. It is NOT a final
// authorization decision. The final decision requires kernel.Authorize.
//
// Policy may constrain authority. Policy may never manufacture authority.
type PolicyConstraints struct {
	ActorPermitted    bool     `json:"actor_permitted"`
	StagePermitted    bool     `json:"stage_permitted"`
	ToolPermitted     bool     `json:"tool_permitted"`
	RequiresHuman     bool     `json:"requires_human"`
	RequiresAuthority bool     `json:"requires_authority"`
	Reasons           []string `json:"reasons,omitempty"`
}

// IsSatisfied returns true if all policy constraints are met.
func (pc *PolicyConstraints) IsSatisfied() bool {
	return pc.ActorPermitted && pc.StagePermitted && pc.ToolPermitted
}

// Service manages tool and actor registries and policy evaluation.
type Service struct {
	db *sql.DB
}

// New creates a new policy Service.
func New(db *sql.DB) *Service {
	return &Service{db: db}
}

// RegisterTool adds a new tool to the registry.
func (s *Service) RegisterTool(ctx context.Context, name string, class ToolClass, requiredBeliefs []string, description string) (*Tool, error) {
	t := &Tool{
		ID:              uuid.New().String(),
		Name:            name,
		Class:           class,
		RequiredBeliefs: requiredBeliefs,
		Description:     description,
		CreatedAt:       time.Now(),
	}

	beliefsJSON, err := json.Marshal(requiredBeliefs)
	if err != nil {
		return nil, fmt.Errorf("marshal required beliefs: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO policy_tool (id, name, class, required_beliefs, description, created_at)
		VALUES ($1::UUID, $2, $3, $4::JSONB, $5, $6)`,
		t.ID, t.Name, t.Class, string(beliefsJSON), t.Description, t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("register tool: %w", err)
	}

	return t, nil
}

// GetTool retrieves a tool by name.
func (s *Service) GetTool(ctx context.Context, name string) (*Tool, error) {
	var t Tool
	var beliefsJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, class, required_beliefs, description, created_at
		FROM policy_tool WHERE name = $1`, name).Scan(
		&t.ID, &t.Name, &t.Class, &beliefsJSON, &t.Description, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("get tool: %w", err)
	}

	if err := json.Unmarshal(beliefsJSON, &t.RequiredBeliefs); err != nil {
		return nil, fmt.Errorf("unmarshal required beliefs: %w", err)
	}

	return &t, nil
}

// ListTools returns all registered tools.
func (s *Service) ListTools(ctx context.Context) ([]*Tool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, class, required_beliefs, description, created_at
		FROM policy_tool ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	defer rows.Close()

	var tools []*Tool
	for rows.Next() {
		var t Tool
		var beliefsJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Class, &beliefsJSON, &t.Description, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tool: %w", err)
		}
		if err := json.Unmarshal(beliefsJSON, &t.RequiredBeliefs); err != nil {
			return nil, fmt.Errorf("unmarshal required beliefs: %w", err)
		}
		tools = append(tools, &t)
	}
	return tools, rows.Err()
}

// RegisterActor adds a new actor to the registry.
func (s *Service) RegisterActor(ctx context.Context, name string, roles []string, maxClass ToolClass) (*Actor, error) {
	a := &Actor{
		ID:        uuid.New().String(),
		Name:      name,
		Roles:     roles,
		MaxClass:  maxClass,
		Active:    true,
		CreatedAt: time.Now(),
	}

	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return nil, fmt.Errorf("marshal roles: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO policy_actor (id, name, roles, max_class, active, created_at)
		VALUES ($1::UUID, $2, $3::JSONB, $4, $5, $6)`,
		a.ID, a.Name, string(rolesJSON), a.MaxClass, a.Active, a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("register actor: %w", err)
	}

	return a, nil
}

// GetActor retrieves an actor by ID.
func (s *Service) GetActor(ctx context.Context, actorID string) (*Actor, error) {
	var a Actor
	var rolesJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, roles, max_class, active, created_at
		FROM policy_actor WHERE id = $1::UUID`, actorID).Scan(
		&a.ID, &a.Name, &rolesJSON, &a.MaxClass, &a.Active, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("actor %s not found", actorID)
	}
	if err != nil {
		return nil, fmt.Errorf("get actor: %w", err)
	}

	if err := json.Unmarshal(rolesJSON, &a.Roles); err != nil {
		return nil, fmt.Errorf("unmarshal roles: %w", err)
	}

	return &a, nil
}

// DeactivateActor sets an actor as inactive.
func (s *Service) DeactivateActor(ctx context.Context, actorID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE policy_actor SET active = false WHERE id = $1::UUID`, actorID)
	if err != nil {
		return fmt.Errorf("deactivate actor: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("actor %s not found", actorID)
	}
	return nil
}

// EvaluateConstraints evaluates policy constraints for a tool call.
//
// Returns a PolicyConstraints record, NOT a final boolean authorization.
// The final decision requires kernel.Authorize in addition to policy.
//
// Policy may constrain authority. Policy may never manufacture authority.
func (s *Service) EvaluateConstraints(
	ctx context.Context,
	actorID, toolName string,
	promotedBeliefs []string,
) (*PolicyConstraints, error) {
	pc := &PolicyConstraints{
		ActorPermitted:    true,
		StagePermitted:    true,
		ToolPermitted:     true,
		RequiresAuthority: true,
	}

	tool, err := s.GetTool(ctx, toolName)
	if err != nil {
		pc.ToolPermitted = false
		pc.RequiresAuthority = false
		pc.Reasons = append(pc.Reasons, fmt.Sprintf("tool not found: %s", toolName))
		return pc, nil
	}

	actor, err := s.GetActor(ctx, actorID)
	if err != nil {
		pc.ActorPermitted = false
		pc.RequiresAuthority = false
		pc.Reasons = append(pc.Reasons, fmt.Sprintf("actor not found: %s", actorID))
		return pc, nil
	}

	if !actor.Active {
		pc.ActorPermitted = false
		pc.RequiresAuthority = false
		pc.Reasons = append(pc.Reasons, fmt.Sprintf("actor %s is deactivated", actorID))
		return pc, nil
	}

	if !classWithinLimits(tool.Class, actor.MaxClass) {
		pc.ToolPermitted = false
		pc.RequiresAuthority = false
		pc.Reasons = append(pc.Reasons, fmt.Sprintf("tool class %s exceeds actor %s max class %s",
			tool.Class, actorID, actor.MaxClass))
		return pc, nil
	}

	missing := missingBeliefs(tool.RequiredBeliefs, promotedBeliefs)
	if len(missing) > 0 {
		pc.ToolPermitted = false
		pc.Reasons = append(pc.Reasons, fmt.Sprintf("missing required beliefs: %v", missing))
	}

	return pc, nil
}

// classWithinLimits checks whether a tool class is within an actor's allowed class.
func classWithinLimits(toolClass, maxClass ToolClass) bool {
	levels := map[ToolClass]int{
		ClassRead:        0,
		ClassMutate:      1,
		ClassOrchestrate: 2,
	}
	return levels[toolClass] <= levels[maxClass]
}

// missingBeliefs returns the required beliefs not present in the promoted set.
func missingBeliefs(required, promoted []string) []string {
	promotedSet := make(map[string]bool, len(promoted))
	for _, p := range promoted {
		promotedSet[p] = true
	}
	var missing []string
	for _, r := range required {
		if !promotedSet[r] {
			missing = append(missing, r)
		}
	}
	return missing
}
