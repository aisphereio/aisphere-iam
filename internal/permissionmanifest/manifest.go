package permissionmanifest

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Capabilities  []Capability    `yaml:"capabilities"`
	ResourceTypes []ResourceType  `yaml:"resource_types"`
	RoleTemplates []RoleTemplate  `yaml:"role_templates"`
	Bootstrap     BootstrapPolicy `yaml:"bootstrap"`
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

type BootstrapPolicy struct {
	DefaultRole    string                   `yaml:"default_role"`
	Roles          map[string]BootstrapRole `yaml:"roles"`
	AdminResources []AdminResource          `yaml:"admin_resources"`
}

type BootstrapRole struct {
	Aliases           []string `yaml:"aliases"`
	ZoneRelations     []string `yaml:"zone_relations"`
	ControlPlaneAdmin bool     `yaml:"control_plane_admin"`
}

type AdminResource struct {
	Type string `yaml:"type"`
	ID   string `yaml:"id"`
}

func Load(path string) (*Manifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := yaml.Unmarshal(body, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (m *Manifest) ResolveBootstrapRole(role string) (BootstrapRole, string, bool) {
	if m == nil {
		return BootstrapRole{}, "", false
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = strings.TrimSpace(m.Bootstrap.DefaultRole)
	}
	if policy, ok := m.Bootstrap.Roles[role]; ok {
		return policy, role, true
	}
	for canonical, policy := range m.Bootstrap.Roles {
		for _, alias := range policy.Aliases {
			if strings.TrimSpace(alias) == role {
				return policy, canonical, true
			}
		}
	}
	return BootstrapRole{}, "", false
}
