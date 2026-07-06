package server

import (
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	"github.com/aisphereio/kernel/serverx"
)

func IAMBindings(resources *data.Resources, authSvc *service.IAMAuthService, dirSvc *service.IAMDirectoryService, permSvc *service.IAMPermissionService, projectSvc *service.ProjectService, resourceSvc *service.ResourceService, grantSvc *service.GrantService) []serverx.ServiceBinding {
	modules := IAMModules()
	return []serverx.ServiceBinding{
		{Module: modules[0], Implementation: authSvc},
		{Module: modules[1], Implementation: dirSvc},
		{Module: modules[2], Implementation: newIdentityAdminService(resources)},
		{Module: modules[3], Implementation: permSvc},
		{Module: modules[4], Implementation: projectSvc},
		{Module: modules[5], Implementation: resourceSvc},
		{Module: modules[6], Implementation: grantSvc},
	}
}
