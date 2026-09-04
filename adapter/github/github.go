// Package github implements the GitHub adapter: the first real integration
// for Solvent's commercial MVP.
//
// This adapter maps the GitHub/CI-CD/deployment workflow to Solvent's
// belief lifecycle:
//
//   - GitHub events (push, PR, CI status, deployment) become evidence.
//   - Evidence is normalized and derived into beliefs.
//   - Beliefs go through the debt/promotion cycle.
//   - Actions (deploy, merge, rollback) are gated on promoted beliefs.
//
// The adapter is domain-specific but follows the same pattern as the
// agentjacking adapter: it emits only existing generic types and never
// touches the kernel directly.
package github

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// EventType classifies a GitHub event.
type EventType string

const (
	EventPush       EventType = "push"
	EventPR         EventType = "pull_request"
	EventCIStatus   EventType = "status"
	EventDeployment EventType = "deployment"
	EventIssue      EventType = "issue"
	EventComment    EventType = "issue_comment"
)

// Event is a normalized GitHub event.
type Event struct {
	Type        EventType         `json:"type"`
	Repository  string            `json:"repository"`
	Branch      string            `json:"branch"`
	SHA         string            `json:"sha"`
	Title       string            `json:"title,omitempty"`
	Body        string            `json:"body,omitempty"`
	State       string            `json:"state,omitempty"`
	Author      string            `json:"author"`
	Timestamp   time.Time         `json:"timestamp"`
	Labels      []string          `json:"labels,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Adapter converts GitHub events into evidence for the Solvent pipeline.
type Adapter struct {
	repoOwner string
	repoName  string
}

// New creates a new GitHub adapter for the given repository.
func New(repoOwner, repoName string) *Adapter {
	return &Adapter{
		repoOwner: repoOwner,
		repoName:  repoName,
	}
}

// ProcessEvent converts a GitHub event into normalized evidence.
//
// The adapter never creates beliefs directly — it only produces evidence.
// The pipeline layer handles belief creation and promotion.
func (a *Adapter) ProcessEvent(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	switch event.Type {
	case EventPush:
		return a.processPush(ctx, event)
	case EventPR:
		return a.processPR(ctx, event)
	case EventCIStatus:
		return a.processCIStatus(ctx, event)
	case EventDeployment:
		return a.processDeployment(ctx, event)
	case EventIssue:
		return a.processIssue(ctx, event)
	case EventComment:
		return a.processComment(ctx, event)
	default:
		return nil, fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// NormalizedEvidence is the output of the adapter: evidence ready for the pipeline.
type NormalizedEvidence struct {
	SourceType       string                 `json:"source_type"`
	Subject          string                 `json:"subject"`
	Assertion        string                 `json:"assertion"`
	ProvenanceClass  string                 `json:"provenance_class"`
	SourceURL        string                 `json:"source_url"`
	ContentSHA256    string                 `json:"content_sha256"`
	DomainPayload    map[string]interface{} `json:"domain_payload"`
}

func (a *Adapter) processPush(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	commitURL := fmt.Sprintf("https://github.com/%s/%s/commit/%s", a.repoOwner, a.repoName, event.SHA)

	payload := map[string]interface{}{
		"repository": event.Repository,
		"branch":     event.Branch,
		"sha":        event.SHA,
		"author":     event.Author,
	}

	// Hash the commit SHA for content tracking.
	hash := sha256.Sum256([]byte(event.SHA))
	contentHash := fmt.Sprintf("%x", hash)

	return &NormalizedEvidence{
		SourceType:      "github_push",
		Subject:         a.repoName,
		Assertion:       fmt.Sprintf("commit %s pushed to %s", truncateSHA(event.SHA), event.Branch),
		ProvenanceClass: "external_feed",
		SourceURL:       commitURL,
		ContentSHA256:   contentHash,
		DomainPayload:   payload,
	}, nil
}

func (a *Adapter) processPR(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	prURL := fmt.Sprintf("https://github.com/%s/%s/pull/%s", a.repoOwner, a.repoName, event.Metadata["number"])

	merged := event.State == "merged"
	action := event.Metadata["action"]

	payload := map[string]interface{}{
		"number":   event.Metadata["number"],
		"action":   action,
		"merged":   merged,
		"branch":   event.Branch,
		"author":   event.Author,
		"labels":   event.Labels,
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("pr-%s-%s", event.Metadata["number"], event.SHA)))
	contentHash := fmt.Sprintf("%x", hash)

	assertion := fmt.Sprintf("PR #%s %s by %s", event.Metadata["number"], action, event.Author)
	if merged {
		assertion = fmt.Sprintf("PR #%s merged into %s", event.Metadata["number"], event.Branch)
	}

	return &NormalizedEvidence{
		SourceType:      "github_pr",
		Subject:         a.repoName,
		Assertion:       assertion,
		ProvenanceClass: "external_feed",
		SourceURL:       prURL,
		ContentSHA256:   contentHash,
		DomainPayload:   payload,
	}, nil
}

func (a *Adapter) processCIStatus(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	statusURL := fmt.Sprintf("https://github.com/%s/%s/commit/%s/checks", a.repoOwner, a.repoName, event.SHA)

	payload := map[string]interface{}{
		"state":    event.State,
		"sha":      event.SHA,
		"context":  event.Metadata["context"],
		"author":   event.Author,
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("ci-%s-%s", event.SHA, event.State)))
	contentHash := fmt.Sprintf("%x", hash)

	assertion := fmt.Sprintf("CI status %s for commit %s", event.State, truncateSHA(event.SHA))

	return &NormalizedEvidence{
		SourceType:      "github_ci",
		Subject:         a.repoName,
		Assertion:       assertion,
		ProvenanceClass: "external_feed",
		SourceURL:       statusURL,
		ContentSHA256:   contentHash,
		DomainPayload:   payload,
	}, nil
}

func (a *Adapter) processDeployment(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	deployURL := fmt.Sprintf("https://github.com/%s/%s/deployments", a.repoOwner, a.repoName)

	payload := map[string]interface{}{
		"environment": event.Metadata["environment"],
		"ref":         event.Branch,
		"sha":         event.SHA,
		"status":      event.State,
		"author":      event.Author,
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("deploy-%s-%s-%s", event.Branch, event.SHA, event.State)))
	contentHash := fmt.Sprintf("%x", hash)

	assertion := fmt.Sprintf("deployment to %s %s for %s", event.Metadata["environment"], event.State, truncateSHA(event.SHA))

	return &NormalizedEvidence{
		SourceType:      "github_deployment",
		Subject:         a.repoName,
		Assertion:       assertion,
		ProvenanceClass: "external_feed",
		SourceURL:       deployURL,
		ContentSHA256:   contentHash,
		DomainPayload:   payload,
	}, nil
}

func (a *Adapter) processIssue(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	issueURL := fmt.Sprintf("https://github.com/%s/%s/issues/%s", a.repoOwner, a.repoName, event.Metadata["number"])

	payload := map[string]interface{}{
		"number": event.Metadata["number"],
		"action": event.Metadata["action"],
		"state":  event.State,
		"author": event.Author,
		"labels": event.Labels,
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("issue-%s-%s", event.Metadata["number"], event.SHA)))
	contentHash := fmt.Sprintf("%x", hash)

	assertion := fmt.Sprintf("issue #%s %s", event.Metadata["number"], event.Metadata["action"])

	return &NormalizedEvidence{
		SourceType:      "github_issue",
		Subject:         a.repoName,
		Assertion:       assertion,
		ProvenanceClass: "external_feed",
		SourceURL:       issueURL,
		ContentSHA256:   contentHash,
		DomainPayload:   payload,
	}, nil
}

func (a *Adapter) processComment(ctx context.Context, event Event) (*NormalizedEvidence, error) {
	commentURL := fmt.Sprintf("https://github.com/%s/%s/issues/%s#issuecomment-%s",
		a.repoOwner, a.repoName, event.Metadata["number"], event.SHA)

	payload := map[string]interface{}{
		"number":   event.Metadata["number"],
		"action":   event.Metadata["action"],
		"author":   event.Author,
		"body":     event.Body,
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("comment-%s-%s", event.Metadata["number"], event.SHA)))
	contentHash := fmt.Sprintf("%x", hash)

	assertion := fmt.Sprintf("comment on issue #%s by %s", event.Metadata["number"], event.Author)

	return &NormalizedEvidence{
		SourceType:      "github_comment",
		Subject:         a.repoName,
		Assertion:       assertion,
		ProvenanceClass: "external_feed",
		SourceURL:       commentURL,
		ContentSHA256:   contentHash,
		DomainPayload:   payload,
	}, nil
}

// ParseWebhook parses a raw GitHub webhook payload into an Event.
func ParseWebhook(payload []byte, eventType string) (*Event, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse webhook: %w", err)
	}

	event := &Event{
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}

	repo, _ := raw["repository"].(map[string]interface{})
	if repo != nil {
		event.Repository, _ = repo["full_name"].(string)
	}

	sender, _ := raw["sender"].(map[string]interface{})
	if sender != nil {
		event.Author, _ = sender["login"].(string)
	}

	switch eventType {
	case "push":
		event.Type = EventPush
		event.Branch = trimRef(raw["ref"].(string))
		event.SHA = raw["after"].(string)

	case "pull_request":
		event.Type = EventPR
		pr, _ := raw["pull_request"].(map[string]interface{})
		if pr != nil {
			event.SHA, _ = pr["head"].(map[string]interface{})["sha"].(string)
			event.Branch, _ = pr["head"].(map[string]interface{})["ref"].(string)
			event.Title, _ = pr["title"].(string)
			event.Body, _ = pr["body"].(string)
			number := fmt.Sprintf("%v", pr["number"])
			event.Metadata["number"] = number
			action, _ := raw["action"].(string)
			event.Metadata["action"] = action
			event.State = action
			if action == "closed" {
				merged, _ := pr["merged"].(bool)
				if merged {
					event.State = "merged"
				}
			}
		}

	case "status":
		event.Type = EventCIStatus
		event.SHA = raw["sha"].(string)
		event.State = raw["state"].(string)
		event.Metadata["context"] = raw["context"].(string)

	case "deployment":
		event.Type = EventDeployment
		event.SHA = raw["sha"].(string)
		event.Branch = raw["ref"].(string)
		event.Metadata["environment"] = raw["environment"].(string)
		event.State = raw["state"].(string)

	case "issues":
		event.Type = EventIssue
		issue, _ := raw["issue"].(map[string]interface{})
		if issue != nil {
			event.SHA = fmt.Sprintf("%v", issue["id"])
			event.Title, _ = issue["title"].(string)
			event.Body, _ = issue["body"].(string)
			number := fmt.Sprintf("%v", issue["number"])
			event.Metadata["number"] = number
			action, _ := raw["action"].(string)
			event.Metadata["action"] = action
		}

	case "issue_comment":
		event.Type = EventComment
		issue, _ := raw["issue"].(map[string]interface{})
		if issue != nil {
			number := fmt.Sprintf("%v", issue["number"])
			event.Metadata["number"] = number
		}
		comment, _ := raw["comment"].(map[string]interface{})
		if comment != nil {
			event.SHA = fmt.Sprintf("%v", comment["id"])
			event.Body, _ = comment["body"].(string)
		}
		action, _ := raw["action"].(string)
		event.Metadata["action"] = action

	default:
		return nil, fmt.Errorf("unsupported webhook event: %s", eventType)
	}

	return event, nil
}

func trimRef(ref string) string {
	const prefix = "refs/heads/"
	if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
		return ref[len(prefix):]
	}
	return ref
}

// truncateSHA returns the first 7 characters of a SHA, or the full SHA if shorter.
func truncateSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}
