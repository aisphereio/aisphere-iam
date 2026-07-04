package conf

import (
	"time"

	"github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/authz/spicedb"
	"github.com/aisphereio/kernel/cachex"
	"github.com/aisphereio/kernel/dbx"
	"github.com/aisphereio/kernel/dtmx"
	"github.com/aisphereio/kernel/logx"
	"github.com/aisphereio/kernel/migrationx"
	"github.com/aisphereio/kernel/objectstorex"
	khttp "github.com/aisphereio/kernel/transportx/http"
)

type Bootstrap struct {
	Service      ServiceConfig      `json:"service" yaml:"service"`
	Server       ServerConfig       `json:"server" yaml:"server"`
	Log          logx.Config        `json:"log" yaml:"log"`
	Data         DataConfig         `json:"data" yaml:"data"`
	Security     SecurityConfig     `json:"security" yaml:"security"`
	ControlPlane ControlPlaneConfig `json:"control_plane" yaml:"control_plane"`
	Gateway      GatewayConfig      `json:"gateway" yaml:"gateway"`
	Audit        AuditConfig        `json:"audit" yaml:"audit"`
	Metrics      MetricsConfig      `json:"metrics" yaml:"metrics"`
	DTM          dtmx.Config        `json:"dtm" yaml:"dtm"`
}

type ServiceConfig struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Env     string `json:"env" yaml:"env"`
}

type ServerConfig struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
	GRPC GRPCConfig `json:"grpc" yaml:"grpc"`
}

type HTTPConfig struct {
	Addr    string           `json:"addr" yaml:"addr"`
	Timeout time.Duration    `json:"timeout_ns" yaml:"timeout_ns"`
	CORS    khttp.CORSConfig `json:"cors" yaml:"cors"`
}

type GRPCConfig struct {
	Addr    string        `json:"addr" yaml:"addr"`
	Timeout time.Duration `json:"timeout_ns" yaml:"timeout_ns"`
}

type DataConfig struct {
	Database    DatabaseConfig    `json:"database" yaml:"database"`
	Migration   MigrationConfig   `json:"migration" yaml:"migration"`
	Cache       CacheConfig       `json:"cache" yaml:"cache"`
	ObjectStore ObjectStoreConfig `json:"object_store" yaml:"object_store"`
}

type DatabaseConfig struct {
	Enabled bool       `json:"enabled" yaml:"enabled"`
	Config  dbx.Config `json:"config" yaml:"config"`
}

type MigrationConfig struct {
	Enabled bool              `json:"enabled" yaml:"enabled"`
	Config  migrationx.Config `json:"config" yaml:"config"`
}

type CacheConfig struct {
	Enabled bool          `json:"enabled" yaml:"enabled"`
	Config  cachex.Config `json:"config" yaml:"config"`
}

type ObjectStoreConfig struct {
	Enabled bool                `json:"enabled" yaml:"enabled"`
	Config  objectstorex.Config `json:"config" yaml:"config"`
}

type SecurityConfig struct {
	Authn  AuthnConfig  `json:"authn" yaml:"authn"`
	Authz  AuthzConfig  `json:"authz" yaml:"authz"`
	Access AccessConfig `json:"access" yaml:"access"`
}

// AccessConfig controls per-operation access policies. Each entry maps an
// operation pattern to its access mode. This allows operators to override
// the default authorization behavior without code changes.
type AccessConfig struct {
	// SkipOperations lists operations that should skip the SpiceDB
	// authorization check but still require authentication and record audit.
	// This is the recommended replacement for the deprecated AllowAllOperations.
	SkipOperations []string `json:"skip_operations" yaml:"skip_operations"`

	// PublicOperations lists operations that skip both authentication AND
	// authorization. Use for endpoints that must be accessible without any
	// credentials.
	PublicOperations []string `json:"public_operations" yaml:"public_operations"`

	// AllowAllOperations lists operations where any authenticated user is
	// allowed. This is the legacy field — new deployments should use
	// SkipOperations instead.
	//
	// Deprecated: Use SkipOperations instead.
	AllowAllOperations []string `json:"allow_all_operations" yaml:"allow_all_operations"`
}

type AuthnConfig struct {
	Enabled  bool           `json:"enabled" yaml:"enabled"`
	Provider string         `json:"provider" yaml:"provider"`
	Casdoor  casdoor.Config `json:"casdoor" yaml:"casdoor"`
}

type AuthzConfig struct {
	Enabled              bool           `json:"enabled" yaml:"enabled"`
	Provider             string         `json:"provider" yaml:"provider"`
	DevAllowAll          bool           `json:"dev_allow_all" yaml:"dev_allow_all"`
	InstallDefaultSchema bool           `json:"install_default_schema" yaml:"install_default_schema"`
	SchemaPath           string         `json:"schema_path" yaml:"schema_path"`
	SpiceDB              spicedb.Config `json:"spicedb" yaml:"spicedb"`
}

type ControlPlaneConfig struct {
	Defaults        ControlPlaneDefaultsConfig        `json:"defaults" yaml:"defaults"`
	BootstrapAdmins ControlPlaneBootstrapAdminsConfig `json:"bootstrap_admins" yaml:"bootstrap_admins"`
}

type ControlPlaneDefaultsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Path    string `json:"path" yaml:"path"`
}

type ControlPlaneBootstrapAdminsConfig struct {
	Enabled   bool                        `json:"enabled" yaml:"enabled"`
	Subjects  []ControlPlaneAdminSubject  `json:"subjects" yaml:"subjects"`
	Resources []ControlPlaneAdminResource `json:"resources" yaml:"resources"`
}

type ControlPlaneAdminSubject struct {
	Type     string `json:"type" yaml:"type"`
	ID       string `json:"id" yaml:"id"`
	Relation string `json:"relation" yaml:"relation"`
}

type ControlPlaneAdminResource struct {
	Type string `json:"type" yaml:"type"`
	ID   string `json:"id" yaml:"id"`
}

type GatewayConfig struct {
	RouteRegistry RouteRegistryConfig `json:"route_registry" yaml:"route_registry"`
}

type RouteRegistryConfig struct {
	Provider       string        `json:"provider" yaml:"provider"`
	Prefix         string        `json:"prefix" yaml:"prefix"`
	Endpoints      []string      `json:"endpoints" yaml:"endpoints"`
	DialTimeout    time.Duration `json:"dial_timeout_ns" yaml:"dial_timeout_ns"`
	RequestTimeout time.Duration `json:"request_timeout_ns" yaml:"request_timeout_ns"`
}

type AuditConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Store   string `json:"store" yaml:"store"`
}

type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Addr    string `json:"addr" yaml:"addr"`
	Path    string `json:"path" yaml:"path"`
	Pprof   bool   `json:"pprof" yaml:"pprof"`
	Runtime bool   `json:"runtime" yaml:"runtime"`
}
