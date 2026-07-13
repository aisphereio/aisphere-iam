package server

import (
	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/serverx"
)

func IAMModules() []serverx.ServiceModule {
	return []serverx.ServiceModule{
		v1.IAMAuthServiceKernelModule(),
		v1.IAMDirectoryServiceKernelModule(),
		v1.IAMIdentityAdminServiceKernelModule(),
		v1.IAMGroupAdminServiceKernelModule(),
		v1.IAMPermissionServiceKernelModule(),
		projectv1.ProjectServiceKernelModule(),
		resourcev1.ResourceServiceKernelModule(),
		grantv1.GrantServiceKernelModule(),
	}
}

func IAMCatalog() serverx.ServiceCatalog {
	return serverx.MustServiceCatalog(IAMModules()...)
}
