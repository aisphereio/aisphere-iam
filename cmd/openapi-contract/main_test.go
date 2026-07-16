package main

import "testing"

func TestCollectFilePoliciesAndFilterConsoleOperations(t *testing.T) {
	protoSource := `
syntax = "proto3";
service ExampleService {
  rpc PublicCall(Request) returns (Reply) {
    option (google.api.http) = { get: "/v1/orgs/{org_id}/items/{item_id}" };
    option (aisphere.access.v1.policy) = { exposure: AUTHORIZED };
  }
  rpc InternalCall(Request) returns (Reply) {
    option (google.api.http) = { post: "/v1/internal/items:sync" body: "*" };
    option (aisphere.access.v1.policy) = { exposure: INTERNAL };
  }
}
`
	policies := make(map[routeKey]routePolicy)
	if err := collectFilePolicies("example.proto", protoSource, policies); err != nil {
		t.Fatalf("collectFilePolicies() error = %v", err)
	}

	publicKey := routeKey{Method: "get", Path: "/v1/orgs/{}/items/{}"}
	if got := policies[publicKey].Exposure; got != "AUTHORIZED" {
		t.Fatalf("public exposure = %q, want AUTHORIZED", got)
	}

	document := map[string]any{
		"swagger": "2.0",
		"paths": map[string]any{
			"/v1/orgs/{orgId}/items/{itemId}": map[string]any{
				"get": map[string]any{"operationId": "ExampleService_PublicCall"},
			},
			"/v1/internal/items:sync": map[string]any{
				"post": map[string]any{"operationId": "ExampleService_InternalCall"},
			},
		},
	}
	if err := filterConsoleOperations(document, policies); err != nil {
		t.Fatalf("filterConsoleOperations() error = %v", err)
	}

	paths := document["paths"].(map[string]any)
	if _, ok := paths["/v1/internal/items:sync"]; ok {
		t.Fatal("internal operation was not removed")
	}
	publicPath := paths["/v1/orgs/{orgId}/items/{itemId}"].(map[string]any)
	operation := publicPath["get"].(map[string]any)
	if got := operation["x-aisphere-exposure"]; got != "AUTHORIZED" {
		t.Fatalf("x-aisphere-exposure = %v, want AUTHORIZED", got)
	}
	if got := operation["x-aisphere-rpc"]; got != "ExampleService.PublicCall" {
		t.Fatalf("x-aisphere-rpc = %v, want ExampleService.PublicCall", got)
	}
}

func TestCollectFilePoliciesRequiresExposure(t *testing.T) {
	protoSource := `
service BrokenService {
  rpc BrokenCall(Request) returns (Reply) {
    option (google.api.http) = { get: "/v1/broken" };
  }
}
`
	if err := collectFilePolicies("broken.proto", protoSource, make(map[routeKey]routePolicy)); err == nil {
		t.Fatal("collectFilePolicies() error = nil, want missing exposure error")
	}
}

func TestNormalizePath(t *testing.T) {
	got := normalizePath("/v1/orgs/{orgId}/projects/{project_id}")
	want := "/v1/orgs/{}/projects/{}"
	if got != want {
		t.Fatalf("normalizePath() = %q, want %q", got, want)
	}
}
