// Package accessquery provides a unified permission query model that aggregates
// Postgres Grant records, SpiceDB effective permissions, directory membership,
// and resource inheritance into a single Entitlement view.
package accessquery

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/aisphereio/kernel/authz"

	"github.com/aisphereio/aisphere-iam/internal/data"
)

// ResourceRef is a lightweight resource reference.
type ResourceRef struct{ Type, ID string }

// SubjectRef is a lightweight subject reference.
type SubjectRef struct{ Type, ID, Relation string }

// EntitlementSourceType describes how an entitlement was derived.
type EntitlementSourceType string

const (
	SourceDirectGrant         EntitlementSourceType = "DIRECT_GRANT"
	SourceGroupGrant          EntitlementSourceType = "GROUP_GRANT"
	SourceParentInheritance   EntitlementSourceType = "PARENT_INHERITANCE"
	SourceOrgInheritance      EntitlementSourceType = "ORG_INHERITANCE"
	SourcePlatformInheritance EntitlementSourceType = "PLATFORM_INHERITANCE"
)

// Entitlement represents a single effective permission entry for a subject
// on a resource, with its derivation source.
type Entitlement struct {
	ID               string
	Subject          SubjectRef
	Resource         ResourceRef
	RoleKey          string
	Permissions      []string
	SourceType       EntitlementSourceType
	SourceSubject    SubjectRef
	SourceResource   ResourceRef
	GrantID          string
	RevocableHere    bool
	ExpiresAt        *time.Time
	ConsistencyToken string
}

// ListSubjectEntitlementsRequest
type ListSubjectEntitlementsRequest struct {
	OrgID        string
	Subject      SubjectRef
	ResourceType string
	PageSize     int
	PageToken    string
}

// ListSubjectEntitlementsReply
type ListSubjectEntitlementsReply struct {
	Entitlements  []Entitlement
	NextPageToken string
	TotalSize     int64
}

// ListResourceAccessRequest
type ListResourceAccessRequest struct {
	OrgID       string
	Resource    ResourceRef
	SubjectType string
	PageSize    int
	PageToken   string
}

// ListResourceAccessReply
type ListResourceAccessReply struct {
	Entitlements  []Entitlement
	NextPageToken string
	TotalSize     int64
}

// PreviewGrantRequest
type PreviewGrantRequest struct {
	OrgID    string
	Resource ResourceRef
	RoleKey  string
	Subject  SubjectRef
}

// PreviewGrantReply
type PreviewGrantReply struct {
	Permissions      []string
	AlreadyGranted   bool
	ExistingGrantID  string
	ConsistencyToken string
}

// Service implements the AccessQueryService business logic.
type Service struct {
	repo              data.ControlPlaneRepository
	relationshipStore authz.RelationshipStore
	now               func() time.Time
}

// NewService creates a new AccessQueryService.
// svc provides the full authorization control-plane surface including
// relationship reads needed for inheritance tracing.
func NewService(repo data.ControlPlaneRepository, svc authz.Service) *Service {
	return &Service{
		repo:              repo,
		relationshipStore: svc,
		now:               time.Now,
	}
}

