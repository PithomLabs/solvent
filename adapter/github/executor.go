package github

import (
	"context"
	"fmt"

	"github.com/PithomLabs/solvent/service/executor"
)

// ExecutorName is the registered name for the GitHub workflow executor.
const ExecutorName = "github_trigger_workflow"

// NewExecutor creates an ActionFunc that triggers a GitHub Actions workflow.
// It reads repo/workflow/ref from params (provided by the service from snapshot).
// It does NOT know about snapshots or authorization.
func NewExecutor(provider GitHubProvider) executor.ActionFunc {
	return func(ctx context.Context, params map[string]interface{}) (string, error) {
		repo, _ := params["repo"].(string)
		workflow, _ := params["workflow"].(string)
		ref, _ := params["ref"].(string)

		if repo == "" || workflow == "" || ref == "" {
			return "", fmt.Errorf("missing required params: repo=%q workflow=%q ref=%q", repo, workflow, ref)
		}

		inputs := make(map[string]string)
		if rawInputs, ok := params["inputs"].(map[string]interface{}); ok {
			for k, v := range rawInputs {
				if s, ok := v.(string); ok {
					inputs[k] = s
				}
			}
		}

		result, err := provider.TriggerWorkflow(ctx, repo, workflow, ref, inputs)
		if err != nil {
			return "", err
		}

		return result.RunID, nil
	}
}

// RegisterExecutor registers the GitHub workflow executor in the given registry.
func RegisterExecutor(reg *executor.Registry, provider GitHubProvider) {
	reg.Register(ExecutorName, NewExecutor(provider))
}
