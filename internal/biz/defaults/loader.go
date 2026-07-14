// Package defaults loads built-in resource-control-plane definitions from
// configs/resource/defaults.yaml and reconciles them into IAM DB through the
// domain services. It is deliberately idempotent: all writes use upsert paths.
package defaults

import (
	"context"
	"encoding/json"
	"fmt"

	grantbiz "github.com/aisphereio/aisphere-iam/internal/biz/grant"
	projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
	resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
	"github.com/aisphereio/aisphere-iam/internal/permissionmanifest"
)

type File = permissionmanifest.Manifest
type Capability = permissionmanifest.Capability
type ResourceType = permissionmanifest.ResourceType
type RoleTemplate = permissionmanifest.RoleTemplate

type Services struct {
	Projects  *projectbiz.Service
	Resources *resourcebiz.Service
	Grants    *grantbiz.Service
}

type Result struct {
	Capabilities  int
	ResourceTypes int
	RoleTemplates int
}

func LoadFile(path string) (*File, error) {
	return permissionmanifest.Load(path)
}

func ReconcileFile(ctx context.Context, path string, services Services) (Result, error) {
	file, err := LoadFile(path)
	if err != nil {
		return Result{}, err
	}
	return Reconcile(ctx, file, services)
}

func Reconcile(ctx context.Context, file *File, services Services) (Result, error) {
	if file == nil {
		return Result{}, nil
	}
	if services.Projects == nil || services.Resources == nil || services.Grants == nil {
		return Result{}, fmt.Errorf("defaults reconcile requires project, resource and grant services")
	}
	var result Result
	for _, cap := range file.Capabilities {
		if _, err := services.Projects.RegisterCapability(ctx, projectbiz.RegisterCapabilityRequest{
			ID:           cap.ID,
			Name:         cap.Name,
			DisplayName:  cap.DisplayName,
			OwnerService: cap.OwnerService,
		}); err != nil {
			return result, err
		}
		result.Capabilities++
	}
	for _, rt := range file.ResourceTypes {
		if _, err := services.Resources.RegisterResourceType(ctx, resourcebiz.RegisterResourceTypeRequest{
			Type:            rt.Type,
			CapabilityID:    rt.CapabilityID,
			OwnerService:    rt.OwnerService,
			ParentTypesJSON: mustJSON(rt.ParentTypes),
			Grantable:       rt.Grantable,
			Auditable:       rt.Auditable,
			SpiceDBType:     rt.SpiceDBType,
			RelationsJSON:   mustJSON(rt.Relations),
			PermissionsJSON: mustJSON(rt.Permissions),
		}); err != nil {
			return result, err
		}
		result.ResourceTypes++
	}
	for _, role := range file.RoleTemplates {
		if _, err := services.Grants.RegisterRoleTemplate(ctx, grantbiz.RegisterRoleTemplateRequest{
			ResourceType: role.ResourceType,
			RoleKey:      role.RoleKey,
			DisplayName:  role.DisplayName,
			Relation:     role.Relation,
			BuiltIn:      role.BuiltIn,
			Enabled:      role.Enabled,
			SortOrder:    role.SortOrder,
		}); err != nil {
			return result, err
		}
		result.RoleTemplates++
	}
	return result, nil
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
