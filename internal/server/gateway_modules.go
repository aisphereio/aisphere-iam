package server

import (
	grantv1 "github.com/aisphereio/aisphere-iam/api/iam/grant/v1"
	projectv1 "github.com/aisphereio/aisphere-iam/api/iam/project/v1"
	resourcev1 "github.com/aisphereio/aisphere-iam/api/iam/resource/v1"
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/serverx"
)

// IAMGatewayModules returns every generated IAM service module that should be
// published into the Gateway route registry.
//
// Keep service discovery centralized here instead of scattering generated module
// calls through cmd/aisphere-iam/main.go.
func IAMGatewayModules() []serverx.ServiceModule {
	return []serverx.ServiceModule{
		v1.IAMAuthServiceKernelModule(),
		v1.IAMDirectoryServiceKernelModule(),
		v1.IAMIdentityAdminServiceKernelModule(),
		v1.IAMPermissionServiceKernelModule(),
		projectv1.ProjectServiceKernelModule(),
		resourcev1.ResourceServiceKernelModule(),
		grantv1.GrantServiceKernelModule(),
	}
}
