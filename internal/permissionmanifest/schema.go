package permissionmanifest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type Schema struct {
	Definitions map[string]Definition
}

type Definition struct {
	Relations   map[string]string
	Permissions map[string]string
}

type SchemaDiff struct {
	Additions []string
	Conflicts []string
}

var (
	definitionPattern  = regexp.MustCompile(`\bdefinition\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{`)
	declarationPattern = regexp.MustCompile(`(?m)^\s*(relation|permission)\s+([A-Za-z_][A-Za-z0-9_]*)\s*[:=]\s*([^\r\n]+?)\s*$`)
)

func ParseSchema(text string) (Schema, error) {
	clean := stripSchemaComments(text)
	schema := Schema{Definitions: map[string]Definition{}}
	for offset := 0; offset < len(clean); {
		match := definitionPattern.FindStringSubmatchIndex(clean[offset:])
		if match == nil {
			break
		}
		name := clean[offset+match[2] : offset+match[3]]
		if _, exists := schema.Definitions[name]; exists {
			return Schema{}, fmt.Errorf("duplicate definition %s", name)
		}
		open := offset + match[1] - 1
		close, err := matchingBrace(clean, open)
		if err != nil {
			return Schema{}, fmt.Errorf("definition %s: %w", name, err)
		}
		definition, err := parseDefinition(name, clean[open+1:close])
		if err != nil {
			return Schema{}, err
		}
		schema.Definitions[name] = definition
		offset = close + 1
	}
	return schema, nil
}

func CompareSchemas(current, desired Schema) SchemaDiff {
	diff := SchemaDiff{}
	for name, currentDefinition := range current.Definitions {
		desiredDefinition, ok := desired.Definitions[name]
		if !ok {
			diff.Conflicts = append(diff.Conflicts, "definition "+name+" exists only in active schema")
			continue
		}
		compareMembers(name, "relation", currentDefinition.Relations, desiredDefinition.Relations, &diff)
		compareMembers(name, "permission", currentDefinition.Permissions, desiredDefinition.Permissions, &diff)
	}
	for name, desiredDefinition := range desired.Definitions {
		currentDefinition, ok := current.Definitions[name]
		if !ok {
			diff.Additions = append(diff.Additions, "definition "+name)
			continue
		}
		appendMissingMembers(name, "relation", currentDefinition.Relations, desiredDefinition.Relations, &diff)
		appendMissingMembers(name, "permission", currentDefinition.Permissions, desiredDefinition.Permissions, &diff)
	}
	sort.Strings(diff.Additions)
	sort.Strings(diff.Conflicts)
	return diff
}

func (d SchemaDiff) Identical() bool {
	return len(d.Additions) == 0 && len(d.Conflicts) == 0
}

func (d SchemaDiff) Additive() bool {
	return len(d.Additions) > 0 && len(d.Conflicts) == 0
}

func parseDefinition(name, body string) (Definition, error) {
	definition := Definition{Relations: map[string]string{}, Permissions: map[string]string{}}
	for _, match := range declarationPattern.FindAllStringSubmatch(body, -1) {
		kind, member, expression := match[1], match[2], normalizeSchemaExpression(match[3])
		if _, exists := definition.Relations[member]; exists {
			return Definition{}, fmt.Errorf("definition %s has duplicate member %s", name, member)
		}
		if _, exists := definition.Permissions[member]; exists {
			return Definition{}, fmt.Errorf("definition %s has duplicate member %s", name, member)
		}
		if kind == "relation" {
			definition.Relations[member] = expression
		} else {
			definition.Permissions[member] = expression
		}
	}
	return definition, nil
}

func compareMembers(definition, kind string, current, desired map[string]string, diff *SchemaDiff) {
	for name, currentExpression := range current {
		desiredExpression, ok := desired[name]
		path := definition + "." + kind + "." + name
		if !ok {
			diff.Conflicts = append(diff.Conflicts, path+" exists only in active schema")
			continue
		}
		if currentExpression != desiredExpression {
			diff.Conflicts = append(diff.Conflicts, path+" changed")
		}
	}
}

func appendMissingMembers(definition, kind string, current, desired map[string]string, diff *SchemaDiff) {
	for name := range desired {
		if _, ok := current[name]; !ok {
			diff.Additions = append(diff.Additions, definition+"."+kind+"."+name)
		}
	}
}

func matchingBrace(text string, open int) (int, error) {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return -1, fmt.Errorf("missing closing brace")
}

func normalizeSchemaExpression(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
}

func stripSchemaComments(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '/' {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(text) && text[i] == '/' && text[i+1] == '*' {
			i += 2
			for i+1 < len(text) && !(text[i] == '*' && text[i+1] == '/') {
				if text[i] == '\n' {
					out.WriteByte('\n')
				}
				i++
			}
			if i+1 < len(text) {
				i += 2
			}
			continue
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}
