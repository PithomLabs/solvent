package github

import (
	"testing"
	"time"
)

func TestProcessPush(t *testing.T) {
	a := New("owner", "repo")
	event := Event{
		Type:       EventPush,
		Repository: "owner/repo",
		Branch:     "main",
		SHA:        "abc123def456",
		Author:     "developer",
		Timestamp:  time.Now(),
	}

	ev, err := a.ProcessEvent(t.Context(), event)
	if err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	if ev.SourceType != "github_push" {
		t.Errorf("SourceType = %q, want %q", ev.SourceType, "github_push")
	}
	if ev.Subject != "repo" {
		t.Errorf("Subject = %q, want %q", ev.Subject, "repo")
	}
	if ev.ProvenanceClass != "external_feed" {
		t.Errorf("ProvenanceClass = %q, want %q", ev.ProvenanceClass, "external_feed")
	}
	if ev.ContentSHA256 == "" {
		t.Error("ContentSHA256 should not be empty")
	}
}

func TestProcessPR(t *testing.T) {
	a := New("owner", "repo")
	event := Event{
		Type:       EventPR,
		Repository: "owner/repo",
		Branch:     "main",
		SHA:        "abc123",
		Author:     "developer",
		Timestamp:  time.Now(),
		Metadata: map[string]string{
			"number": "42",
			"action": "opened",
		},
	}

	ev, err := a.ProcessEvent(t.Context(), event)
	if err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	if ev.SourceType != "github_pr" {
		t.Errorf("SourceType = %q, want %q", ev.SourceType, "github_pr")
	}
	if ev.Assertion == "" {
		t.Error("Assertion should not be empty")
	}
}

func TestProcessCIStatus(t *testing.T) {
	a := New("owner", "repo")
	event := Event{
		Type:       EventCIStatus,
		Repository: "owner/repo",
		SHA:        "abc123",
		State:      "success",
		Author:     "ci-bot",
		Timestamp:  time.Now(),
		Metadata: map[string]string{
			"context": "build",
		},
	}

	ev, err := a.ProcessEvent(t.Context(), event)
	if err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	if ev.SourceType != "github_ci" {
		t.Errorf("SourceType = %q, want %q", ev.SourceType, "github_ci")
	}
}

func TestParseWebhookPush(t *testing.T) {
	payload := `{
		"ref": "refs/heads/main",
		"after": "abc123",
		"repository": {"full_name": "owner/repo"},
		"sender": {"login": "developer"}
	}`

	event, err := ParseWebhook([]byte(payload), "push")
	if err != nil {
		t.Fatalf("ParseWebhook returned error: %v", err)
	}

	if event.Type != EventPush {
		t.Errorf("Type = %q, want %q", event.Type, EventPush)
	}
	if event.Branch != "main" {
		t.Errorf("Branch = %q, want %q", event.Branch, "main")
	}
	if event.SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", event.SHA, "abc123")
	}
}

func TestTrimRef(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"refs/heads/main", "main"},
		{"refs/heads/feature/foo", "feature/foo"},
		{"main", "main"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimRef(tt.input)
			if got != tt.want {
				t.Errorf("trimRef(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
