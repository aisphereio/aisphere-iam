package service

import (
	"context"

	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authn"
)

// GetGroup reads one Casdoor-backed group after applying the same
// zone-scoped directory authorization contract used by ListGroups.
func (s *IAMDirectoryService) GetGroup(ctx context.Context, req *v1.GetGroupRequest) (*v1.Group, error) {
	if err := s.requireZonePermission(ctx, req.GetOrgId(), "view_groups"); err != nil {
		return nil, err
	}
	if s.deps.Identity == nil {
		return nil, authn.ErrIdentityBackendFailed("identity provider is not configured", nil)
	}
	group, err := s.deps.Identity.GetGroup(ctx, req.GetOrgId(), req.GetGroupId())
	if err != nil {
		return nil, err
	}
	return groupToProto(group), nil
}
