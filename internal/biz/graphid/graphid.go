// Package graphid provides utilities for generating SpiceDB object IDs that
// include organization scope as a prefix.
//
// Some resource types (group, project) use qualified IDs of the form
// "{org_id}/{local_id}" in SpiceDB to ensure uniqueness across organizations
// and to enable org-scoped authorization checks.
package graphid

import (
	"strings"

	"github.com/aisphereio/aisphere-iam/internal/data"
)

// QualifiedID returns a SpiceDB object ID that includes the org scope as a
// prefix: "{org_id}/{local_id}". Both parts are sanitized for SpiceDB
// compatibility.
//
// Example:
//
//	QualifiedID("aisphere", "platform") // => "aisphere/platform"
func QualifiedID(orgID, localID string) string {
	orgID = strings.Trim(strings.TrimSpace(orgID), "/")
	localID = strings.Trim(strings.TrimSpace(localID), "/")
	if orgID == "" {
		return data.SanitizeObjectID(localID)
	}
	if localID == "" {
		return data.SanitizeObjectID(orgID)
	}
	return data.SanitizeObjectID(orgID) + "/" + data.SanitizeObjectID(localID)
}