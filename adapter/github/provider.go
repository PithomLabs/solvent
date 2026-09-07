package github

import "context"

// GitHubProvider is the interface for triggering GitHub Actions workflows.
// Real implementations would call the GitHub API. Tests use FakeGitHubProvider.
type GitHubProvider interface {
	TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
		inputs map[string]string) (*WorkflowResult, error)
}

// WorkflowResult is the outcome of a successful workflow trigger.
type WorkflowResult struct {
	RunID string
}