// ListSubjectEntitlements returns all effective permissions for a subject.
func (s *Service) ListSubjectEntitlements(ctx context.Context, req ListSubjectEntitlementsRequest) (*ListSubjectEntitlementsReply, error) {
	var result []Entitlement

	// 1. Direct grants from Postgres
	directGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
		OrgID:       req.OrgID,
		SubjectType: req.Subject.Type,
		SubjectID:   req.Subject.ID,
		Active:      boolPtr(true),
		Size:        1000,
	})
	if err != nil {
		return nil, fmt.Errorf("list direct grants: %w", err)
	}
	for i := range directGrants.Items {
		g := &directGrants.Items[i]
		perms := s.getPermissions(ctx, g.RoleTemplateID, g.ResourceType, g.RoleKey)
		result = append(result, Entitlement{
			ID:              g.ID,
			Subject:         SubjectRef{Type: g.SubjectType, ID: g.SubjectID, Relation: g.SubjectRelation},
			Resource:        ResourceRef{Type: g.ResourceType, ID: g.ResourceID},
			RoleKey:         g.RoleKey,
			Permissions:     perms,
			SourceType:      SourceDirectGrant,
			SourceSubject:   SubjectRef{Type: g.SubjectType, ID: g.SubjectID},
			GrantID:         g.ID,
			RevocableHere:   true,
			ExpiresAt:       g.ExpiresAt,
			ConsistencyToken: g.ID,
		})
	}

	// 2. Group membership inheritance
	if s.relationshipStore != nil {
		groups, err := s.relationshipStore.ReadRelationships(ctx, authz.RelationshipFilter{
			ResourceType: "group",
			Relation:     "member",
			SubjectType:  req.Subject.Type,
			SubjectID:    req.Subject.ID,
		})
		if err == nil {
			for _, g := range groups {
				groupGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
					OrgID:       req.OrgID,
					SubjectType: "group",
					SubjectID:   g.Resource.ID,
					Active:      boolPtr(true),
					Size:        1000,
				})
				if err != nil {
					continue
				}
				for j := range groupGrants.Items {
					gg := &groupGrants.Items[j]
					perms := s.getPermissions(ctx, gg.RoleTemplateID, gg.ResourceType, gg.RoleKey)
					result = append(result, Entitlement{
						ID:             inheritedID("group", gg.ID, req.Subject.ID),
						Subject:        SubjectRef{Type: req.Subject.Type, ID: req.Subject.ID},
						Resource:       ResourceRef{Type: gg.ResourceType, ID: gg.ResourceID},
						RoleKey:        gg.RoleKey,
						Permissions:    perms,
						SourceType:     SourceGroupGrant,
						SourceSubject:  SubjectRef{Type: "group", ID: g.Resource.ID},
						GrantID:        gg.ID,
						RevocableHere:  false,
						ExpiresAt:      gg.ExpiresAt,
					})
				}
			}
		}

		// 3. Parent resource inheritance
		seen := map[string]bool{}
		for _, e := range result {
			key := e.Resource.Type + ":" + e.Resource.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			s.traceParentInheritance(ctx, req, e.Resource, &result)
		}

