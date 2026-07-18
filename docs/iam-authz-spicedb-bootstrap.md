# IAM Authz / SpiceDB Bootstrap Model

## 1. Boundaries

```text
Casdoor = identity and directory source
  - local break-glass user
  - external IdP users
  - organization, displayed as availability zone
  - group tree

Gateway + Kernel = authentication
  - restore authn.Principal from trusted claim headers

IAM = authorization control plane
  - converts grants/bootstrap into SpiceDB relationships
  - checks subject + resource + permission before sensitive operations

SpiceDB = final authorization fact store
```

`Principal.roles` and external token claims are not final authorization facts. They can be displayed or used as import hints, but runtime checks must use SpiceDB.

## 2. Runtime check shape

Every sensitive operation is checked as:

```text
subject + resource + permission
```

When a business service calls IAM over gRPC, the trusted caller metadata carries
only stable authorization identity fields: subject ID, subject type, provider,
organization ID and project ID. Display profile fields such as name, username,
email and phone are not authorization facts and are not forwarded as gRPC
metadata. This keeps SpiceDB checks keyed by the Casdoor subject UUID and avoids
transport failures for non-ASCII display names.

Examples:

```text
user:<casdoor-sub> can create_groups on zone:aisphere
user:<casdoor-sub> can manage on group:aisphere/platform
group:aisphere/platform#member can group_manager on zone:aisphere
```

## 3. Role mapping

Product role keys are persisted as SpiceDB relations:

| Role key | Resource | Relation |
|---|---|---|
| zone_owner | zone | owner |
| zone_admin | zone | admin |
| user_viewer | zone | user_viewer |
| user_manager | zone | user_manager |
| group_viewer | zone | group_viewer |
| group_manager | zone | group_manager |
| permission_admin | zone | permission_admin |
| group_owner | group | owner |
| group_manager | group | manager |
| group_viewer | group | viewer |
| group_member | group | member |

At runtime, IAM checks permissions such as `create_groups`, `manage`, and `manage_permissions`.

## 4. Bootstrap local operator

Do not hard-code username-based allow logic such as `username == admin`.

The local Casdoor operator is granted through bootstrap relationships. The subject must be the stable Casdoor subject used by Gateway/Kernal Principal, not email.

```yaml
control_plane:
  bootstrap_admins:
    enabled: true
    subjects:
      - type: user
        id: <casdoor-local-operator-sub>
        zone_id: aisphere
        casdoor_org: aisphere
        role: zone_owner
        source: bootstrap
        reason: initial local operator
```

This writes:

```text
zone:aisphere#owner@user:<casdoor-local-operator-sub>
```

For compatibility with current generated IAM control-plane metadata, zone owner/admin bootstrap subjects are also written to built-in `iam:*#admin` relationships.

## 5. External users

External users may sign in through Casdoor federated identity providers. They are not administrators by default.

They receive permissions only through grants, for example:

```text
zone:aisphere#group_manager@user:<external-user-sub>
zone:aisphere#group_manager@group:aisphere/platform#member
```

## 6. Group management checks

Group write APIs use these checks:

| Operation | Resource | Permission |
|---|---|---|
| create top-level group | zone:{zone} | create_groups |
| create child group | group:{zone}/{parent} | create_child_groups |
| update group | group:{zone}/{group} | manage |
| delete group | group:{zone}/{group} | manage |

IAM writes group structure relationships after successful Casdoor group operations:

```text
group:{zone}/{group}#zone@zone:{zone}
group:{zone}/{child}#parent@group:{zone}/{parent}
```
