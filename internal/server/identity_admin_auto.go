package server

import (
	v1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/aisphere-iam/internal/data"
	"github.com/aisphereio/aisphere-iam/internal/service"
	krpc "github.com/aisphereio/kernel/transportx/grpc"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

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

func registerIdentityAdminHTTP(srv *khttp.Server, resources *data.Resources) {
	identityAdminSvc := newIdentityAdminService(resources)
	if identityAdminSvc == nil {
		return
	}
	v1.RegisterIAMIdentityAdminServiceHTTPServer(srv, identityAdminSvc)
}

func registerIdentityAdminRPC(srv *krpc.Server, resources *data.Resources) {
	identityAdminSvc := newIdentityAdminService(resources)
	if identityAdminSvc == nil {
		return
	}
	v1.RegisterIAMIdentityAdminServiceServer(srv, identityAdminSvc)
}
