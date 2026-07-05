// Package graph contains the minimal IAM -> ReBAC projection adapter.
//
// Domain services call this package after their own control-plane records are
// persisted. The implementation delegates to kernel/authz interfaces so IAM
// does not import SpiceDB directly in biz code.
package graph

import (
	"context"
	"strings"

	"github.com/aisphereio/kernel/authz"
)

// Projector writes and deletes ReBAC relationships through kernel/authz.
type Projector struct {
	writer authz.RelationshipWriter
}

func NewProjector(writer authz.RelationshipWriter) *Projector {
	return &Projector{writer: writer}
}

func (p *Projector) Enabled() bool { return p != nil && p.writer != nil }

func (p *Projector) Write(ctx context.Context, relationships ...authz.Relationship) (authz.WriteResult, error) {
	if p == nil || p.writer == nil || len(relationships) == 0 {
		return authz.WriteResult{}, nil
	}
	compact := make([]authz.Relationship, 0, len(relationships))
	for _, rel := range relationships {
		if rel.Resource.IsZero() || rel.Subject.IsZero() || strings.TrimSpace(rel.Relation) == "" {
			continue
		}
		compact = append(compact, rel)
	}
	if len(compact) == 0 {
		return authz.WriteResult{}, nil
	}
	return p.writer.WriteRelationships(ctx, compact...)
}

func (p *Projector) Delete(ctx context.Context, filter authz.RelationshipFilter) (authz.WriteResult, error) {
	if p == nil || p.writer == nil {
		return authz.WriteResult{}, nil
	}
	return p.writer.DeleteRelationships(ctx, filter)
}

func Object(typ, id string) authz.ObjectRef {
	return authz.ObjectRef{Type: strings.TrimSpace(typ), ID: strings.TrimSpace(id)}
}

func Subject(typ, id, rel string) authz.SubjectRef {
	return authz.SubjectRef{Type: strings.TrimSpace(typ), ID: strings.TrimSpace(id), Relation: strings.TrimSpace(rel)}
}
