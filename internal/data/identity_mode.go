package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authz"
	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/dtmx"
)

const (
	IdentityModeCasdoorLocal = "casdoor_local"
	IdentityModeExternalOIDC = "external_oidc"

	identityAuthZProjectionTopic       = "iam.identity.authz.projection"
	identityAuthZProjectionOperationUp = "write"
	identityAuthZProjectionOperationRm = "delete"

	IdentityProjectionStatusPending    = "pending"
	IdentityProjectionStatusSubmitted  = "submitted"
	IdentityProjectionStatusProjecting = "projecting"
	IdentityProjectionStatusSynced     = "synced"
	IdentityProjectionStatusFailed     = "failed"
	IdentityProjectionStatusArchived   = "archived"
)

func identityMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", IdentityModeCasdoorLocal:
		return IdentityModeCasdoorLocal, nil
	case IdentityModeExternalOIDC:
		return IdentityModeExternalOIDC, nil
	default:
		return "", fmt.Errorf("unsupported authn identity_mode: %s", mode)
	}
}

func identityForMode(mode string, next authn.IdentityAdmin) (authn.IdentityAdmin, error) {
	resolved, err := identityMode(mode)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return nil, nil
	}
	if resolved == IdentityModeExternalOIDC {
		return externalOIDCIdentityAdmin{next: next}, nil
	}
	return next, nil
}

type IdentityAuthZProjectionPayload struct {
	Operation     string                     `json:"operation"`
	Relationships []authz.Relationship       `json:"relationships,omitempty"`
	Filters       []authz.RelationshipFilter `json:"filters,omitempty"`
}

type IdentityAuthZBranchPayload struct {
	EventID string                         `json:"event_id,omitempty"`
	Payload IdentityAuthZProjectionPayload `json:"payload"`
}

