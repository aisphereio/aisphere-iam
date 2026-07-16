package server

import (
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/serverx"
)

func IAMBindings(resources *data.Resources, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, groupSvc *service.IAMGroupAdminService, permSvc *service.IAMPermissionService, authzAdminSvc *service.IAMAuthorizationAdminService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService, accessQuerySvc *service.AccessQueryService) []serverx.ServiceBinding {
	modules := IAMModules()
	return []serverx.ServiceBinding{
		{Module: modules[0], Implementation: authSvc},
		{Module: modules[1], Implementation: dirSvc},
		{Module: modules[2], Implementation: newDirectoryProjectionService(resources)},
		{Module: modules[3], Implementation: newIdentityAdminService(resources)},
		{Module: modules[4], Implementation: groupSvc},
		{Module: modules[5], Implementation: permSvc},
		{Module: modules[6], Implementation: authzAdminSvc},
		{Module: modules[7], Implementation: projectSvc},
		{Module: modules[8], Implementation: resourceSvc},
		{Module: modules[9], Implementation: grantSvc},
		{Module: modules[10], Implementation: accessQuerySvc},
	}
}

func newDirectoryProjectionService(resources *data.Resources) *service.IAMDirectoryProjectionService {
	return service.NewIAMDirectoryProjectionService(newDirectoryProjectionOps(resources))
}

func newDirectoryProjectionOps(resources *data.Resources) *service.DirectoryProjectionOps {
	if resources == nil {
		return nil
	}
	return service.NewDirectoryProjectionOpsFromDeps(resources.Identity, resources.AuthzAdmin, resources.IdentityProjection)
}

func newIdentityAdminService(resources *data.Resources) *service.IAMIdentityAdminService {
	if resources == nil || resources.Identity == nil {
		return nil
	}
	return service.NewIAMIdentityAdminService(service.IAMDeps{
		Tokens:   resources.Tokens,
		Identity: resources.Identity,
		Authz:    resources.AuthzAdmin,
	})
}
