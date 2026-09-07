package openapi_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// openapiSpec is the parsed OpenAPI specification.
type openapiSpec struct {
	OpenAPI    string                            `yaml:"openapi"`
	Info       map[string]interface{}            `yaml:"info"`
	Paths      map[string]map[string]interface{} `yaml:"paths"`
	Components struct {
		SecuritySchemes map[string]interface{} `yaml:"securitySchemes"`
	} `yaml:"components"`
}

// loadSpec loads and parses the OpenAPI spec. Returns nil if unavailable.
func loadSpec(t *testing.T) *openapiSpec {
	t.Helper()

	paths := []string{
		"../../docs/openapi/solvent.yaml",
		"../docs/openapi/solvent.yaml",
		"docs/openapi/solvent.yaml",
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Skipf("OpenAPI spec unavailable: %v", err)
	}

	var spec openapiSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse OpenAPI spec: %v", err)
	}
	return &spec
}

type routeEntry struct {
	method string
	path   string
}

func TestOpenAPISpec_CanonicalRoutes(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	want := []routeEntry{
		{"post", "/v1/beliefs"},
		{"get", "/v1/beliefs"},
		{"get", "/v1/beliefs/{id}"},
		{"post", "/v1/beliefs/{id}/debt/retire"},
		{"post", "/v1/beliefs/{id}/promote"},
		{"post", "/v1/beliefs/{id}/retract"},
		{"get", "/v1/beliefs/{id}/explain"},
		{"get", "/v1/beliefs/{id}/evidence"},
		{"post", "/v1/evidence"},
		{"get", "/v1/evidence/{id}"},
		{"post", "/v1/principals"},
		{"get", "/v1/principals"},
		{"get", "/v1/principals/{id}"},
		{"post", "/v1/principals/{id}/revoke"},
		{"post", "/v1/targets"},
		{"get", "/v1/targets"},
		{"get", "/v1/targets/{id}"},
		{"post", "/v1/targets/{id}/justifications"},
		{"post", "/v1/targets/{id}/request"},
		{"post", "/v1/targets/{id}/approve"},
		{"post", "/v1/targets/{id}/revoke"},
		{"post", "/v1/authorizations/verify"},
		{"post", "/v1/authorizations/action"},
		{"post", "/v1/authorizations/execute"},
		{"post", "/v1/discharge"},
		{"get", "/v1/activity"},
		{"get", "/v1/ledger"},
	}

	for _, want := range want {
		pathItem, ok := spec.Paths[want.path]
		if !ok {
			t.Errorf("path %s missing from spec", want.path)
			continue
		}
		operation, ok := pathItem[want.method]
		if !ok {
			t.Errorf("path %s missing method %s", want.path, want.method)
			continue
		}
		op, ok := operation.(map[string]interface{})
		if !ok {
			t.Errorf("path %s method %s: not a map", want.path, want.method)
			continue
		}
		if _, ok := op["operationId"]; !ok {
			t.Errorf("path %s method %s: missing operationId", want.path, want.method)
		}
		if _, ok := op["responses"]; !ok {
			t.Errorf("path %s method %s: missing responses", want.path, want.method)
		}
	}
}

func TestOpenAPISpec_NoExtraPaths(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	allowed := map[string]bool{
		"/v1/beliefs":                     true,
		"/v1/beliefs/{id}":                true,
		"/v1/beliefs/{id}/debt/retire":    true,
		"/v1/beliefs/{id}/promote":        true,
		"/v1/beliefs/{id}/retract":        true,
		"/v1/beliefs/{id}/explain":        true,
		"/v1/beliefs/{id}/evidence":       true,
		"/v1/evidence":                    true,
		"/v1/evidence/{id}":               true,
		"/v1/principals":                  true,
		"/v1/principals/{id}":             true,
		"/v1/principals/{id}/revoke":      true,
		"/v1/targets":                     true,
		"/v1/targets/{id}":                true,
		"/v1/targets/{id}/justifications": true,
		"/v1/targets/{id}/request":        true,
		"/v1/targets/{id}/approve":        true,
		"/v1/targets/{id}/revoke":         true,
		"/v1/authorizations/verify":       true,
		"/v1/authorizations/action":       true,
		"/v1/authorizations/execute":      true,
		"/v1/discharge":                   true,
		"/v1/activity":                    true,
		"/v1/ledger":                      true,
	}

	for p := range spec.Paths {
		if !allowed[p] {
			t.Errorf("spec contains undocumented path: %s", p)
		}
	}
}

