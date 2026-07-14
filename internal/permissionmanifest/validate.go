package permissionmanifest

import (
	"fmt"
	"sort"
	"strings"
)

func Validate(manifest *Manifest, schema Schema) error {
	if manifest == nil {
		return fmt.Errorf("permission manifest is nil")
	}
	resourceTypes := make(map[string]ResourceType, len(manifest.ResourceTypes))
	for _, resourceType := range manifest.ResourceTypes {
		name := strings.TrimSpace(resourceType.Type)
		if name == "" {
			return fmt.Errorf("resource type name is required")
		}
		if _, exists := resourceTypes[name]; exists {
			return fmt.Errorf("duplicate resource type %s", name)
		}
		resourceTypes[name] = resourceType
		definitionName := strings.TrimSpace(resourceType.SpiceDBType)
		definition, ok := schema.Definitions[definitionName]
		if !ok {
			return fmt.Errorf("resource type %s references missing schema definition %s", name, definitionName)
		}
		if err := validateExactSet("resource type "+name+" relations", resourceType.Relations, definition.Relations); err != nil {
			return err
		}
		if err := validateExactSet("resource type "+name+" permissions", resourceType.Permissions, definition.Permissions); err != nil {
			return err
		}
	}

	for _, role := range manifest.RoleTemplates {
		resourceType, ok := resourceTypes[strings.TrimSpace(role.ResourceType)]
		if !ok {
			return fmt.Errorf("role template %s references unknown resource type %s", role.RoleKey, role.ResourceType)
		}
		definition := schema.Definitions[resourceType.SpiceDBType]
		if _, ok := definition.Relations[strings.TrimSpace(role.Relation)]; !ok {
			return fmt.Errorf("role template %s references missing relation %s on %s", role.RoleKey, role.Relation, resourceType.SpiceDBType)
		}
	}

	if err := validateBootstrap(manifest, schema); err != nil {
		return err
	}
	return nil
}

func validateBootstrap(manifest *Manifest, schema Schema) error {
	zone, ok := schema.Definitions["zone"]
	if !ok {
		return fmt.Errorf("bootstrap requires schema definition zone")
	}
	roleNames := make([]string, 0, len(manifest.Bootstrap.Roles))
	for name := range manifest.Bootstrap.Roles {
		roleNames = append(roleNames, name)
	}
	sort.Strings(roleNames)
	seen := map[string]struct{}{}
	for _, name := range roleNames {
		role := manifest.Bootstrap.Roles[name]
		for _, candidate := range append([]string{name}, role.Aliases...) {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				return fmt.Errorf("bootstrap role name or alias is empty")
			}
			if _, exists := seen[candidate]; exists {
				return fmt.Errorf("duplicate bootstrap role name or alias %s", candidate)
			}
			seen[candidate] = struct{}{}
		}
		for _, relation := range role.ZoneRelations {
			relation = strings.TrimSpace(relation)
			if _, ok := zone.Relations[relation]; !ok {
				return fmt.Errorf("bootstrap role %s references missing zone relation %s", name, relation)
			}
		}
	}
	if _, _, ok := manifest.ResolveBootstrapRole(manifest.Bootstrap.DefaultRole); !ok {
		return fmt.Errorf("bootstrap default role %s does not resolve", manifest.Bootstrap.DefaultRole)
	}
	for _, resource := range manifest.Bootstrap.AdminResources {
		resourceType := strings.TrimSpace(resource.Type)
		if _, ok := schema.Definitions[resourceType]; !ok {
			return fmt.Errorf("bootstrap admin resource type %s is not defined in schema", resourceType)
		}
		if strings.TrimSpace(resource.ID) == "" {
			return fmt.Errorf("bootstrap admin resource %s has empty id", resourceType)
		}
	}
	return nil
}

func validateExactSet(label string, manifestValues []string, schemaValues map[string]string) error {
	manifestSet := make(map[string]struct{}, len(manifestValues))
	for _, value := range manifestValues {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains an empty value", label)
		}
		manifestSet[value] = struct{}{}
	}
	missing := make([]string, 0)
	extra := make([]string, 0)
	for value := range schemaValues {
		if _, ok := manifestSet[value]; !ok {
			missing = append(missing, value)
		}
	}
	for value := range manifestSet {
		if _, ok := schemaValues[value]; !ok {
			extra = append(extra, value)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return fmt.Errorf("%s drift: missing=%v extra=%v", label, missing, extra)
}
