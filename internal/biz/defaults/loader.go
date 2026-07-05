// Package defaults loads built-in resource-control-plane definitions from
// configs/resource/defaults.yaml and reconciles them into IAM DB through the
// domain services. It is deliberately idempotent: all writes use upsert paths.
package defaults

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	grantbiz "github.com/aisphereio/aisphere-iam/internal/biz/grant"
	projectbiz "github.com/aisphereio/aisphere-iam/internal/biz/project"
	resourcebiz "github.com/aisphereio/aisphere-iam/internal/biz/resource"
	"gopkg.in/yaml.v3"
)

type File struct {
	Capabilities  []Capability   `yaml:"capabilities"`
	ResourceTypes []ResourceType `yaml:"resource_types"`
	RoleTemplates []RoleTemplate `yaml:"role_templates"`
}

type Capability struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	DisplayName  string `yaml:"display_name"`
	OwnerService string `yaml:"owner_service"`
	Status       string `yaml:"status"`
}

type ResourceType struct {
	Type         string   `yaml:"type"`
	CapabilityID string   `yaml:"capability_id"`
	OwnerService string   `yaml:"owner_service"`
	ParentTypes  []string `yaml:"parent_types"`
	Grantable    bool     `yaml:"grantable"`
	Auditable    bool     `yaml:"auditable"`
	SpiceDBType  string   `yaml:"spicedb_type"`
	Relations    []string `yaml:"relations"`
	Permissions  []string `yaml:"permissions"`
	Status       string   `yaml:"status"`
}

type RoleTemplate struct {
	ResourceType string `yaml:"resource_type"`
	RoleKey      string `yaml:"role_key"`
	DisplayName  string `yaml:"display_name"`
	Relation     string `yaml:"relation"`
	BuiltIn      bool   `yaml:"built_in"`
	Enabled      bool   `yaml:"enabled"`
	SortOrder    int    `yaml:"sort_order"`
}

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
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out File
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
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