// 4. Zone-level (org) permissions
			// Only subjects with owner or admin relation on the zone should inherit
			// zone-level grants.  All zone members (member relation) get view_zone
			// via SpiceDB permission computation, not via Postgres grant inheritance.
			zoneRels, err := s.relationshipStore.ReadRelationships(ctx, authz.RelationshipFilter{
				ResourceType: "zone",
				SubjectType:  req.Subject.Type,
				SubjectID:    req.Subject.ID,
			})
			if err == nil {
				for _, zr := range zoneRels {
					// Skip member-only relationships — zone grants are for admins/owners only.
					if zr.Relation != "owner" && zr.Relation != "admin" {
						continue
					}
					zoneGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
						OrgID:        req.OrgID,
						ResourceType: "zone",
						ResourceID:   zr.Resource.ID,
						Active:       boolPtr(true),
						Size:         1000,
					})
					if err != nil {
						continue
					}
					for j := range zoneGrants.Items {
						zg := &zoneGrants.Items[j]
						perms := s.getPermissions(ctx, zg.RoleTemplateID, zg.ResourceType, zg.RoleKey)
						result = append(result, Entitlement{
							ID:             inheritedID("zone", zg.ID, req.Subject.ID),
							Subject:        SubjectRef{Type: req.Subject.Type, ID: req.Subject.ID},
							Resource:       ResourceRef{Type: "zone", ID: zr.Resource.ID},
							RoleKey:        zg.RoleKey,
							Permissions:    perms,
							SourceType:     SourceOrgInheritance,
							SourceResource: ResourceRef{Type: "zone", ID: zr.Resource.ID},
							GrantID:        zg.ID,
							RevocableHere:  false,
							ExpiresAt:      zg.ExpiresAt,
						})
					}
				}
			}

		// 5. Platform-level permissions
		platformRels, err := s.relationshipStore.ReadRelationships(ctx, authz.RelationshipFilter{
			ResourceType: "platform",
			SubjectType:  req.Subject.Type,
			SubjectID:    req.Subject.ID,
		})
		if err == nil {
			for _, pr := range platformRels {
				platformGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
					ResourceType: "platform",
					ResourceID:   pr.Resource.ID,
					Active:       boolPtr(true),
					Size:         1000,
				})
				if err != nil {
					continue
				}
				for j := range platformGrants.Items {
					pg := &platformGrants.Items[j]
					perms := s.getPermissions(ctx, pg.RoleTemplateID, pg.ResourceType, pg.RoleKey)
					result = append(result, Entitlement{
						ID:             inheritedID("platform", pg.ID, req.Subject.ID),
						Subject:        SubjectRef{Type: req.Subject.Type, ID: req.Subject.ID},
						Resource:       ResourceRef{Type: "platform", ID: pr.Resource.ID},
						RoleKey:        pg.RoleKey,
						Permissions:    perms,
						SourceType:     SourcePlatformInheritance,
						SourceResource: ResourceRef{Type: "platform", ID: pr.Resource.ID},
						GrantID:        pg.ID,
						RevocableHere:  false,
						ExpiresAt:      pg.ExpiresAt,
					})
				}
			}
		}
	}

	// Filter by resource_type if specified
	if req.ResourceType != "" {
		var filtered []Entitlement
		for _, e := range result {
			if e.Resource.Type == req.ResourceType {
				filtered = append(filtered, e)
			}
		}
		result = filtered
	}

	// Deduplicate by ID
	result = dedupEntitlements(result)

	total := int64(len(result))

	// Apply pagination
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := 0
	if req.PageToken != "" {
		start = pageFromToken(req.PageToken)
	}
	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}
	var nextToken string
	if end < len(result) {
		nextToken = pageToken(end)
	}
	if start > len(result) {
		start = len(result)
	}
	if start > end {
		start = end
	}

	return &ListSubjectEntitlementsReply{
		Entitlements:  result[start:end],
		NextPageToken: nextToken,
		TotalSize:     total,
	}, nil
}

