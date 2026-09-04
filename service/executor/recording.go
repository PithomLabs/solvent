package executor

import (
	"context"
	"sync"
)

// RecordingFunc is a test-only executor that records whether it was called
// and with what parameters. It never produces real side effects.
//
// Usage in integration tests:
//
//	reg := executor.NewRegistry()
//	rec := executor.NewRecordingFunc("deploy", "etcd deployed")
//	reg.Register("deploy", rec.Func())
//	// ... call ExecuteAction ...
//	if rec.Called() { t.Error("executor must not be called on DENIED") }
type RecordingFunc struct {
	mu     sync.Mutex
	name   string
	output string
	called bool
	params map[string]interface{}
}

// NewRecordingFunc creates a recording executor with the given name and output.
func NewRecordingFunc(name, output string) *RecordingFunc {
	return &RecordingFunc{name: name, output: output}
}

// Func returns an ActionFunc that records the call.
func (r *RecordingFunc) Func() ActionFunc {
	return func(ctx context.Context, params map[string]interface{}) (string, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.called = true
		r.params = params
		return r.output, nil
	}
}

// Called reports whether the executor was invoked.
func (r *RecordingFunc) Called() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.called
}

// Params returns the parameters passed to the executor.
func (r *RecordingFunc) Params() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.params
}

// Reset clears the called state for reuse across test cases.
func (r *RecordingFunc) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.called = false
	r.params = nil
}
