package executor

import (
	"context"
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	called := false
	r.Register("test_action", func(ctx context.Context, params map[string]interface{}) (string, error) {
		called = true
		return "ok", nil
	})

	fn, ok := r.Get("test_action")
	if !ok {
		t.Fatal("Get returned false for registered action")
	}

	result, err := fn(context.Background(), map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("fn returned error: %v", err)
	}
	if result != "ok" {
		t.Errorf("fn returned %q, want %q", result, "ok")
	}
	if !called {
		t.Error("fn was not called")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get returned true for unregistered action")
	}
}

func TestExecuteAction(t *testing.T) {
	r := NewRegistry()
	r.Register("echo", func(ctx context.Context, params map[string]interface{}) (string, error) {
		return "echoed", nil
	})

	result, err := ExecuteAction(context.Background(), r, "echo", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteAction returned error: %v", err)
	}
	if result != "echoed" {
		t.Errorf("ExecuteAction returned %q, want %q", result, "echoed")
	}
}

func TestExecuteActionNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := ExecuteAction(context.Background(), r, "missing", map[string]interface{}{})
	if err == nil {
		t.Error("ExecuteAction should return error for unregistered action")
	}
}
