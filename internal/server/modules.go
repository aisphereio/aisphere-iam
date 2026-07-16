package server

import (
	accessv1 "github.com/aisphereio/aisphere-iam/api/iam/access/v1"
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
		v1.IAMDirectoryProjectionServiceKernelModule(),
		v1.IAMIdentityAdminServiceKernelModule(),
		v1.IAMGroupAdminServiceKernelModule(),
		v1.IAMPermissionServiceKernelModule(),
		v1.IAMAuthorizationAdminServiceKernelModule(),
		projectv1.ProjectServiceKernelModule(),
		resourcev1.ResourceServiceKernelModule(),
		grantv1.GrantServiceKernelModule(),
		accessv1.AccessQueryServiceKernelModule(),
	}
}

func IAMCatalog() serverx.ServiceCatalog {
	return serverx.MustServiceCatalog(IAMModules()...)
}
