package server

import (
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
)

func newIdentityAdminService(resources *data.Resources) *service.IAMIdentityAdminService {
	if resources == nil || resources.Identity == nil {
		return nil
	}
	return service.NewIAMIdentityAdminService(service.IAMDeps{
		Login:    resources.Login,
		Logout:   resources.Logout,
		Tokens:   resources.Tokens,
		Profile:  resources.Profile,
		Identity: resources.Identity,
		Authz:    resources.AuthzAdmin,
	})
}
