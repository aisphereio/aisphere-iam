#!/usr/bin/env python3
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    (ROOT / path).write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str) -> None:
    content = read(path)
    if old not in content:
        raise RuntimeError(f"expected text not found in {path}: {old[:160]!r}")
    write(path, content.replace(old, new, 1))


proto_rpc_marker = '''  rpc WriteRelationship(WriteRelationshipRequest) returns (WriteRelationshipReply) {
'''
proto_runtime_rpcs = '''  rpc BatchCheckPermissions(BatchCheckPermissionsRequest) returns (BatchCheckPermissionsReply) {
    option (google.api.http) = { post: "/v1/iam/permissions:batch-check" body: "*" };
    option (aisphere.access.v1.policy) = {
      exposure: INTERNAL
      authz: { action: "check" resource: "iam:permission" audience: "iam-service" mode: CHECK_ONLY }
      audit: { enabled: true event: "iam.permission.batch_check" risk: "medium" }
      reason: "Batch permission decisions are restricted to trusted platform services."
    };
  }

  rpc WriteRelationships(WriteRelationshipsRequest) returns (WriteRelationshipsReply) {
    option (google.api.http) = { post: "/v1/iam/relationships:write" body: "*" };
    option (aisphere.access.v1.policy) = {
      exposure: INTERNAL
      authz: { action: "write" resource: "iam:relationship" audience: "iam-service" mode: CHECK_ONLY }
      audit: { enabled: true event: "iam.relationship.batch_write" risk: "high" }
      reason: "Relationship projection is restricted to trusted platform services."
    };
  }

  rpc DeleteRelationships(DeleteRelationshipsRequest) returns (DeleteRelationshipsReply) {
    option (google.api.http) = { post: "/v1/iam/relationships:delete" body: "*" };
    option (aisphere.access.v1.policy) = {
      exposure: INTERNAL
      authz: { action: "delete" resource: "iam:relationship" audience: "iam-service" mode: CHECK_ONLY }
      audit: { enabled: true event: "iam.relationship.batch_delete" risk: "high" }
      reason: "Relationship projection cleanup is restricted to trusted platform services."
    };
  }

  rpc ReadRelationships(ListRelationshipsRequest) returns (ListRelationshipsReply) {
    option (google.api.http) = { post: "/v1/iam/relationships:read" body: "*" };
    option (aisphere.access.v1.policy) = {
      exposure: INTERNAL
      authz: { action: "read" resource: "iam:relationship" audience: "iam-service" mode: CHECK_ONLY }
      audit: { enabled: true event: "iam.relationship.read" risk: "medium" }
      reason: "Relationship graph reads are restricted to trusted platform services."
    };
  }

'''
proto = read("api/iam/v1/iam.proto")
if "rpc BatchCheckPermissions" not in proto:
    replace_once("api/iam/v1/iam.proto", proto_rpc_marker, proto_runtime_rpcs + proto_rpc_marker)

message_marker = '''message CheckPermissionReply { bool allowed = 1; string effect = 2; string reason = 3; string consistency_token = 4; }
'''
batch_messages = '''message BatchCheckPermissionsRequest { repeated CheckPermissionRequest checks = 1 [(google.api.field_behavior) = REQUIRED, (buf.validate.field).repeated.min_items = 1]; }
message BatchCheckPermissionsReply { repeated CheckPermissionReply decisions = 1; }
'''
proto = read("api/iam/v1/iam.proto")
if "message BatchCheckPermissionsRequest" not in proto:
    replace_once("api/iam/v1/iam.proto", message_marker, message_marker + batch_messages)

service_path = "internal/service/iam.go"
service = read(service_path)
old_decision = '''	return &v1.CheckPermissionReply{
		Allowed:          decision.IsAllowed(),
		Effect:           string(decision.Effect),
		Reason:           decision.Reason,
		ConsistencyToken: decision.ConsistencyToken,
	}, nil
'''
if old_decision in service:
    service = service.replace(old_decision, "\treturn permissionDecisionToProto(decision), nil\n", 1)
    write(service_path, service)

