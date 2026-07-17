# Grant Resource Selector Design

## Goal

Make access assignment understandable without requiring operators to know or type SpiceDB resource IDs. The selected identity source in the application header is the only zone context. Every grant must target a real resource registered in the IAM resource catalog.

## User Experience

The access-assignment form keeps two resource controls:

1. **Resource type** selects the kind of scope, such as identity source, organization, project, or Skill.
2. **Effective resource** selects one real resource of that type from the current identity source.

The header identity source is not repeated as an editable form field. The form shows a compact read-only context label, for example `Current identity source: aisphere`, so the operator can see the active scope without choosing it again.

### Zone resources

When the selected resource type is `zone`:

- The effective resource is automatically set to the current `identityOrg` value.
- The resource selector is replaced by a read-only value such as `Current identity source: aisphere`.
- Roles such as User Viewer, User Manager, Group Viewer, Group Manager, and Permission Admin therefore require no resource ID input.

### Other resource types

For `group`, `project`, `skill`, and other concrete resource types:

- The effective resource is required.
- The control is a searchable combobox populated only from the IAM resource catalog for the current identity source and selected resource type.
- Operators may search by display name, slug, path, or canonical ID/UUID.
- Typing filters or searches existing resources; it never creates a free-form value.
- The submitted grant always contains the selected resource's canonical `ref.id`.
- When no resource matches, the UI explains that no registered resource is available and disables submission.

Each option displays enough disambiguating information:

```text
Aisphere IAM Front
iam-front / 550e8400-e29b-41d4-a716-446655440000
```

Duplicate display names are allowed because selection is based on canonical ID. Path, slug, and ID make duplicate names distinguishable.

## Data Flow

1. The application passes the header's `identityOrg` into `GrantEditor`.
2. Selecting a resource type clears the previous resource and role.
3. For `zone`, the editor assigns `resourceId = identityOrg`.
4. For other types, the editor queries the resource catalog using `identityOrg`, resource type, and the current search text.
5. Selecting an option stores its canonical `ref.id`.
6. The grant request sends `{ resource: { type, id }, subject, roleKey, ... }`.
7. The backend remains authoritative and rejects resources that do not exist or do not belong to the current zone.

Changing the header identity source invalidates the current draft. Resource, subject, and role selections are cleared before data for the new identity source is shown. This prevents a resource or subject from one zone being submitted under another zone.

## API Contract

`ListResourcesRequest` gains an optional `query` field. The change is proto-first and generated clients are regenerated through the repository's normal API generation command.

The service passes `query` into `data.ListOptions.Q`. Repository search covers:

- `id`
- `slug`
- `display_name`
- `path`

The frontend debounces remote searches. Empty search text returns the normal first page for the selected resource type. Search results remain scoped by `org_id`, resource type, and active status.

No resolve-by-arbitrary-name endpoint and no free-form resource fallback are introduced.

## UI States

- **Loading:** Show a loading state inside the resource combobox and disable submission.
- **Empty:** Show `No grantable resources of this type are registered in the current identity source` and disable submission.
- **Load failure:** Show a specific resource-list error with retry; do not replace the selector with a text field.
- **Stale selection:** If a selected resource disappears, is archived, or is no longer returned, clear it and require a new selection.
- **Zone context:** Show the current identity source as a read-only resolved scope.
- **Identity-source change:** Clear the zone-scoped draft and reload users, groups, resources, and grants.

## Current Assignment Display

Assignments show human-readable scope rather than raw type/ID syntax:

- `Identity source: aisphere`
- `Organization: Platform Team`
- `Project: Aisphere IAM Front`

The canonical ID remains available as secondary text or a tooltip for diagnosis.

## Compatibility Fix

The frontend grant adapter and mutation types must match the generated JSON contract. Request fields use `roleKey` and `expiresAt`, not handwritten snake_case aliases. This keeps the UI request aligned with the proto-generated client.

## Testing

Frontend tests cover:

- `zone` automatically resolves to `identityOrg` and renders no editable resource input.
- Concrete resource types require selection from catalog results.
- Search matches display name, slug, path, and ID/UUID results.
- Unmatched text cannot be submitted as a resource.
- Empty and failed resource queries disable submission.
- Changing `identityOrg` clears zone-scoped selections.
- Grant payload uses canonical resource ID, `roleKey`, and `expiresAt`.

Backend tests cover:

- `query` is scoped by zone and resource type.
- Search matches ID, slug, display name, and path.
- Results from other zones are never returned.
- Grant creation still rejects missing, archived, or cross-zone resources.

## Out of Scope

- Creating resources from the grant form.
- Granting access to unregistered resources.
- Allowing an operator to override the header identity source inside the form.
- Changing SpiceDB relationship semantics or the DTM projection flow.
