package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrGitHubAPI is a sentinel for GitHub API errors, enabling errors.Is classification.
var ErrGitHubAPI = errors.New("GitHub API error")

const maxErrorBodyLen = 1024

// truncateError limits the length of an error body for audit-visible errors.
func truncateError(body []byte) string {
	if len(body) > maxErrorBodyLen {
		return string(body[:maxErrorBodyLen]) + "... (truncated)"
	}
	return string(body)
}

// HTTPProvider implements GitHubProvider by calling the GitHub Actions API.
// It triggers workflow_dispatch events via the REST API.
type HTTPProvider struct {
	token      string
	httpClient *http.Client
}

// HTTPProviderOption configures an HTTPProvider.
type HTTPProviderOption func(*HTTPProvider)

// WithTimeout sets the HTTP client timeout. The default is 30s.
// Context cancellation remains supported; the client timeout is a safety bound.
func WithTimeout(d time.Duration) HTTPProviderOption {
	return func(p *HTTPProvider) {
		p.httpClient.Timeout = d
	}
}

// NewHTTPProvider creates a new HTTP-backed GitHub provider.
// The token is a GitHub personal access token with "repo" scope.
func NewHTTPProvider(token string, opts ...HTTPProviderOption) *HTTPProvider {
	p := &HTTPProvider{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// workflowDispatchRequest is the request body for the GitHub workflow dispatch API.
type workflowDispatchRequest struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs,omitempty"`
}

// classifyGitHubError maps a GitHub HTTP response to a ProviderOutcome.
// Network errors, timeouts, and 5xx are ambiguous (provider may have accepted).
// 4xx (except 408) are definitive rejections. 408 is ambiguous.
func classifyGitHubError(statusCode int, err error) *ProviderError {
	if err != nil {
		return &ProviderError{Err: err, Outcome: ProviderAmbiguous}
	}
	switch {
	case statusCode >= 200 && statusCode < 300:
		return &ProviderError{Err: fmt.Errorf("unexpected success in error path"), Outcome: ProviderAccepted}
	case statusCode == 408:
		return &ProviderError{Err: fmt.Errorf("GitHub API returned %d", statusCode), Outcome: ProviderAmbiguous}
	case statusCode >= 400 && statusCode < 500:
		return &ProviderError{Err: fmt.Errorf("%w %d", ErrGitHubAPI, statusCode), Outcome: ProviderRejected}
	case statusCode >= 500:
		return &ProviderError{Err: fmt.Errorf("GitHub API returned %d", statusCode), Outcome: ProviderAmbiguous}
	default:
		return &ProviderError{Err: fmt.Errorf("GitHub API returned %d", statusCode), Outcome: ProviderAmbiguous}
	}
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
		return nil, &ProviderError{Err: err, Outcome: ProviderAmbiguous}
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
			return nil, &ProviderError{Err: fmt.Errorf("read response body: %w", err), Outcome: ProviderAmbiguous}
		}
		if err := json.Unmarshal(respBody, &result); err == nil && result.WorkflowRun.ID != 0 {
			return &WorkflowResult{
				RunID: fmt.Sprintf("%d", result.WorkflowRun.ID),
			}, nil
		}
	}

	// Any other status code is an error — classified by outcome.
	respBody, _ := io.ReadAll(resp.Body)
	pErr := classifyGitHubError(resp.StatusCode, nil)
	pErr.Err = fmt.Errorf("%w %d: %s", ErrGitHubAPI, resp.StatusCode, truncateError(respBody))
	return nil, pErr
}
