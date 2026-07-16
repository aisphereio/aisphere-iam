package server

import (
	"context"
	"testing"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authz"
)

func TestGroupAdminGeneratedAccessUsesGroupManagementModel(t *testing.T) {
	catalog := IAMCatalog()
	tests := []struct {
		name       string
		operation  string
		req        any
		resource   authz.ObjectRef
		permission string
	}{
		{
			name:       "create top level group checks zone create_groups",
			operation:  "/iam.v1.IAMGroupAdminService/CreateGroup",
			req:        &v1.CreateGroupRequest{OrgId: "aisphere", Group: &v1.Group{Name: "platform"}},
			resource:   authz.ObjectRef{Type: "zone", ID: "aisphere"},
			permission: "create_groups",
		},
{
				name:       "update group checks group manage",
				operation:  "/iam.v1.IAMGroupAdminService/UpdateGroup",
				req:        &v1.UpdateGroupRequest{OrgId: "aisphere", GroupId: "platform", Group: &v1.Group{Name: "platform"}},
				resource:   authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
				permission: "manage",
			},
			{
				name:       "delete group checks group manage",
				operation:  "/iam.v1.IAMGroupAdminService/DeleteGroup",
				req:        &v1.DeleteGroupRequest{OrgId: "aisphere", GroupId: "platform"},
				resource:   authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
				permission: "manage",
			},
{
					name:       "assign group member checks group manage_members",
					operation:  "/iam.v1.IAMGroupAdminService/AssignUserToGroup",
					req:        &v1.AssignUserToGroupRequest{OrgId: "aisphere", GroupId: "platform", UserId: "user-1"},
					resource:   authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
					permission: "manage_members",
				},
				{
					name:       "remove group member checks group manage_members",
					operation:  "/iam.v1.IAMGroupAdminService/RemoveUserFromGroup",
					req:        &v1.RemoveUserFromGroupRequest{OrgId: "aisphere", GroupId: "platform", UserId: "user-1"},
					resource:   authz.ObjectRef{Type: "group", ID: "aisphere/platform"},
					permission: "manage_members",
				},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check, ok, err := catalog.AccessResolver(context.Background(), tt.operation, tt.req)
			if err != nil {
				t.Fatalf("AccessResolver returned error: %v", err)
			}
			if !ok {
				t.Fatalf("AccessResolver did not resolve %s", tt.operation)
			}
			if check.Resource != tt.resource {
				t.Fatalf("resource = %#v, want %#v", check.Resource, tt.resource)
			}
			if check.Permission != tt.permission {
				t.Fatalf("permission = %q, want %q", check.Permission, tt.permission)
			}
		})
	}
}

func TestGroupAdminGeneratedGatewayUsesPatchForGroupUpdate(t *testing.T) {
	manifest := v1.IAMGroupAdminServiceGatewayManifest()
	for _, route := range manifest.Routes {
		if route.Upstream.Operation != "/iam.v1.IAMGroupAdminService/UpdateGroup" {
			continue
		}
		if route.Method != "PATCH" {
			t.Fatalf("UpdateGroup gateway method = %q, want PATCH", route.Method)
		}
		return
	}
	t.Fatal("UpdateGroup route not found in generated gateway manifest")
}

func TestIdentityAdminHasNoGroupOperations(t *testing.T) {
	manifest := v1.IAMIdentityAdminServiceGatewayManifest()
	for _, route := range manifest.Routes {
		switch route.Upstream.Operation {
		case "/iam.v1.IAMIdentityAdminService/CreateGroup",
			"/iam.v1.IAMIdentityAdminService/UpdateGroup",
			"/iam.v1.IAMIdentityAdminService/DeleteGroup",
			"/iam.v1.IAMIdentityAdminService/AssignUserToGroup",
			"/iam.v1.IAMIdentityAdminService/RemoveUserFromGroup":
			t.Fatalf("IAMIdentityAdminService still exposes group operation: %s", route.Upstream.Operation)
		}
	}
}