method_marker = '''func (s *IAMPermissionService) WriteRelationship(ctx context.Context, req *v1.WriteRelationshipRequest) (*v1.WriteRelationshipReply, error) {
'''
runtime_methods = '''func (s *IAMPermissionService) BatchCheckPermissions(ctx context.Context, req *v1.BatchCheckPermissionsRequest) (*v1.BatchCheckPermissionsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	inputs := req.GetChecks()
	checks := make([]authz.CheckRequest, 0, len(inputs))
	for _, input := range inputs {
		checks = append(checks, permissionCheckFromProto(input))
	}
	result, err := s.deps.Authz.BatchCheck(ctx, authz.BatchCheckRequest{Checks: checks})
	if err != nil {
		return nil, err
	}
	decisions := make([]*v1.CheckPermissionReply, 0, len(result.Decisions))
	for _, decision := range result.Decisions {
		decisions = append(decisions, permissionDecisionToProto(decision))
	}
	return &v1.BatchCheckPermissionsReply{Decisions: decisions}, nil
}

func (s *IAMPermissionService) WriteRelationships(ctx context.Context, req *v1.WriteRelationshipsRequest) (*v1.WriteRelationshipsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	inputs := req.GetRelationships()
	rels := make([]authz.Relationship, 0, len(inputs))
	for _, input := range inputs {
		rels = append(rels, relationshipFromProto(input))
	}
	result, err := s.deps.Authz.WriteRelationships(ctx, rels...)
	if err != nil {
		return nil, err
	}
	return &v1.WriteRelationshipsReply{Written: int32(result.Written), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMPermissionService) DeleteRelationships(ctx context.Context, req *v1.DeleteRelationshipsRequest) (*v1.DeleteRelationshipsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	result, err := s.deps.Authz.DeleteRelationships(ctx, relationshipFilterFromProto(req.GetFilter()))
	if err != nil {
		return nil, err
	}
	return &v1.DeleteRelationshipsReply{Deleted: int32(result.Deleted), ConsistencyToken: result.ConsistencyToken}, nil
}

func (s *IAMPermissionService) ReadRelationships(ctx context.Context, req *v1.ListRelationshipsRequest) (*v1.ListRelationshipsReply, error) {
	if s.deps.Authz == nil {
		return nil, authz.ErrBackendFailed("authz provider is not configured", nil)
	}
	rels, err := s.deps.Authz.ReadRelationships(ctx, authz.RelationshipFilter{
		ResourceType: req.GetResourceType(),
		ResourceID:   req.GetResourceId(),
		Relation:     req.GetRelation(),
		SubjectType:  req.GetSubjectType(),
		SubjectID:    req.GetSubjectId(),
		SubjectRel:   req.GetSubjectRelation(),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*v1.Relationship, 0, len(rels))
	for _, rel := range rels {
		out = append(out, relationshipToProto(rel))
	}
	return &v1.ListRelationshipsReply{Relationships: out}, nil
}

'''
service = read(service_path)
if "func (s *IAMPermissionService) BatchCheckPermissions" not in service:
    replace_once(service_path, method_marker, runtime_methods + method_marker)

helper_marker = '''func tokenSetToProto(in authn.TokenSet) *v1.TokenSet {
'''
helpers = '''func permissionCheckFromProto(req *v1.CheckPermissionRequest) authz.CheckRequest {
	return authz.CheckRequest{
		Subject:    subjectFromProto(req.GetSubject()),
		Resource:   objectFromProto(req.GetResource()),
		Permission: req.GetPermission(),
		OrgID:      req.GetOrgId(),
		ProjectID:  req.GetProjectId(),
	}
}

func permissionDecisionToProto(decision authz.Decision) *v1.CheckPermissionReply {
	return &v1.CheckPermissionReply{
		Allowed:          decision.IsAllowed(),
		Effect:           string(decision.Effect),
		Reason:           decision.Reason,
		ConsistencyToken: decision.ConsistencyToken,
	}
}

'''
service = read(service_path)
if "func permissionCheckFromProto" not in service:
    replace_once(service_path, helper_marker, helpers + helper_marker)

# Make the single check use the same conversion path as batch checks.
service = read(service_path)
old_check_request = '''	decision, err := s.deps.Authz.Check(ctx, authz.CheckRequest{
		Subject:    subjectFromProto(req.GetSubject()),
		Resource:   objectFromProto(req.GetResource()),
		Permission: req.GetPermission(),
		OrgID:      req.GetOrgId(),
		ProjectID:  req.GetProjectId(),
	})
'''
if old_check_request in service:
    write(service_path, service.replace(old_check_request, "\tdecision, err := s.deps.Authz.Check(ctx, permissionCheckFromProto(req))\n", 1))

# Extend the client test to verify batch behavior as part of the public runtime contract.
test_path = "client/authzgrpc/client_test.go"
test = read(test_path)
needle = '''	if err != nil || !decision.IsAllowed() {
		t.Fatalf("Check = (%+v, %v)", decision, err)
	}

'''
addition = '''	batch, err := client.BatchCheck(ctx, authz.BatchCheckRequest{Checks: []authz.CheckRequest{{
		Subject: authz.SubjectRef{Type: "user", ID: "alice"}, Resource: authz.ObjectRef{Type: "skill", ID: "demo"}, Permission: "view",
	}}})
	if err != nil || len(batch.Decisions) != 1 || !batch.Decisions[0].IsAllowed() {
		t.Fatalf("BatchCheck = (%+v, %v)", batch, err)
	}

'''
if "BatchCheck =" not in test:
    replace_once(test_path, needle, needle + addition)

print("IAM runtime authorization gRPC API repair applied")
