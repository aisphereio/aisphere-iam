// Package graphid provides utilities for generating SpiceDB object IDs that
// include organization scope as a prefix.
//
// Some resource types (group, project) use qualified IDs of the form
// "{org_id}/{local_id}" in SpiceDB to ensure uniqueness across organizations
// and to enable org-scoped authorization checks.
//
// With the stable-ID model, local IDs are already SpiceDB-safe (generated as
// "grp_" + 32 hex chars), so no sanitization is needed.
package graphid

import "strings"

// QualifiedID returns a SpiceDB object ID that includes the org scope as a
// prefix: "{org_id}/{local_id}".
//
// Example:
//
//	QualifiedID("aisphere", "grp_01AR...") // => "aisphere/grp_01AR..."
func QualifiedID(orgID, localID string) string {
	orgID = strings.Trim(strings.TrimSpace(orgID), "/")
	localID = strings.Trim(strings.TrimSpace(localID), "/")
	if orgID == "" {
		return localID
	}
	if localID == "" {
		return orgID
	}
	return orgID + "/" + localID
}