// ListResourceAccess returns all subjects with effective access to a resource.
func (s *Service) ListResourceAccess(ctx context.Context, req ListResourceAccessRequest) (*ListResourceAccessReply, error) {
	var result []Entitlement

	// 1. Direct grants on this resource
	directGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
		OrgID:        req.OrgID,
		ResourceType: req.Resource.Type,
		ResourceID:   req.Resource.ID,
		Active:       boolPtr(true),
		Size:         1000,
	})
	if err != nil {
		return nil, fmt.Errorf("list direct grants: %w", err)
	}
	for i := range directGrants.Items {
		g := &directGrants.Items[i]
		if req.SubjectType != "" && g.SubjectType != req.SubjectType {
			continue
		}
		perms := s.getPermissions(ctx, g.RoleTemplateID, g.ResourceType, g.RoleKey)
		result = append(result, Entitlement{
			ID:              g.ID,
			Subject:         SubjectRef{Type: g.SubjectType, ID: g.SubjectID, Relation: g.SubjectRelation},
			Resource:        ResourceRef{Type: g.ResourceType, ID: g.ResourceID},
			RoleKey:         g.RoleKey,
			Permissions:     perms,
			SourceType:      SourceDirectGrant,
			SourceSubject:   SubjectRef{Type: g.SubjectType, ID: g.SubjectID},
			GrantID:         g.ID,
			RevocableHere:   true,
			ExpiresAt:       g.ExpiresAt,
			ConsistencyToken: g.ID,
		})
	}

	// 2. Group grants that apply to this resource
	if s.relationshipStore != nil {
		groupGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
			OrgID:        req.OrgID,
			ResourceType: req.Resource.Type,
			ResourceID:   req.Resource.ID,
			SubjectType:  "group",
			Active:       boolPtr(true),
			Size:         1000,
		})
		if err == nil {
			for i := range groupGrants.Items {
				gg := &groupGrants.Items[i]
				if req.SubjectType != "" && req.SubjectType != "user" {
					continue
				}
				// Find all members of this group
				members, err := s.relationshipStore.ReadRelationships(ctx, authz.RelationshipFilter{
					ResourceType: "group",
					ResourceID:   gg.SubjectID,
					Relation:     "member",
				})
				if err != nil {
					continue
				}
				for _, m := range members {
					perms := s.getPermissions(ctx, gg.RoleTemplateID, gg.ResourceType, gg.RoleKey)
					result = append(result, Entitlement{
						ID:            inheritedID("group_ent", gg.ID, m.Subject.ID),
						Subject:       SubjectRef{Type: m.Subject.Type, ID: m.Subject.ID},
						Resource:      ResourceRef{Type: gg.ResourceType, ID: gg.ResourceID},
						RoleKey:       gg.RoleKey,
						Permissions:   perms,
						SourceType:    SourceGroupGrant,
						SourceSubject: SubjectRef{Type: "group", ID: gg.SubjectID},
						GrantID:       gg.ID,
						RevocableHere: false,
						ExpiresAt:     gg.ExpiresAt,
					})
				}
			}
		}

		// 3. Parent resource inheritance
		parents, err := s.relationshipStore.ReadRelationships(ctx, authz.RelationshipFilter{
			ResourceType: req.Resource.Type,
			ResourceID:   req.Resource.ID,
			Relation:     "parent",
		})
		if err == nil {
			for _, p := range parents {
				parentGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
					OrgID:        req.OrgID,
					ResourceType: p.Subject.Type,
					ResourceID:   p.Subject.ID,
					Active:       boolPtr(true),
					Size:         1000,
				})
				if err != nil {
					continue
				}
				for j := range parentGrants.Items {
					pg := &parentGrants.Items[j]
					if req.SubjectType != "" && pg.SubjectType != req.SubjectType {
						continue
					}
					perms := s.getPermissions(ctx, pg.RoleTemplateID, pg.ResourceType, pg.RoleKey)
					result = append(result, Entitlement{
						ID:             inheritedID("parent", pg.ID, req.Resource.ID),
						Subject:        SubjectRef{Type: pg.SubjectType, ID: pg.SubjectID, Relation: pg.SubjectRelation},
						Resource:       ResourceRef{Type: req.Resource.Type, ID: req.Resource.ID},
						RoleKey:        pg.RoleKey,
						Permissions:    perms,
						SourceType:     SourceParentInheritance,
						SourceResource: ResourceRef{Type: p.Subject.Type, ID: p.Subject.ID},
						GrantID:        pg.ID,
						RevocableHere:  false,
						ExpiresAt:      pg.ExpiresAt,
					})
				}
			}
		}
	}

	// Deduplicate
	result = dedupEntitlements(result)

	total := int64(len(result))

	// Pagination
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := 0
	if req.PageToken != "" {
		start = pageFromToken(req.PageToken)
	}
	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}
	var nextToken string
	if end < len(result) {
		nextToken = pageToken(end)
	}
	if start > len(result) {
		start = len(result)
	}
	if end > len(result) {
		end = len(result)
	}

	return &ListResourceAccessReply{
		Entitlements:  result[start:end],
		NextPageToken: nextToken,
		TotalSize:     total,
	}, nil
}

// PreviewGrant checks what permissions a subject would receive.
func (s *Service) PreviewGrant(ctx context.Context, req PreviewGrantRequest) (*PreviewGrantReply, error) {
	// Get the role template to see what permissions it grants
	roleTemplates, err := s.repo.ListRoleTemplates(ctx, req.Resource.Type)
	if err != nil {
		return nil, fmt.Errorf("list role templates: %w", err)
	}
	var role *data.RoleTemplateModel
	for i := range roleTemplates {
		if roleTemplates[i].RoleKey == req.RoleKey {
			role = &roleTemplates[i]
			break
		}
	}
	if role == nil {
		return nil, fmt.Errorf("role template not found: %s/%s", req.Resource.Type, req.RoleKey)
	}

	// Check if already granted
	existingGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
		OrgID:        req.OrgID,
		ResourceType: req.Resource.Type,
		ResourceID:   req.Resource.ID,
		SubjectType:  req.Subject.Type,
		SubjectID:    req.Subject.ID,
		RoleKey:      req.RoleKey,
		Active:       boolPtr(true),
		Size:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("check existing grants: %w", err)
	}

	alreadyGranted := len(existingGrants.Items) > 0
	var existingGrantID string
	if alreadyGranted {
		existingGrantID = existingGrants.Items[0].ID
	}

	return &PreviewGrantReply{
		Permissions:      role.Permissions,
		AlreadyGranted:   alreadyGranted,
		ExistingGrantID:  existingGrantID,
		ConsistencyToken: role.ID,
	}, nil
}

