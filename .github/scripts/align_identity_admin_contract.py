#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    content = p.read_text(encoding="utf-8")
    if old not in content:
        raise RuntimeError(f"expected text not found in {path}: {old[:160]!r}")
    p.write_text(content.replace(old, new, 1), encoding="utf-8")


proto = Path("api/iam/v1/identity_admin.proto")
content = proto.read_text(encoding="utf-8")
replacements = {
    'authz: { action: "create" resource: "iam:org:{org_id}:user:*" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage_users" resource: "zone:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "update" resource: "iam:org:{org_id}:user:{user_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage_users" resource: "zone:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "disable" resource: "iam:org:{org_id}:user:{user_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage_users" resource: "zone:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "delete" resource: "iam:org:{org_id}:user:{user_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage_users" resource: "zone:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "create" resource: "iam:org:{org_id}:group:*" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "create_groups" resource: "zone:{org_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "update" resource: "iam:org:{org_id}:group:{group_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage" resource: "group:{org_id}/{group_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "delete" resource: "iam:org:{org_id}:group:{group_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage" resource: "group:{org_id}/{group_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "assign" resource: "iam:org:{org_id}:group:{group_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage" resource: "group:{org_id}/{group_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'authz: { action: "remove" resource: "iam:org:{org_id}:group:{group_id}" audience: "iam-service" mode: CHECK_ONLY }':
        'authz: { action: "manage" resource: "group:{org_id}/{group_id}" audience: "iam-service" mode: CHECK_ONLY }',
    'option (google.api.http) = { put: "/v1/iam/orgs/{org_id}/groups/{group_id}" body: "*" };':
        'option (google.api.http) = { patch: "/v1/iam/orgs/{org_id}/groups/{group_id}" body: "*" };',
}
for old, new in replacements.items():
    if old not in content:
        raise RuntimeError(f"expected identity admin contract text not found: {old}")
    content = content.replace(old, new, 1)
proto.write_text(content, encoding="utf-8")

replace_once(
    "internal/data/identity_mode_test.go",
    '''		relationships: store,
''',
    '''		projection: NewIdentityProjectionDispatcher(store, nil, nil),
''',
)

path = Path("internal/data/identity_mode.go")
content = path.read_text(encoding="utf-8")
replacements = {
    'groupMemberRelationship(req.GroupID, req.UserID)':
        'groupMemberRelationship(qualifiedGroupID(req.OrgID, req.GroupID), req.UserID)',
    'groupDeleteFilters(req.GroupID)':
        'groupDeleteFilters(req.OrgID, req.GroupID)',
    'groupMemberRelationship(groupID, user.ID)':
        'groupMemberRelationship(qualifiedGroupID(orgID, groupID), user.ID)',
    'groupMemberRelationship(group.ID, userID)':
        'groupMemberRelationship(qualifiedGroupID(firstNonEmpty(group.OrgID, orgID), group.ID), userID)',
}
for old, new in replacements.items():
    if old not in content:
        raise RuntimeError(f"expected projection expression not found: {old}")
    content = content.replace(old, new)

