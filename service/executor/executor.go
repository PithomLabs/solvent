// Package executor implements the Executor contract: the actual work that happens
// when a prepared workflow token is executed.
//
// The executor is the boundary between the authority system and the outside world.
// It receives a prepared token, performs the action, and reports the outcome.
//
// Key properties:
//   - One token, one executor call, one outcome.
//   - The executor re-validates belief state before acting (already done by Prepare).
//   - The executor reports success or failure; the workflow layer handles state.
//   - The executor never modifies belief state directly.
package executor

import (
	"context"
	"fmt"
)

// ActionFunc is the function signature for executable actions.
type ActionFunc func(ctx context.Context, params map[string]interface{}) (string, error)

// Registry maps action type names to their implementation.
type Registry struct {
	actions map[string]ActionFunc
}

// NewRegistry creates an empty executor registry.
func NewRegistry() *Registry {
	return &Registry{
		actions: make(map[string]ActionFunc),
	}
}

// Register adds an action implementation to the registry.
func (r *Registry) Register(name string, fn ActionFunc) {
	r.actions[name] = fn
}

// Get retrieves an action implementation by name.
func (r *Registry) Get(name string) (ActionFunc, bool) {
	fn, ok := r.actions[name]
	return fn, ok
}

// ExecuteAction runs a named action with the given parameters.
func ExecuteAction(ctx context.Context, registry *Registry, actionName string, params map[string]interface{}) (string, error) {
	fn, ok := registry.Get(actionName)
	if !ok {
		return "", fmt.Errorf("action %q not registered", actionName)
	}
	return fn(ctx, params)
}