type IdentityProjectionEventModel struct {
	ID            string     `gorm:"column:id;primaryKey"`
	Source        string     `gorm:"column:source;index"`
	AggregateType string     `gorm:"column:aggregate_type;index"`
	AggregateID   string     `gorm:"column:aggregate_id;index"`
	Operation     string     `gorm:"column:operation"`
	PayloadJSON   string     `gorm:"column:payload_json;type:jsonb"`
	Status        string     `gorm:"column:status;index"`
	RetryCount    int        `gorm:"column:retry_count"`
	LastError     string     `gorm:"column:last_error;type:text"`
	NextRunAt     *time.Time `gorm:"column:next_run_at;index"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (IdentityProjectionEventModel) TableName() string { return "iam_directory_projection_events" }

type IdentityProjectionDispatcher struct {
	writer authz.RelationshipWriter
	dtm    dtmx.Manager
	db     dbx.DB
	now    func() time.Time
}

type identityProjectionConfig struct {
	dispatcher *IdentityProjectionDispatcher
	writer     authz.RelationshipWriter
	dtm        dtmx.Manager
	db         dbx.DB
}

type IdentityProjectionOption func(*identityProjectionConfig)

func WithIdentityProjectionDispatcher(dispatcher *IdentityProjectionDispatcher) IdentityProjectionOption {
	return func(cfg *identityProjectionConfig) { cfg.dispatcher = dispatcher }
}

func WithIdentityProjectionDTM(dtm dtmx.Manager) IdentityProjectionOption {
	return func(cfg *identityProjectionConfig) { cfg.dtm = dtm }
}

func WithIdentityProjectionDB(db dbx.DB) IdentityProjectionOption {
	return func(cfg *identityProjectionConfig) { cfg.db = db }
}

func NewIdentityProjectionDispatcher(writer authz.RelationshipWriter, dtm dtmx.Manager, db dbx.DB) *IdentityProjectionDispatcher {
	if writer == nil {
		return nil
	}
	return &IdentityProjectionDispatcher{writer: writer, dtm: dtm, db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (d *IdentityProjectionDispatcher) EnsureStore(ctx context.Context) error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.AutoMigrate(ctx, &IdentityProjectionEventModel{})
}

func (d *IdentityProjectionDispatcher) Dispatch(ctx context.Context, source, aggregateType, aggregateID string, payload IdentityAuthZProjectionPayload) error {
	if d == nil || d.writer == nil {
		return authz.ErrBackendFailed("identity authz projection dispatcher is not configured", nil)
	}
	payload = normalizeProjectionPayload(payload)
	if isProjectionPayloadEmpty(payload) {
		return nil
	}
	eventID := newIdentityProjectionID()
	if d.db != nil {
		nextRun := d.now().Add(time.Minute)
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		now := d.now()
		event := &IdentityProjectionEventModel{ID: eventID, Source: firstNonEmpty(source, "iam_api"), AggregateType: strings.TrimSpace(aggregateType), AggregateID: strings.TrimSpace(aggregateID), Operation: payload.Operation, PayloadJSON: string(body), Status: IdentityProjectionStatusPending, NextRunAt: &nextRun, CreatedAt: now, UpdatedAt: now}
		if err := d.db.Create(ctx, event); err != nil {
			return err
		}
	}
	return d.submit(ctx, eventID, payload)
}

func (d *IdentityProjectionDispatcher) ApplyBranch(ctx context.Context, branch IdentityAuthZBranchPayload) (authz.WriteResult, error) {
	if d == nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("identity authz projection dispatcher is not configured", nil)
	}
	if branch.EventID != "" {
		_ = d.mark(ctx, branch.EventID, IdentityProjectionStatusProjecting, "")
	}
	wr, err := ApplyIdentityAuthZProjection(ctx, d.writer, branch.Payload)
	if err != nil {
		if branch.EventID != "" {
			_ = d.markFailure(ctx, branch.EventID, err)
		}
		return wr, err
	}
	if branch.EventID != "" {
		_ = d.mark(ctx, branch.EventID, IdentityProjectionStatusSynced, "")
	}
	return wr, nil
}

func (d *IdentityProjectionDispatcher) CompensateBranch(ctx context.Context, branch IdentityAuthZBranchPayload) (authz.WriteResult, error) {
	if d == nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("identity authz projection dispatcher is not configured", nil)
	}
	wr, err := CompensateIdentityAuthZProjection(ctx, d.writer, branch.Payload)
	if err != nil {
		if branch.EventID != "" {
			_ = d.markFailure(ctx, branch.EventID, err)
		}
		return wr, err
	}
	if branch.EventID != "" {
		_ = d.mark(ctx, branch.EventID, IdentityProjectionStatusArchived, "")
	}
	return wr, nil
}

func (d *IdentityProjectionDispatcher) RetryOnce(ctx context.Context, limit int) (int, error) {
	if d == nil || d.db == nil {
		return 0, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var events []IdentityProjectionEventModel
	now := d.now()
	if err := d.db.GORM(ctx).Where("status IN ? AND (next_run_at IS NULL OR next_run_at <= ?)", []string{IdentityProjectionStatusPending, IdentityProjectionStatusSubmitted, IdentityProjectionStatusFailed}, now).Order("created_at ASC").Limit(limit).Find(&events).Error; err != nil {
		return 0, err
	}
	processed := 0
	for _, event := range events {
		var payload IdentityAuthZProjectionPayload
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			_ = d.markFailure(ctx, event.ID, err)
			continue
		}
		if err := d.submit(ctx, event.ID, payload); err != nil {
			_ = d.markFailure(ctx, event.ID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

func (d *IdentityProjectionDispatcher) StartRetryWorker(ctx context.Context, interval time.Duration) {
	if d == nil || d.db == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = d.RetryOnce(ctx, 100)
		}
	}
}

func (d *IdentityProjectionDispatcher) submit(ctx context.Context, eventID string, payload IdentityAuthZProjectionPayload) error {
	if d.dtm != nil && d.dtm.Enabled() {
		gid, err := d.dtm.NewGID(ctx)
		if err != nil {
			return err
		}
		_ = d.mark(ctx, eventID, IdentityProjectionStatusSubmitted, "")
		branch := IdentityAuthZBranchPayload{EventID: eventID, Payload: payload}
		saga := dtmx.NewSaga(gid, identityAuthZProjectionTopic).AddHTTP("identity-authz", d.dtm.BranchURL("iam/identity-authz/apply"), d.dtm.BranchURL("iam/identity-authz/compensate"), branch)
		_, err = d.dtm.SubmitSaga(ctx, saga)
		return err
	}
	branch := IdentityAuthZBranchPayload{EventID: eventID, Payload: payload}
	_, err := d.ApplyBranch(ctx, branch)
	return err
}

func (d *IdentityProjectionDispatcher) mark(ctx context.Context, eventID, status, lastError string) error {
	if d == nil || d.db == nil || strings.TrimSpace(eventID) == "" {
		return nil
	}
	return d.db.Update(ctx, &IdentityProjectionEventModel{}, "id = ?", []any{eventID}, map[string]any{"status": status, "last_error": lastError, "updated_at": d.now()})
}

func (d *IdentityProjectionDispatcher) markFailure(ctx context.Context, eventID string, err error) error {
	if d == nil || d.db == nil || strings.TrimSpace(eventID) == "" || err == nil {
		return nil
	}
	nextRun := d.now().Add(time.Minute)
	if updateErr := d.db.Update(ctx, &IdentityProjectionEventModel{}, "id = ?", []any{eventID}, map[string]any{"status": IdentityProjectionStatusFailed, "last_error": err.Error(), "next_run_at": nextRun, "updated_at": d.now()}); updateErr != nil {
		return updateErr
	}
	return d.db.Increment(ctx, &IdentityProjectionEventModel{}, "id = ?", []any{eventID}, "retry_count", 1)
}

func BindIdentityAuthZ(next authn.IdentityAdmin, relationships authz.RelationshipWriter, opts ...IdentityProjectionOption) authn.IdentityAdmin {
	if next == nil || relationships == nil {
		return next
	}
	cfg := identityProjectionConfig{writer: relationships}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.dispatcher == nil {
		cfg.dispatcher = NewIdentityProjectionDispatcher(relationships, cfg.dtm, cfg.db)
	}
	return authzProjectingIdentityAdmin{IdentityAdmin: next, projection: cfg.dispatcher}
}

// externalOIDCIdentityAdmin protects the upstream user/org directory while still allowing Aisphere-owned groups and membership to be managed.
type externalOIDCIdentityAdmin struct{ next authn.IdentityAdmin }

func (a externalOIDCIdentityAdmin) ExchangeCode(ctx context.Context, req authn.AuthCodeExchangeRequest) (authn.TokenSet, authn.Principal, error) {
	return a.next.ExchangeCode(ctx, req)
}
func (a externalOIDCIdentityAdmin) RefreshToken(ctx context.Context, req authn.RefreshTokenRequest) (authn.TokenSet, error) {
	return a.next.RefreshToken(ctx, req)
}
func (a externalOIDCIdentityAdmin) VerifyToken(ctx context.Context, req authn.VerifyTokenRequest) (authn.Principal, error) {
	return a.next.VerifyToken(ctx, req)
}
func (a externalOIDCIdentityAdmin) RevokeToken(ctx context.Context, req authn.RevokeTokenRequest) error {
	return a.next.RevokeToken(ctx, req)
}
func (a externalOIDCIdentityAdmin) GetUser(ctx context.Context, orgID, userID string) (authn.User, error) {
	return a.next.GetUser(ctx, orgID, userID)
}
func (a externalOIDCIdentityAdmin) FindUsers(ctx context.Context, filter authn.UserFilter) ([]authn.User, error) {
	return a.next.FindUsers(ctx, filter)
}
func (a externalOIDCIdentityAdmin) CreateUser(ctx context.Context, req authn.CreateUserRequest) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("CreateUser")
}
func (a externalOIDCIdentityAdmin) UpdateUser(ctx context.Context, req authn.UpdateUserRequest) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("UpdateUser")
}
func (a externalOIDCIdentityAdmin) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	return externalDirectoryReadOnlyError("DeleteUser")
}
func (a externalOIDCIdentityAdmin) UpsertUser(ctx context.Context, user authn.User) (authn.User, error) {
	return authn.User{}, externalDirectoryReadOnlyError("UpsertUser")
}
func (a externalOIDCIdentityAdmin) DisableUser(ctx context.Context, orgID, userID string) error {
	return externalDirectoryReadOnlyError("DisableUser")
}
func (a externalOIDCIdentityAdmin) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, externalDirectoryReadOnlyError("CreateOrganization")
}
func (a externalOIDCIdentityAdmin) GetOrganization(ctx context.Context, orgID string) (authn.Organization, error) {
	return a.next.GetOrganization(ctx, orgID)
}
func (a externalOIDCIdentityAdmin) UpdateOrganization(ctx context.Context, req authn.UpdateOrganizationRequest) (authn.Organization, error) {
	return authn.Organization{}, externalDirectoryReadOnlyError("UpdateOrganization")
}
func (a externalOIDCIdentityAdmin) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	return externalDirectoryReadOnlyError("DeleteOrganization")
}
func (a externalOIDCIdentityAdmin) CreateApplication(ctx context.Context, req authn.CreateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, externalDirectoryReadOnlyError("CreateApplication")
}
func (a externalOIDCIdentityAdmin) GetApplication(ctx context.Context, orgID, appID string) (authn.Application, error) {
	return a.next.GetApplication(ctx, orgID, appID)
}
func (a externalOIDCIdentityAdmin) UpdateApplication(ctx context.Context, req authn.UpdateApplicationRequest) (authn.Application, error) {
	return authn.Application{}, externalDirectoryReadOnlyError("UpdateApplication")
}
func (a externalOIDCIdentityAdmin) DeleteApplication(ctx context.Context, req authn.DeleteApplicationRequest) error {
	return externalDirectoryReadOnlyError("DeleteApplication")
}
func (a externalOIDCIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	return a.next.CreateGroup(ctx, req)
}
func (a externalOIDCIdentityAdmin) GetGroup(ctx context.Context, orgID, groupID string) (authn.Group, error) {
	return a.next.GetGroup(ctx, orgID, groupID)
}
func (a externalOIDCIdentityAdmin) ListGroups(ctx context.Context, filter authn.GroupFilter) ([]authn.Group, error) {
	return a.next.ListGroups(ctx, filter)
}
func (a externalOIDCIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	return a.next.UpdateGroup(ctx, req)
}
func (a externalOIDCIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	return a.next.DeleteGroup(ctx, req)
}
func (a externalOIDCIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return a.next.AssignUserToGroup(ctx, req)
}
func (a externalOIDCIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	return a.next.RemoveUserFromGroup(ctx, req)
}

func externalDirectoryReadOnlyError(operation string) error {
	return authn.ErrIdentityBackendFailed("identity user/org directory is read-only in external_oidc mode: "+operation, nil)
}

type authzProjectingIdentityAdmin struct {
	authn.IdentityAdmin
	projection *IdentityProjectionDispatcher
}

func (a authzProjectingIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group, err := a.IdentityAdmin.CreateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	if err := a.projectWrite(ctx, "iam_api", "group", firstNonEmpty(group.ID, req.Group.ID), groupTopologyRelationships(group, req.Group)...); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}
func (a authzProjectingIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	var oldGroup authn.Group
	if req.Group.OrgID != "" && req.Group.ID != "" {
		oldGroup, _ = a.IdentityAdmin.GetGroup(ctx, req.Group.OrgID, req.Group.ID)
	}
	group, err := a.IdentityAdmin.UpdateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	if err := a.projectDelete(ctx, "iam_api", "group", firstNonEmpty(group.ID, req.Group.ID), groupTopologyDeleteFilters(oldGroup, req.Group), nil); err != nil {
		return authn.Group{}, err
	}
	if err := a.projectWrite(ctx, "iam_api", "group", firstNonEmpty(group.ID, req.Group.ID), groupTopologyRelationships(group, req.Group)...); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}
func (a authzProjectingIdentityAdmin) DeleteGroup(ctx context.Context, req authn.DeleteGroupRequest) error {
	if err := a.IdentityAdmin.DeleteGroup(ctx, req); err != nil {
		return err
	}
	return a.projectDelete(ctx, "iam_api", "group", req.GroupID, groupDeleteFilters(req.OrgID, req.GroupID), nil)
}
func (a authzProjectingIdentityAdmin) AssignUserToGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	if err := a.IdentityAdmin.AssignUserToGroup(ctx, req); err != nil {
		return err
	}
	rels := []authz.Relationship{groupMemberRelationship(qualifiedGroupID(req.OrgID, req.GroupID), req.UserID)}
	if strings.TrimSpace(req.OrgID) != "" {
		rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: strings.TrimSpace(req.OrgID)}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(req.UserID)}})
	}
	return a.projectWrite(ctx, "iam_api", "group_membership", req.GroupID, rels...)
}
func (a authzProjectingIdentityAdmin) RemoveUserFromGroup(ctx context.Context, req authn.AssignUserToGroupRequest) error {
	if err := a.IdentityAdmin.RemoveUserFromGroup(ctx, req); err != nil {
		return err
	}
	return a.projectDelete(ctx, "iam_api", "group_membership", req.GroupID, []authz.RelationshipFilter{{ResourceType: "group", ResourceID: strings.TrimSpace(req.GroupID), Relation: "member", SubjectType: "user", SubjectID: strings.TrimSpace(req.UserID)}}, []authz.Relationship{groupMemberRelationship(qualifiedGroupID(req.OrgID, req.GroupID), req.UserID)})
}

func (a authzProjectingIdentityAdmin) projectWrite(ctx context.Context, source, aggregateType, aggregateID string, rels ...authz.Relationship) error {
	return a.projection.Dispatch(ctx, source, aggregateType, aggregateID, IdentityAuthZProjectionPayload{Operation: identityAuthZProjectionOperationUp, Relationships: rels})
}
func (a authzProjectingIdentityAdmin) projectDelete(ctx context.Context, source, aggregateType, aggregateID string, filters []authz.RelationshipFilter, rels []authz.Relationship) error {
	return a.projection.Dispatch(ctx, source, aggregateType, aggregateID, IdentityAuthZProjectionPayload{Operation: identityAuthZProjectionOperationRm, Filters: filters, Relationships: rels})
}

func ApplyIdentityAuthZProjection(ctx context.Context, writer authz.RelationshipWriter, payload IdentityAuthZProjectionPayload) (authz.WriteResult, error) {
	if writer == nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("authz relationship writer is not configured", nil)
	}
	payload = normalizeProjectionPayload(payload)
	switch payload.Operation {
	case identityAuthZProjectionOperationUp:
		return writer.WriteRelationships(ctx, payload.Relationships...)
	case identityAuthZProjectionOperationRm:
		var out authz.WriteResult
		for _, filter := range payload.Filters {
			part, err := writer.DeleteRelationships(ctx, filter)
			out.Deleted += part.Deleted
			if part.ConsistencyToken != "" {
				out.ConsistencyToken = part.ConsistencyToken
			}
			if err != nil {
				return out, err
			}
		}
		return out, nil
	default:
		return authz.WriteResult{}, fmt.Errorf("unsupported identity authz projection operation: %s", payload.Operation)
	}
}
func CompensateIdentityAuthZProjection(ctx context.Context, writer authz.RelationshipWriter, payload IdentityAuthZProjectionPayload) (authz.WriteResult, error) {
	if writer == nil {
		return authz.WriteResult{}, authz.ErrBackendFailed("authz relationship writer is not configured", nil)
	}
	payload = normalizeProjectionPayload(payload)
	switch payload.Operation {
	case identityAuthZProjectionOperationUp:
		var out authz.WriteResult
		for _, rel := range payload.Relationships {
			part, err := writer.DeleteRelationships(ctx, authz.RelationshipFilter{ResourceType: rel.Resource.Type, ResourceID: rel.Resource.ID, Relation: rel.Relation, SubjectType: rel.Subject.Type, SubjectID: rel.Subject.ID, SubjectRel: rel.Subject.Relation})
			out.Deleted += part.Deleted
			if part.ConsistencyToken != "" {
				out.ConsistencyToken = part.ConsistencyToken
			}
			if err != nil {
				return out, err
			}
		}
		return out, nil
	case identityAuthZProjectionOperationRm:
		return writer.WriteRelationships(ctx, payload.Relationships...)
	default:
		return authz.WriteResult{}, fmt.Errorf("unsupported identity authz projection operation: %s", payload.Operation)
	}
}

func BuildDirectoryProjectionRelationships(ctx context.Context, identity authn.IdentityAdmin, orgID string) ([]authz.Relationship, error) {
	orgID = strings.TrimSpace(orgID)
	if identity == nil || orgID == "" {
		return nil, nil
	}
	groups, err := identity.ListGroups(ctx, authn.GroupFilter{OrgID: orgID, Limit: 10000})
	if err != nil {
		return nil, err
	}
	users, err := identity.FindUsers(ctx, authn.UserFilter{OrgID: orgID, Limit: 10000})
	if err != nil {
		return nil, err
	}
	rels := make([]authz.Relationship, 0, len(groups)*3+len(users)*2)
	for _, user := range users {
		if strings.TrimSpace(user.ID) == "" {
			continue
		}
		rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: orgID}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(user.ID)}})
		for _, groupID := range user.Groups {
			rels = append(rels, groupMemberRelationship(qualifiedGroupID(orgID, groupID), user.ID))
		}
	}
	for _, group := range groups {
		rels = append(rels, groupTopologyRelationships(group, authn.Group{OrgID: orgID})...)
		for _, userID := range group.Users {
			rels = append(rels, groupMemberRelationship(qualifiedGroupID(firstNonEmpty(group.OrgID, orgID), group.ID), userID))
		}
	}
	return dedupeRelationships(rels), nil
}
func DetectDirectoryProjectionDrift(ctx context.Context, reader authz.RelationshipReader, desired []authz.Relationship) (missing []authz.Relationship, err error) {
	if reader == nil {
		return nil, authz.ErrBackendFailed("authz relationship reader is not configured", nil)
	}
	for _, rel := range dedupeRelationships(desired) {
		current, err := reader.ReadRelationships(ctx, authz.RelationshipFilter{ResourceType: rel.Resource.Type, ResourceID: rel.Resource.ID, Relation: rel.Relation, SubjectType: rel.Subject.Type, SubjectID: rel.Subject.ID, SubjectRel: rel.Subject.Relation})
		if err != nil {
			return missing, err
		}
		if len(current) == 0 {
			missing = append(missing, rel)
		}
	}
	return missing, nil
}

func groupTopologyRelationships(primary authn.Group, fallback authn.Group) []authz.Relationship {
	orgID := firstNonEmpty(primary.OrgID, fallback.OrgID)
	groupID := qualifiedGroupID(orgID, firstNonEmpty(primary.ID, fallback.ID))
	parentID := qualifiedGroupID(orgID, firstNonEmpty(primary.ParentID, fallback.ParentID))
	if groupID == "" {
		return nil
	}
	rels := make([]authz.Relationship, 0, 3)
	if orgID != "" {
		rels = append(rels, authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "zone", Subject: authz.SubjectRef{Type: "zone", ID: orgID}})
	}
	if parentID != "" && parentID != groupID && parentID != orgID {
		rels = append(rels,
			authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: groupID}, Relation: "parent", Subject: authz.SubjectRef{Type: "group", ID: parentID}},
			authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: parentID}, Relation: "member", Subject: authz.SubjectRef{Type: "group", ID: groupID, Relation: "member"}},
		)
	}
	return rels
}
func groupTopologyDeleteFilters(oldGroup authn.Group, fallback authn.Group) []authz.RelationshipFilter {
	orgID := firstNonEmpty(oldGroup.OrgID, fallback.OrgID)
	groupID := qualifiedGroupID(orgID, firstNonEmpty(oldGroup.ID, fallback.ID))
	if groupID == "" {
		return nil
	}
	return []authz.RelationshipFilter{{ResourceType: "group", ResourceID: groupID, Relation: "zone"}, {ResourceType: "group", ResourceID: groupID, Relation: "parent"}, {ResourceType: "group", Relation: "member", SubjectType: "group", SubjectID: groupID, SubjectRel: "member"}}
}
func groupDeleteFilters(orgID, groupID string) []authz.RelationshipFilter {
	groupID = qualifiedGroupID(orgID, groupID)
	if groupID == "" {
		return nil
	}
	return []authz.RelationshipFilter{{ResourceType: "group", ResourceID: groupID}, {SubjectType: "group", SubjectID: groupID}, {SubjectType: "group", SubjectID: groupID, SubjectRel: "member"}}
}
func groupMemberRelationship(groupID, userID string) authz.Relationship {
	return authz.Relationship{Resource: authz.ObjectRef{Type: "group", ID: strings.TrimSpace(groupID)}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(userID)}}
}
func qualifiedGroupID(orgID, groupID string) string {
	orgID = strings.Trim(strings.TrimSpace(orgID), "/")
	groupID = strings.Trim(strings.TrimSpace(groupID), "/")
	if groupID == "" {
		return ""
	}
	if orgID == "" || strings.HasPrefix(groupID, orgID+"/") {
		return groupID
	}
	return orgID + "/" + groupID
}

func normalizeProjectionPayload(payload IdentityAuthZProjectionPayload) IdentityAuthZProjectionPayload {
	payload.Operation = strings.TrimSpace(payload.Operation)
	if payload.Operation == "" {
		payload.Operation = identityAuthZProjectionOperationUp
	}
	rels := make([]authz.Relationship, 0, len(payload.Relationships))
	for _, rel := range payload.Relationships {
		if rel.Resource.IsZero() || strings.TrimSpace(rel.Relation) == "" || rel.Subject.IsZero() {
			continue
		}
		rels = append(rels, rel)
	}
	payload.Relationships = dedupeRelationships(rels)
	filters := make([]authz.RelationshipFilter, 0, len(payload.Filters))
	for _, filter := range payload.Filters {
		if filter.ResourceType == "" && filter.ResourceID == "" && filter.Relation == "" && filter.SubjectType == "" && filter.SubjectID == "" && filter.SubjectRel == "" {
			continue
		}
		filters = append(filters, filter)
	}
	payload.Filters = filters
	return payload
}
func isProjectionPayloadEmpty(payload IdentityAuthZProjectionPayload) bool {
	return len(payload.Relationships) == 0 && len(payload.Filters) == 0
}
func dedupeRelationships(in []authz.Relationship) []authz.Relationship {
	seen := map[string]struct{}{}
	out := make([]authz.Relationship, 0, len(in))
	for _, rel := range in {
		if rel.Resource.IsZero() || rel.Subject.IsZero() || strings.TrimSpace(rel.Relation) == "" {
			continue
		}
		key := rel.Resource.Type + ":" + rel.Resource.ID + "#" + rel.Relation + "@" + rel.Subject.Type + ":" + rel.Subject.ID + "#" + rel.Subject.Relation
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, rel)
	}
	return out
}
func newIdentityProjectionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "dirproj_" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("dirproj_%d", time.Now().UnixNano())
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var _ authn.IdentityAdmin = externalOIDCIdentityAdmin{}
var _ authn.IdentityAdmin = authzProjectingIdentityAdmin{}