old_helpers = '''func groupTopologyRelationships(primary authn.Group, fallback authn.Group) []authz.Relationship { groupID := firstNonEmpty(primary.ID, fallback.ID); orgID := firstNonEmpty(primary.OrgID, fallback.OrgID); parentID := firstNonEmpty(primary.ParentID, fallback.ParentID); if groupID == "" { return nil }; rels := make([]authz.Relationship, 0, 3); if orgID != "" { rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "zone", Subject: authz.SubjectRef{Type: "zone", ID: orgID}}) }; if parentID != "" && parentID != groupID && parentID != orgID { rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "parent", Subject: authz.SubjectRef{Type: "group", ID: parentID}}, authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: parentID}, Relation: "member", Subject: authz.SubjectRef{Type: "group", ID: groupID, Relation: "member"}}) }; return rels }
func groupTopologyDeleteFilters(oldGroup authn.Group, fallback authn.Group) []authz.RelationshipFilter { groupID := firstNonEmpty(oldGroup.ID, fallback.ID); if groupID == "" { return nil }; return []authz.RelationshipFilter{{ResourceType: "group", ResourceID: groupID, Relation: "zone"}, {ResourceType: "group", ResourceID: groupID, Relation: "parent"}, {ResourceType: "group", Relation: "member", SubjectType: "group", SubjectID: groupID, SubjectRel: "member"}} }
func groupDeleteFilters(groupID string) []authz.RelationshipFilter { groupID = strings.TrimSpace(groupID); if groupID == "" { return nil }; return []authz.RelationshipFilter{{ResourceType: "group", ResourceID: groupID}, {SubjectType: "group", SubjectID: groupID}, {SubjectType: "group", SubjectID: groupID, SubjectRel: "member"}} }
func groupMemberRelationship(groupID, userID string) authz.Relationship { return authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: strings.TrimSpace(groupID)}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(userID)}} }
'''
new_helpers = '''func groupTopologyRelationships(primary authn.Group, fallback authn.Group) []authz.Relationship {
	orgID := firstNonEmpty(primary.OrgID, fallback.OrgID)
	groupID := qualifiedGroupID(orgID, firstNonEmpty(primary.ID, fallback.ID))
	parentID := qualifiedGroupID(orgID, firstNonEmpty(primary.ParentID, fallback.ParentID))
	if groupID == "" { return nil }
	rels := make([]authz.Relationship, 0, 3)
	if orgID != "" {
		rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "zone", Subject: authz.SubjectRef{Type: "zone", ID: orgID}})
	}
	if parentID != "" && parentID != groupID && parentID != orgID {
		rels = append(rels,
			authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "parent", Subject: authz.SubjectRef{Type: "group", ID: parentID}},
			authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: parentID}, Relation: "member", Subject: authz.SubjectRef{Type: "group", ID: groupID, Relation: "member"}},
		)
	}
	return rels
}
func groupTopologyDeleteFilters(oldGroup authn.Group, fallback authn.Group) []authz.RelationshipFilter {
	orgID := firstNonEmpty(oldGroup.OrgID, fallback.OrgID)
	groupID := qualifiedGroupID(orgID, firstNonEmpty(oldGroup.ID, fallback.ID))
	if groupID == "" { return nil }
	return []authz.RelationshipFilter{{ResourceType: "group", ResourceID: groupID, Relation: "zone"}, {ResourceType: "group", ResourceID: groupID, Relation: "parent"}, {ResourceType: "group", Relation: "member", SubjectType: "group", SubjectID: groupID, SubjectRel: "member"}}
}
func groupDeleteFilters(orgID, groupID string) []authz.RelationshipFilter {
	groupID = qualifiedGroupID(orgID, groupID)
	if groupID == "" { return nil }
	return []authz.RelationshipFilter{{ResourceType: "group", ResourceID: groupID}, {SubjectType: "group", SubjectID: groupID}, {SubjectType: "group", SubjectID: groupID, SubjectRel: "member"}}
}
func groupMemberRelationship(groupID, userID string) authz.Relationship { return authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: strings.TrimSpace(groupID)}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(userID)}} }
func qualifiedGroupID(orgID, groupID string) string {
	orgID = strings.Trim(strings.TrimSpace(orgID), "/")
	groupID = strings.Trim(strings.TrimSpace(groupID), "/")
	if groupID == "" { return "" }
	if orgID == "" || strings.HasPrefix(groupID, orgID+"/") { return groupID }
	return orgID + "/" + groupID
}
'''
if old_helpers not in content:
    raise RuntimeError("expected compact group projection helpers not found")
content = content.replace(old_helpers, new_helpers, 1)
path.write_text(content, encoding="utf-8")

print("Identity Admin contract and projection IDs aligned")
