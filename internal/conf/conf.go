package conf

import (
	"time"

	"github.com/aisphereio/kernel/authn"
	"github.com/aisphereio/kernel/authn/casdoor"
	"github.com/aisphereio/kernel/authn/oidcx"
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
	Authn        AuthnConfig                      `json:"authn" yaml:"authn"`
	Authz        AuthzConfig                      `json:"authz" yaml:"authz"`
	InternalCall authn.InternalServiceTokenConfig `json:"internal_call" yaml:"internal_call"`
}

type AuthnConfig struct {
	Enabled      bool           `json:"enabled" yaml:"enabled"`
	Mode         string         `json:"mode" yaml:"mode"`
	IdentityMode string         `json:"identity_mode" yaml:"identity_mode"`
	Provider     string         `json:"provider" yaml:"provider"`
	OIDC         oidcx.Config   `json:"oidc" yaml:"oidc"`
	Casdoor      casdoor.Config `json:"casdoor" yaml:"casdoor"`
	CacheTTL     time.Duration  `json:"cache_ttl_ns" yaml:"cache_ttl_ns"`
}

type AuthzConfig struct {
	Enabled                   bool           `json:"enabled" yaml:"enabled"`
	Provider                  string         `json:"provider" yaml:"provider"`
	DevAllowAll               bool           `json:"dev_allow_all" yaml:"dev_allow_all"`
	InstallDefaultSchema      bool           `json:"install_default_schema" yaml:"install_default_schema"`
	AllowPermissionMigrations bool           `json:"allow_permission_migrations" yaml:"allow_permission_migrations"`
	SchemaPath                string         `json:"schema_path" yaml:"schema_path"`
	SpiceDB                   spicedb.Config `json:"spicedb" yaml:"spicedb"`
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
	Enabled                 bool                       `json:"enabled" yaml:"enabled"`
	CleanupLegacyExpansions bool                       `json:"cleanup_legacy_expansions" yaml:"cleanup_legacy_expansions"`
	Subjects                []ControlPlaneAdminSubject `json:"subjects" yaml:"subjects"`
}

type ControlPlaneAdminSubject struct {
	// Legacy direct subject form. Still supported for service accounts or exact user IDs.
	Type     string `json:"type" yaml:"type"`
	ID       string `json:"id" yaml:"id"`
	Relation string `json:"relation" yaml:"relation"`

	// Zone bootstrap form. Casdoor local admin can be declared as:
	// zone_id: aisphere, casdoor_org: aisphere, username: admin, role: zone_owner.
	ZoneID          string `json:"zone_id" yaml:"zone_id"`
	Role            string `json:"role" yaml:"role"`
	ExternalIssuer  string `json:"external_issuer" yaml:"external_issuer"`
	ExternalSubject string `json:"external_subject" yaml:"external_subject"`
	CasdoorOrg      string `json:"casdoor_org" yaml:"casdoor_org"`
	Username        string `json:"username" yaml:"username"`
	Source          string `json:"source" yaml:"source"`
	Reason          string `json:"reason" yaml:"reason"`
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
