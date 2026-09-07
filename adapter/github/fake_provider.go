package github

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FakeGitHubProvider is a test double for GitHubProvider.
// It records all calls and can simulate acceptance, rejection, error, or lost response.
type FakeGitHubProvider struct {
	mu           sync.Mutex
	calls        []WorkflowCall
	accept       bool
	err          error
	runID        string
	lostResponse bool
	callWg       sync.WaitGroup
}

// WorkflowCall records one TriggerWorkflow invocation.
type WorkflowCall struct {
	Repo     string
	Workflow string
	Ref      string
	Inputs   map[string]string
	CalledAt time.Time
}

// NewFakeGitHubProvider creates a new fake provider.
// If accept is true, TriggerWorkflow returns the given runID.
// If accept is false, TriggerWorkflow returns an error.
func NewFakeGitHubProvider(accept bool, runID string) *FakeGitHubProvider {
	return &FakeGitHubProvider{
		accept: accept,
		runID:  runID,
	}
}

// TriggerWorkflow records the call and returns based on configured behavior.
func (f *FakeGitHubProvider) TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
	inputs map[string]string) (*WorkflowResult, error) {

	call := WorkflowCall{
		Repo:     repo,
		Workflow: workflow,
		Ref:      ref,
		Inputs:   inputs,
		CalledAt: time.Now(),
	}

	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.callWg.Add(1)
	f.mu.Unlock()
	defer f.callWg.Done()

	if f.lostResponse {
		return nil, fmt.Errorf("response lost: provider accepted but response not received")
	}

	if !f.accept {
		if f.err != nil {
			return nil, f.err
		}
		return nil, fmt.Errorf("provider rejected the workflow trigger")
	}

	return &WorkflowResult{RunID: f.runID}, nil
}

// Calls returns a copy of all recorded workflow calls.
func (f *FakeGitHubProvider) Calls() []WorkflowCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]WorkflowCall, len(f.calls))
	copy(out, f.calls)
	return out
}

// CallCount returns the number of TriggerWorkflow calls.
func (f *FakeGitHubProvider) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// LastCall returns the most recent call, or nil if no calls have been made.
func (f *FakeGitHubProvider) LastCall() *WorkflowCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return nil
	}
	call := f.calls[len(f.calls)-1]
	return &call
}

// SetAccept changes the acceptance behavior mid-test.
func (f *FakeGitHubProvider) SetAccept(accept bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accept = accept
}

// SetErr sets a custom error to return when accept is false.
func (f *FakeGitHubProvider) SetErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// SetLostResponse enables or disables lost-response simulation.
// When enabled, the provider records the call but returns an error,
// simulating response loss after the provider accepted.
func (f *FakeGitHubProvider) SetLostResponse(lost bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lostResponse = lost
}

// WaitForCalls blocks until at least n calls have been recorded,
// or until the timeout expires.
func (f *FakeGitHubProvider) WaitForCalls(n int, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		for {
			f.mu.Lock()
			count := len(f.calls)
			f.mu.Unlock()
			if count >= n {
				close(done)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		f.mu.Lock()
		count := len(f.calls)
		f.mu.Unlock()
		return fmt.Errorf("waited for %d calls, only got %d after %v", n, count, timeout)
	}
}