// traceParentInheritance recursively traces parent resource inheritance.
func (s *Service) traceParentInheritance(ctx context.Context, req ListSubjectEntitlementsRequest, resource ResourceRef, result *[]Entitlement) {
	if s.relationshipStore == nil {
		return
	}
	parents, err := s.relationshipStore.ReadRelationships(ctx, authz.RelationshipFilter{
		ResourceType: resource.Type,
		ResourceID:   resource.ID,
		Relation:     "parent",
	})
	if err != nil || len(parents) == 0 {
		return
	}
	for _, p := range parents {
		parentGrants, err := s.repo.ListGrants(ctx, data.ListOptions{
			OrgID:        req.OrgID,
			ResourceType: p.Subject.Type,
			ResourceID:   p.Subject.ID,
			SubjectType:  req.Subject.Type,
			SubjectID:    req.Subject.ID,
			Active:       boolPtr(true),
			Size:         1000,
		})
		if err != nil {
			continue
		}
		for j := range parentGrants.Items {
			pg := &parentGrants.Items[j]
			perms := s.getPermissions(ctx, pg.RoleTemplateID, pg.ResourceType, pg.RoleKey)
			*result = append(*result, Entitlement{
				ID:             inheritedID("parent", pg.ID, req.Subject.ID),
				Subject:        SubjectRef{Type: req.Subject.Type, ID: req.Subject.ID},
				Resource:       ResourceRef{Type: resource.Type, ID: resource.ID},
				RoleKey:        pg.RoleKey,
				Permissions:    perms,
				SourceType:     SourceParentInheritance,
				SourceResource: ResourceRef{Type: p.Subject.Type, ID: p.Subject.ID},
				GrantID:        pg.ID,
				RevocableHere:  false,
				ExpiresAt:      pg.ExpiresAt,
			})
		}
		// Recurse up the parent chain
		s.traceParentInheritance(ctx, req, ResourceRef{Type: p.Subject.Type, ID: p.Subject.ID}, result)
	}
}

// getPermissions retrieves permissions for a role template.
func (s *Service) getPermissions(ctx context.Context, roleTemplateID, resourceType, roleKey string) []string {
	if roleTemplateID != "" {
		rt, err := s.repo.GetRoleTemplate(ctx, roleTemplateID)
		if err == nil && rt != nil {
			return rt.Permissions
		}
	}
	// Fallback: list role templates by resource type and role key
	templates, err := s.repo.ListRoleTemplates(ctx, resourceType)
	if err != nil {
		return nil
	}
	for i := range templates {
		if templates[i].RoleKey == roleKey {
			return templates[i].Permissions
		}
	}
	return nil
}

// dedupEntitlements removes duplicate entitlements by ID.
func dedupEntitlements(ents []Entitlement) []Entitlement {
	seen := map[string]bool{}
	out := make([]Entitlement, 0, len(ents))
	for _, e := range ents {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		out = append(out, e)
	}
	return out
}

// inheritedID creates a unique ID for an inherited entitlement.
func inheritedID(prefix, grantID, subjectID string) string {
	h := sha256.Sum256([]byte(prefix + ":" + grantID + ":" + subjectID))
	return fmt.Sprintf("inherited:%x", h[:8])
}

// pageToken creates a pagination token from an index.
func pageToken(index int) string {
	return fmt.Sprintf("%d", index)
}

// pageFromToken parses a pagination token into an index.
func pageFromToken(token string) int {
	var index int
	fmt.Sscanf(token, "%d", &index)
	return index
}

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}