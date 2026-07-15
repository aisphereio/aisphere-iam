package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
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

	defaultPlatformID = "global"
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
	reader authz.RelationshipReader
	dtm    dtmx.Manager
	db     dbx.DB
	now    func() time.Time
}

type identityProjectionConfig struct {
	dispatcher *IdentityProjectionDispatcher
	writer     authz.RelationshipWriter
	reader     authz.RelationshipReader
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

func WithIdentityProjectionReader(reader authz.RelationshipReader) IdentityProjectionOption {
	return func(cfg *identityProjectionConfig) { cfg.reader = reader }
}

func NewIdentityProjectionDispatcher(writer authz.RelationshipWriter, reader authz.RelationshipReader, dtm dtmx.Manager, db dbx.DB) *IdentityProjectionDispatcher {
	if writer == nil {
		return nil
	}
	return &IdentityProjectionDispatcher{writer: writer, reader: reader, dtm: dtm, db: db, now: func() time.Time { return time.Now().UTC() }}
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
		cfg.dispatcher = NewIdentityProjectionDispatcher(relationships, cfg.reader, cfg.dtm, cfg.db)
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

// groupOwnerCtxKey carries the principal that should receive a group#owner
// relationship when a group is created.  HTTP handlers set it via
// WithGroupOwner so the owner grant flows through the DTM projection layer
// instead of a separate direct SpiceDB write.
type groupOwnerCtxKey struct{}

// WithGroupOwner attaches a group owner subject to the context so that
// authzProjectingIdentityAdmin.CreateGroup projects a group#owner relationship
// for that subject.  Passing an empty subject clears it.
func WithGroupOwner(ctx context.Context, owner authz.SubjectRef) context.Context {
	if owner.Type == "" || owner.ID == "" {
		return ctx
	}
	return context.WithValue(ctx, groupOwnerCtxKey{}, owner)
}

// groupOwnerFromContext returns the group owner subject attached to the
// context, if any.
func groupOwnerFromContext(ctx context.Context) (authz.SubjectRef, bool) {
	owner, ok := ctx.Value(groupOwnerCtxKey{}).(authz.SubjectRef)
	return owner, ok
}

func (a authzProjectingIdentityAdmin) CreateOrganization(ctx context.Context, req authn.CreateOrganizationRequest) (authn.Organization, error) {
	organization, err := a.IdentityAdmin.CreateOrganization(ctx, req)
	if err != nil {
		return authn.Organization{}, err
	}
	orgID := firstNonEmpty(organization.ID, req.Organization.ID)
	if err := a.projectWrite(ctx, "iam_api", "zone", orgID, platformZoneRelationship(orgID)); err != nil {
		return authn.Organization{}, err
	}
	return organization, nil
}

func (a authzProjectingIdentityAdmin) CreateGroup(ctx context.Context, req authn.CreateGroupRequest) (authn.Group, error) {
	group, err := a.IdentityAdmin.CreateGroup(ctx, req)
	if err != nil {
		return authn.Group{}, err
	}
	rels := groupTopologyRelationships(group, req.Group)
	// Grant the creating principal a group#owner relationship so they can
	// manage the group and create child groups beneath it.  The owner subject
	// is injected via WithGroupOwner by the HTTP handler so this grant is
	// projected through the DTM/outbox layer rather than written directly to
	// SpiceDB (which would bypass compensation and retry).
	if owner, ok := groupOwnerFromContext(ctx); ok && owner.Type != "" && owner.ID != "" {
		groupID := qualifiedGroupID(firstNonEmpty(group.OrgID, req.Group.OrgID), firstNonEmpty(group.ID, req.Group.ID))
		if groupID != "" {
			rels = append(rels, authz.Relationship{
				Resource: authz.ObjectRef{Type: "group", ID: groupID},
				Relation: "owner",
				Subject:  owner,
			})
		}
	}
	if err := a.projectWrite(ctx, "iam_api", "group", firstNonEmpty(group.ID, req.Group.ID), rels...); err != nil {
		return authn.Group{}, err
	}
	return group, nil
}
func (a authzProjectingIdentityAdmin) UpdateGroup(ctx context.Context, req authn.UpdateGroupRequest) (authn.Group, error) {
	var oldGroup authn.Group
	if req.Group.OrgID != "" && req.Group.ID != "" {
		oldGroup, _ = a.IdentityAdmin.GetGroup(ctx, req.Group.OrgID, req.Group.ID)
	}
	// Group Name is the immutable identifier backing SpiceDB object IDs
	// (e.g. "group:aisphere/platform"); renaming would desync authorization
	// topology.  Reject any attempt to change it.  An empty Name (caller
	// omitted the field) is allowed and means "keep current".
	if newName := strings.TrimSpace(req.Group.Name); newName != "" && oldGroup.Name != "" && !strings.EqualFold(newName, oldGroup.Name) {
		return authn.Group{}, authn.ErrInvalidTokenRequest(fmt.Sprintf("group name is immutable: cannot rename %q to %q", oldGroup.Name, newName))
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
	filters := groupDeleteFilters(req.OrgID, req.GroupID)
	rels, _ := a.captureRelationships(ctx, filters)
	return a.projectDelete(ctx, "iam_api", "group", req.GroupID, filters, rels)
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
	groupID := qualifiedGroupID(req.OrgID, req.GroupID)
	userID := strings.TrimSpace(req.UserID)
	orgID := strings.TrimSpace(req.OrgID)
	if groupID == "" || userID == "" {
		return nil
	}
	// Always remove the group#member relationship.  Compensation data (rels)
	// lets the DTM saga undo this if the downstream delete fails.
	filters := []authz.RelationshipFilter{
		{ResourceType: "group", ResourceID: groupID, Relation: "member", SubjectType: "user", SubjectID: userID},
	}
	rels := []authz.Relationship{
		groupMemberRelationship(groupID, userID),
	}
	// Only revoke zone#member when the user has no remaining groups in this
	// zone.  A user may belong to multiple groups; leaving one should not strip
	// zone-level visibility (view_zone, view_users, etc.) while they are still a
	// member of another group.  If ListGroups fails we err on the side of
	// preserving zone membership so we never accidentally lock a user out.
	remainingGroups, listErr := a.IdentityAdmin.ListGroups(ctx, authn.GroupFilter{OrgID: orgID, UserID: userID, Limit: 1000})
	if listErr == nil && len(remainingGroups) == 0 {
		filters = append(filters, authz.RelationshipFilter{
			ResourceType: "zone", ResourceID: orgID, Relation: "member",
			SubjectType: "user", SubjectID: userID,
		})
		rels = append(rels, zoneMemberRelationship(orgID, userID))
	}
	return a.projectDelete(ctx, "iam_api", "group_membership", req.GroupID, filters, rels)
}

// DeleteUser removes the user from the identity provider and then purges all
// IAM-owned SpiceDB relationships where the user appears as a subject.  This
// prevents a deleted user from retaining zone#member, group#member, or
// platform role relationships that would still satisfy permission checks.
func (a authzProjectingIdentityAdmin) DeleteUser(ctx context.Context, req authn.DeleteUserRequest) error {
	if err := a.IdentityAdmin.DeleteUser(ctx, req); err != nil {
		return err
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil
	}
	filters := userSubjectDeleteFilters(userID)
	rels, _ := a.captureRelationships(ctx, filters)
	return a.projectDelete(ctx, "iam_api", "user", userID, filters, rels)
}

// DisableUser disables the user in the identity provider and, like DeleteUser,
// purges the user's SpiceDB relationships so disabled users immediately lose
// all authorization.  If the user is re-enabled they must be re-assigned to
// groups to regain access.
func (a authzProjectingIdentityAdmin) DisableUser(ctx context.Context, orgID, userID string) error {
	if err := a.IdentityAdmin.DisableUser(ctx, orgID, userID); err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	filters := userSubjectDeleteFilters(userID)
	rels, _ := a.captureRelationships(ctx, filters)
	return a.projectDelete(ctx, "iam_api", "user", userID, filters, rels)
}

// DeleteOrganization removes the organization from the identity provider and
// then cleans up all SpiceDB relationships for the zone and every group that
// belonged to it.  Groups are read BEFORE the delete because the identity
// provider will no longer return them afterwards.
func (a authzProjectingIdentityAdmin) DeleteOrganization(ctx context.Context, req authn.DeleteOrganizationRequest) error {
	orgID := strings.TrimSpace(req.OrgID)
	// Read the org's groups before deleting so we can clean their SpiceDB
	// relationships afterwards.  Best-effort: if ListGroups fails we still
	// proceed with the zone-level cleanup.
	var groups []authn.Group
	if orgID != "" {
		groups, _ = a.IdentityAdmin.ListGroups(ctx, authn.GroupFilter{OrgID: orgID})
	}
	if err := a.IdentityAdmin.DeleteOrganization(ctx, req); err != nil {
		return err
	}
	if orgID == "" {
		return nil
	}
	// Delete the zone resource itself (all relations on zone:<orgID>) plus
	// every group resource that was nested under this zone.
	filters := []authz.RelationshipFilter{{ResourceType: "zone", ResourceID: orgID}}
	for _, g := range groups {
		gid := qualifiedGroupID(orgID, firstNonEmpty(g.ID, g.Name))
		if gid != "" {
			filters = append(filters, groupDeleteFilters(orgID, gid)...)
		}
	}
	rels, _ := a.captureRelationships(ctx, filters)
	return a.projectDelete(ctx, "iam_api", "zone", orgID, filters, rels)
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
	rels := make([]authz.Relationship, 0, len(groups)*3+len(users)*2+1)
	rels = append(rels, platformZoneRelationship(orgID))
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

func platformZoneRelationship(orgID string) authz.Relationship {
	return authz.Relationship{
		Resource: authz.ObjectRef{Type: "zone", ID: strings.TrimSpace(orgID)},
		Relation: "platform",
		Subject:  authz.SubjectRef{Type: "platform", ID: defaultPlatformID},
	}
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

// zoneMemberRelationship builds a zone#member@user relationship.  Used as
// compensation data when deleting a user's zone membership so the DTM saga
// can restore it if the downstream delete fails.
func zoneMemberRelationship(orgID, userID string) authz.Relationship {
	return authz.Relationship{Resource: authz.ObjectRef{Type: "zone", ID: strings.TrimSpace(orgID)}, Relation: "member", Subject: authz.SubjectRef{Type: "user", ID: strings.TrimSpace(userID)}}
}

// captureRelationships reads the SpiceDB relationships matched by the given
// filters so they can be stored as compensation data on a delete projection
// event.  Without this, a DTM saga rollback after a delete would re-write an
// empty relationship set and be unable to restore the deleted tuples.
//
// SpiceDB's ReadRelationships requires resource_type to be non-empty, so
// subject-only filters (resource_type == "") are skipped — their delete still
// proceeds, but those tuples cannot be compensated.  Capture is best-effort:
// if no reader is configured or a read errors, the delete continues with nil
// compensation rather than blocking the lifecycle operation.
func (a authzProjectingIdentityAdmin) captureRelationships(ctx context.Context, filters []authz.RelationshipFilter) ([]authz.Relationship, error) {
	if a.projection == nil || a.projection.reader == nil {
		return nil, nil
	}
	var captured []authz.Relationship
	for _, filter := range filters {
		if strings.TrimSpace(filter.ResourceType) == "" {
			continue
		}
		rels, err := a.projection.reader.ReadRelationships(ctx, filter)
		if err != nil {
			return captured, err
		}
		captured = append(captured, rels...)
	}
	return dedupeRelationships(captured), nil
}

// userSubjectDeleteFilters returns filters to delete all IAM-owned
// relationships where the given user is a subject.  SpiceDB's
// DeleteRelationships requires resource_type to be non-empty, so we
// enumerate the IAM-managed resource types rather than issuing a single
// subject-only filter.  This covers direct grants on every resource type
// whose schema permits a user subject (project#developer, skill#editor,
// git_repository#writer, role_binding#grantee, etc.) so a deleted/disabled
// user loses all authorization immediately rather than relying on group/zone
// membership to "naturally" fail to resolve.
func userSubjectDeleteFilters(userID string) []authz.RelationshipFilter {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return nil
	}
	return []authz.RelationshipFilter{
		{ResourceType: "platform", SubjectType: "user", SubjectID: uid},
		{ResourceType: "zone", SubjectType: "user", SubjectID: uid},
		{ResourceType: "group", SubjectType: "user", SubjectID: uid},
		{ResourceType: "iam_authz", SubjectType: "user", SubjectID: uid},
		{ResourceType: "iam", SubjectType: "user", SubjectID: uid},
		{ResourceType: "project", SubjectType: "user", SubjectID: uid},
		{ResourceType: "skill_space", SubjectType: "user", SubjectID: uid},
		{ResourceType: "skill", SubjectType: "user", SubjectID: uid},
		{ResourceType: "git_namespace", SubjectType: "user", SubjectID: uid},
		{ResourceType: "git_repository", SubjectType: "user", SubjectID: uid},
		{ResourceType: "agent_space", SubjectType: "user", SubjectID: uid},
		{ResourceType: "agent", SubjectType: "user", SubjectID: uid},
		{ResourceType: "tool_space", SubjectType: "user", SubjectID: uid},
		{ResourceType: "tool", SubjectType: "user", SubjectID: uid},
		{ResourceType: "sandbox_space", SubjectType: "user", SubjectID: uid},
		{ResourceType: "sandbox", SubjectType: "user", SubjectID: uid},
		{ResourceType: "runtime_environment", SubjectType: "user", SubjectID: uid},
		{ResourceType: "deployment", SubjectType: "user", SubjectID: uid},
		{ResourceType: "role_binding", SubjectType: "user", SubjectID: uid},
	}
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

// spicedbObjectIDAllowedRe matches characters that SpiceDB's default object_id
// regex (`^(([a-zA-Z0-9/_|\-=+]{1,})|\*)$`) permits. Any character outside this
// set is replaced with "_" so that free-form Casdoor group/user/org names
// (which may contain spaces, dots, colons, Chinese characters, emoji, etc.)
// never reach SpiceDB and cause InvalidArgument errors that retry forever in
// DTM sagas.
var spicedbObjectIDAllowedRe = regexp.MustCompile(`[^a-zA-Z0-9/_|\-=+]`)

// SanitizeObjectID normalizes a SpiceDB object_id so it matches the allowed
// character set. The wildcard "*" is returned as-is.
func SanitizeObjectID(id string) string {
	id = strings.TrimSpace(id)
	if id == "*" {
		return id
	}
	return spicedbObjectIDAllowedRe.ReplaceAllString(id, "_")
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
		rel.Resource.ID = SanitizeObjectID(rel.Resource.ID)
		rel.Subject.ID = SanitizeObjectID(rel.Subject.ID)
		rels = append(rels, rel)
	}
	payload.Relationships = dedupeRelationships(rels)
	filters := make([]authz.RelationshipFilter, 0, len(payload.Filters))
	for _, filter := range payload.Filters {
		if filter.ResourceType == "" && filter.ResourceID == "" && filter.Relation == "" && filter.SubjectType == "" && filter.SubjectID == "" && filter.SubjectRel == "" {
			continue
		}
		filter.ResourceID = SanitizeObjectID(filter.ResourceID)
		filter.SubjectID = SanitizeObjectID(filter.SubjectID)
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