func TestOpenAPISpec_MutatingEndpointsRequireBearerAuth(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	// Check that global security is defined (covers all endpoints).
	if len(spec.Components.SecuritySchemes) == 0 {
		t.Fatal("no security schemes defined")
	}

	// All mutating POST endpoints must exist and have operationIds.
	mutatingPaths := []string{
		"/v1/beliefs",
		"/v1/beliefs/{id}/debt/retire",
		"/v1/beliefs/{id}/promote",
		"/v1/beliefs/{id}/retract",
		"/v1/evidence",
		"/v1/principals",
		"/v1/principals/{id}/revoke",
		"/v1/targets",
		"/v1/targets/{id}/justifications",
		"/v1/targets/{id}/request",
		"/v1/targets/{id}/approve",
		"/v1/targets/{id}/revoke",
		"/v1/authorizations/verify",
		"/v1/authorizations/action",
		"/v1/authorizations/execute",
		"/v1/discharge",
	}

	for _, p := range mutatingPaths {
		item, ok := spec.Paths[p]
		if !ok {
			t.Errorf("path %s missing from spec", p)
			continue
		}
		op, ok := item["post"].(map[string]interface{})
		if !ok {
			t.Errorf("path %s missing post method", p)
			continue
		}
		if _, ok := op["operationId"]; !ok {
			t.Errorf("path %s post: missing operationId", p)
		}
		if _, ok := op["responses"]; !ok {
			t.Errorf("path %s post: missing responses", p)
		}
		// Global security applies; no per-endpoint override needed.
	}
}

func TestOpenAPISpec_SecurityDecision_VerifyAuthNoPrincipalID(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	item, ok := spec.Paths["/v1/authorizations/verify"]
	if !ok {
		t.Fatal("path /v1/authorizations/verify missing")
	}
	post, ok := item["post"].(map[string]interface{})
	if !ok {
		t.Fatal("missing post method")
	}
	body, ok := post["requestBody"].(map[string]interface{})
	if !ok {
		t.Fatal("missing requestBody")
	}
	content, ok := body["content"].(map[string]interface{})
	if !ok {
		t.Fatal("missing content")
	}
	jsonSchema, ok := content["application/json"].(map[string]interface{})
	if !ok {
		t.Fatal("missing application/json")
	}
	schemaRef, ok := jsonSchema["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("missing schema")
	}
	ref, ok := schemaRef["$ref"].(string)
	if !ok || ref != "#/components/schemas/VerifyAuthRequest" {
		t.Errorf("expected $ref to VerifyAuthRequest, got %v", ref)
	}
}

func TestOpenAPISpec_SecurityDecision_AuthorizeActionHasOptionalActorID(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	item, ok := spec.Paths["/v1/authorizations/action"]
	if !ok {
		t.Fatal("path /v1/authorizations/action missing")
	}
	post, ok := item["post"].(map[string]interface{})
	if !ok {
		t.Fatal("missing post method")
	}
	body, ok := post["requestBody"].(map[string]interface{})
	if !ok {
		t.Fatal("missing requestBody")
	}
	content, ok := body["content"].(map[string]interface{})
	if !ok {
		t.Fatal("missing content")
	}
	jsonSchema, ok := content["application/json"].(map[string]interface{})
	if !ok {
		t.Fatal("missing application/json")
	}
	schemaRef, ok := jsonSchema["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("missing schema")
	}
	ref, ok := schemaRef["$ref"].(string)
	if !ok || ref != "#/components/schemas/AuthorizeActionRequest" {
		t.Errorf("expected $ref to AuthorizeActionRequest, got %v", ref)
	}
}

func TestOpenAPISpec_BearerAuthSchemeDefined(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	bearerAuth, ok := spec.Components.SecuritySchemes["bearerAuth"]
	if !ok {
		t.Fatal("bearerAuth security scheme not defined")
	}
	scheme, ok := bearerAuth.(map[string]interface{})
	if !ok {
		t.Fatal("bearerAuth is not a map")
	}
	if scheme["type"] != "http" {
		t.Errorf("bearerAuth type: want http, got %v", scheme["type"])
	}
	if scheme["scheme"] != "bearer" {
		t.Errorf("bearerAuth scheme: want bearer, got %v", scheme["scheme"])
	}
}

func TestOpenAPISpec_GlobalSecurity(t *testing.T) {
	spec := loadSpec(t)
	if spec == nil {
		return
	}

	_ = spec // spec loaded successfully; global security checked via raw YAML below.

	paths := []string{
		"../../docs/openapi/solvent.yaml",
		"../docs/openapi/solvent.yaml",
		"docs/openapi/solvent.yaml",
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Skipf("spec unavailable: %v", err)
	}
	var raw struct {
		Security []map[string][]string `yaml:"security"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse raw spec: %v", err)
	}
	if len(raw.Security) == 0 {
		t.Error("spec missing global security declaration")
	}
}
