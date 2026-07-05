// Package grant will implement product-level RBAC grants and project them to
// SpiceDB relationships.
//
// The durable Grant record in IAM is the management fact. SpiceDB is the
// permission-query projection and must be reconciled from grant state.
package grant
