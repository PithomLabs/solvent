// Package main demonstrates a GitHub integration for the Solvent API.
//
// This is deliberately dumb — it orchestrates API calls in sequence.
// It is NOT a workflow engine. GitHub-specific logic stays inside
// adapter/github/; this example calls the REST API.
//
// Usage:
//
//	go run . -url http://localhost:8080 -key your-api-key
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "Solvent API URL")
	apiKey := flag.String("key", "", "API key")
	flag.Parse()

	if *apiKey == "" {
		fmt.Fprintln(os.Stderr, "Set -key flag")
		os.Exit(1)
	}

	c := &client{baseURL: *baseURL, apiKey: *apiKey}

	// 1. Create principal for the GitHub adapter
	principal, err := c.post("/v1/principals", map[string]string{
		"principal_type": "service",
		"issuer":         "github-adapter",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create principal: %v\n", err)
		os.Exit(1)
	}
	principalID := principal["principal_id"].(string)
	fmt.Printf("1. Created principal: %s\n", principalID)

	// 2. Enter belief from a GitHub issue
	scenarioID := "00000000-0000-0000-0000-000000000001"
	belief, err := c.post("/v1/beliefs", map[string]string{
		"scenario_id": scenarioID,
		"claim":       "etcd v3.5.0 is safe to deploy (from GitHub issue #12345)",
		"claim_type":  "derived",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "enter belief: %v\n", err)
		os.Exit(1)
	}
	beliefID := belief["belief_id"].(string)
	fmt.Printf("2. Entered belief: %s\n", beliefID)

	// 3. Add evidence from the GitHub issue
	_, err = c.post("/v1/evidence", map[string]string{
		"scenario_id":      scenarioID,
		"belief_id":        beliefID,
		"provenance_class": "external_feed",
		"source_url":       "https://github.com/etcd-io/etcd/issues/12345",
		"content_sha256":   "abc123",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "add evidence: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("3. Added evidence")

	// 4. Promote belief
	promoted, err := c.post(fmt.Sprintf("/v1/beliefs/%s/promote?scenario_id=%s", beliefID, scenarioID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "promote belief: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("4. Belief status: %v\n", promoted["status"])

	// 5. Create target
	target, err := c.post("/v1/targets", map[string]interface{}{
		"principal_id":     principalID,
		"resource_type":    "scenario",
		"resource_id":      scenarioID,
		"scope":            "belief:" + beliefID,
		"action_namespace": "solvent",
		"action_name":      "deploy",
		"consequence_type": "execution",
		"created_by":       principalID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create target: %v\n", err)
		os.Exit(1)
	}
	targetID := target["target_id"].(string)
	fmt.Printf("5. Created target: %s\n", targetID)

	// 6. Attach justification
	_, err = c.post(fmt.Sprintf("/v1/targets/%s/justifications?belief_id=%s", targetID, beliefID), map[string]string{
		"instrument_ref": "github-issue-12345",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "attach justification: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("6. Attached justification")

	// 7. Request authorization
	_, err = c.post(fmt.Sprintf("/v1/targets/%s/request-authorization?principal_id=%s", targetID, principalID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "request authorization: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("7. Requested authorization")

	// 8. Approve
	_, err = c.post(fmt.Sprintf("/v1/targets/%s/approve?principal_id=%s", targetID, principalID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "approve: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("8. Approved")

	// 9. Create intent and execute
	intent, err := c.post("/v1/intents", map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   beliefID,
		"action":      "deploy",
		"target_id":   targetID,
		"principal_id": principalID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create intent: %v\n", err)
		os.Exit(1)
	}
	intentID := intent["intent_id"].(string)
	fmt.Printf("9. Created intent: %s\n", intentID)

	execResult, err := c.post("/v1/authorizations/execute", map[string]interface{}{
		"scenario_id": scenarioID,
		"belief_id":   beliefID,
		"action":      "deploy",
		"target_id":   targetID,
		"intent_id":   intentID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("10. Executed: allowed=%v success=%v\n", execResult["allowed"], execResult["success"])

	fmt.Println("\nGitHub integration example complete.")
	fmt.Println("The adapter calls the REST API; all authorization logic lives in the kernel.")
}

type client struct {
	baseURL string
	apiKey  string
}

func (c *client) post(path string, body interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %v", resp.StatusCode, result["message"])
	}

	return result, nil
}
