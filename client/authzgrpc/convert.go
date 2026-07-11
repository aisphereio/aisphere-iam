package authzgrpc

import (
	iamv1 "github.com/aisphereio/aisphere-iam/api/iam/v1"
	"github.com/aisphereio/kernel/authz"
)

func checkRequestToProto(in authz.CheckRequest) *iamv1.CheckPermissionRequest {
	return &iamv1.CheckPermissionRequest{Subject: subjectToProto(in.Subject), Resource: objectToProto(in.Resource), Permission: in.Permission, OrgId: in.OrgID, ProjectId: in.ProjectID}
}

func decisionFromProto(in *iamv1.CheckPermissionReply) authz.Decision {
	if in == nil {
		return authz.Deny("empty IAM authorization decision")
	}
	return authz.Decision{Allowed: in.GetAllowed(), Effect: authz.DecisionEffect(in.GetEffect()), Reason: in.GetReason(), ConsistencyToken: in.GetConsistencyToken()}
}

func objectToProto(in authz.ObjectRef) *iamv1.ObjectRef {
	return &iamv1.ObjectRef{Type: in.Type, Id: in.ID}
}
func objectFromProto(in *iamv1.ObjectRef) authz.ObjectRef {
	if in == nil {
		return authz.ObjectRef{}
	}
	return authz.ObjectRef{Type: in.GetType(), ID: in.GetId()}
}
func subjectToProto(in authz.SubjectRef) *iamv1.SubjectRef {
	return &iamv1.SubjectRef{Type: in.Type, Id: in.ID, Relation: in.Relation}
}
func subjectFromProto(in *iamv1.SubjectRef) authz.SubjectRef {
	if in == nil {
		return authz.SubjectRef{}
	}
	return authz.SubjectRef{Type: in.GetType(), ID: in.GetId(), Relation: in.GetRelation()}
}
func relationshipToProto(in authz.Relationship) *iamv1.Relationship {
	return &iamv1.Relationship{Resource: objectToProto(in.Resource), Relation: in.Relation, Subject: subjectToProto(in.Subject)}
}
func relationshipFromProto(in *iamv1.Relationship) authz.Relationship {
	if in == nil {
		return authz.Relationship{}
	}
	return authz.Relationship{Resource: objectFromProto(in.GetResource()), Relation: in.GetRelation(), Subject: subjectFromProto(in.GetSubject())}
}
func filterToProto(in authz.RelationshipFilter) *iamv1.RelationshipFilter {
	return &iamv1.RelationshipFilter{ResourceType: in.ResourceType, ResourceId: in.ResourceID, Relation: in.Relation, SubjectType: in.SubjectType, SubjectId: in.SubjectID, SubjectRelation: in.SubjectRel}
}
func listRequestToProto(in authz.RelationshipFilter) *iamv1.ListRelationshipsRequest {
	return &iamv1.ListRelationshipsRequest{ResourceType: in.ResourceType, ResourceId: in.ResourceID, Relation: in.Relation, SubjectType: in.SubjectType, SubjectId: in.SubjectID, SubjectRelation: in.SubjectRel}
}
