package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPProvider implements GitHubProvider by calling the GitHub Actions API.
// It triggers workflow_dispatch events via the REST API.
type HTTPProvider struct {
	token      string
	httpClient *http.Client
}

// NewHTTPProvider creates a new HTTP-backed GitHub provider.
// The token is a GitHub personal access token with "repo" scope.
func NewHTTPProvider(token string) *HTTPProvider {
	return &HTTPProvider{
		token:      token,
		httpClient: &http.Client{},
	}
}

// workflowDispatchRequest is the request body for the GitHub workflow dispatch API.
type workflowDispatchRequest struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs,omitempty"`
}

// TriggerWorkflow triggers a GitHub Actions workflow dispatch via the REST API.
func (h *HTTPProvider) TriggerWorkflow(ctx context.Context, repo, workflow, ref string,
	inputs map[string]string) (*WorkflowResult, error) {

	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}
	if workflow == "" {
		return nil, fmt.Errorf("workflow is required")
	}
	if ref == "" {
		return nil, fmt.Errorf("ref is required")
	}

	// POST /repos/{owner}/{repo}/actions/workflows/{workflow}/dispatches
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/workflows/%s/dispatches",
		strings.TrimPrefix(repo, "/"), workflow)

	reqBody := workflowDispatchRequest{
		Ref:    ref,
		Inputs: inputs,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// A 204 No Content means the dispatch was accepted.
	if resp.StatusCode == http.StatusNoContent {
		return &WorkflowResult{RunID: ""}, nil
	}

	// A 201 Created with workflow_run body means the dispatch created a run.
	if resp.StatusCode == http.StatusCreated {
		var result struct {
			WorkflowRun struct {
				ID   int64  `json:"id"`
				HTML string `json:"html_url"`
			} `json:"workflow_run"`
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}
		if err := json.Unmarshal(respBody, &result); err == nil && result.WorkflowRun.ID != 0 {
			return &WorkflowResult{
				RunID: fmt.Sprintf("%d", result.WorkflowRun.ID),
			}, nil
		}
	}

	// Any other status code is an error.
	respBody, _ := io.ReadAll(resp.Body)
	return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))
}
