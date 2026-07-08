# AuthZ Bootstrap and Permission Console

This document records the first implementation slice for the IAM authorization model.

## 1. Principal contract

Gateway OIDC restores the authenticated caller into Kernel `authn.Principal`.

The authorization subject must always use the stable Kernel subject id:

```text
user:<principal.subject_id>
```

For Casdoor users, `principal.subject_id` must be the stable Casdoor user UUID. Do not use username, display name, email, or `external_id` as the SpiceDB subject.

Example:

```text
subject_id   = b22888d1-5cd0-4700-8e67-aa4a622fd715
subject_type = user
provider     = casdoor
external_id  = aisphere/use
org_id       = aisphere
username     = use
groups       = []
```

The correct SpiceDB subject is:

```text
user:b22888d1-5cd0-4700-8e67-aa4a622fd715
```

`groups = []` only means the current OIDC token did not project groups or roles. It does not mean the user has no SpiceDB permissions.

## 2. Directory permission checks

The generated proto policy for directory reads previously used old `iam:org:*` resources. Directory reads now skip generated authz and are checked in `IAMDirectoryService` directly against the `zone` resource.

| API | SpiceDB check |
| --- | --- |
| `GetUser` | `zone:{org_id}#view_users@user:{subject_id}` |
| `ListUsers` | `zone:{org_id}#view_users@user:{subject_id}` |
| `GetOrganization` | `zone:{org_id}#view_zone@user:{subject_id}` |
| `ListGroups` | `zone:{org_id}#view_groups@user:{subject_id}` |

This makes the user-list failure explicit and easier to diagnose:

```text
spicedb check permission failed: zone:aisphere#view_users@user:<uuid>
```

## 3. Bootstrap admin

IAM can bootstrap the initial Casdoor admin user by username.

```yaml
control_plane:
  bootstrap_admins:
    enabled: true
    subjects:
      - type: user
        username: admin
        zone_id: aisphere
        casdoor_org: aisphere
        role: zone_owner
        source: bootstrap
        reason: initial Casdoor admin user
```

At startup, IAM uses the configured Casdoor M2M admin client to resolve `aisphere/admin` into the stable user UUID and writes:

```text
zone:aisphere#owner@user:<admin_uuid>
```

For zone owner/admin bootstrap subjects, IAM also writes control-plane admin relationships for default IAM resources, including:

```text
iam:organization#admin@user:<admin_uuid>
iam:capability#admin@user:<admin_uuid>
iam:resource_type#admin@user:<admin_uuid>
iam:resource#admin@user:<admin_uuid>
iam:resource_binding#admin@user:<admin_uuid>
iam:external_resource_binding#admin@user:<admin_uuid>
iam:role_template#admin@user:<admin_uuid>
iam:grant#admin@user:<admin_uuid>
iam_authz:global#admin@user:<admin_uuid>
```

## 4. Permission Console resource

The SpiceDB schema now includes a global IAM authorization-control resource:

```zed
definition iam_authz {
  relation owner: user | service | service_account | group#member
  relation admin: user | service | service_account | group#member
  relation schema_admin: user | service | service_account | group#member
  relation auditor: user | service | service_account | group#member

  permission view_schema = owner + admin + schema_admin + auditor
  permission publish_schema = owner + admin + schema_admin
  permission view_relationships = owner + admin + schema_admin + auditor
  permission repair_relationships = owner + admin + schema_admin
  permission manage = owner + admin + schema_admin
}
```

This will back the UI module for:

- schema view / validate / publish / diff;
- relationship explorer;
- grant and revoke operations;
- permission explain;
- drift detection and repair.

## 5. Next implementation slices

1. Add IAM AuthZ Admin API:
   - `GetAuthzSchema`
   - `ValidateAuthzSchema`
   - `PublishAuthzSchema`
   - `ListRelationships`
   - `GrantRelationship`
   - `RevokeRelationship`
   - `ExplainPermission`
2. Add frontend Permission Console tab.
3. Add Resource/Subject selector components.
4. Add audit events for schema publish, grant, revoke, and repair.
5. Add integration tests for admin bootstrap and user-list authorization.
