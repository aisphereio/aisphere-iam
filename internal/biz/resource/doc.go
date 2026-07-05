// Package resource will implement resource type registration, resource
// projections, resource bindings and external resource bindings for the
// Aisphere resource control plane.
//
// Business services still own their domain tables and payloads. This package
// stores only the authorization/control-plane projection that IAM and SpiceDB
// need.
package resource
