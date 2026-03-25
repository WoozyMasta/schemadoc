package model

import "time"

// Port is a typed TCP/UDP port.
type Port uint16

// LogLevel is a typed service log level.
type LogLevel string

const (
	// LogLevelInfo enables informational logs.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn enables warning logs.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError enables error logs.
	LogLevelError LogLevel = "error"
)

// EndpointMeta describes metadata for one endpoint target.
type EndpointMeta struct {
	// Region is deployment region code.
	Region string `json:"region,omitempty" jsonschema:"example=eu-central-1" jsonschema_extras:"x-order=91"`
	// Weight is load balancing weight.
	Weight int `json:"weight,omitempty" jsonschema:"minimum=0,maximum=100" jsonschema_extras:"x-order=7"`
	// ReadOnly marks endpoint as read-only.
	ReadOnly bool `json:"read_only,omitempty" jsonschema_extras:"x-order=250"`
}

// SharedTLS describes shared transport security settings.
type SharedTLS struct {
	// Enabled enables TLS for outbound connections.
	Enabled bool `json:"enabled" jsonschema:"default=true,title=TLS Enabled,description=Enable TLS for outbound connections." jsonschema_extras:"x-order=12"`
	// MinVersion is lowest allowed TLS protocol version.
	MinVersion string `json:"min_version" jsonschema:"default=1.2,enum=1.2,enum=1.3" jsonschema_extras:"x-order=88"`
	// CipherSuites lists explicitly allowed cipher suites.
	CipherSuites []string `json:"cipher_suites,omitempty" jsonschema:"example=TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" jsonschema_extras:"x-order=13"`
	// HandshakeTimeout sets TLS handshake timeout.
	HandshakeTimeout time.Duration `json:"handshake_timeout,omitempty" jsonschema:"default=5s" jsonschema_extras:"x-order=377"`
}

// SharedEndpoints describes common endpoint set used by services.
type SharedEndpoints struct {
	// API is primary API base URL.
	API string `json:"api" jsonschema:"required,format=uri,example=https://api.acme.local,title=API Endpoint" jsonschema_extras:"x-order=5"`
	// Metrics is optional metrics endpoint URL.
	Metrics string `json:"metrics,omitempty" jsonschema:"format=uri,example=https://metrics.acme.local" jsonschema_extras:"x-order=111"`
	// Internal is internal service endpoint URL.
	Internal string `json:"internal,omitempty" jsonschema:"format=uri,example=http://internal:8080" jsonschema_extras:"x-order=6"`
	// Meta stores endpoint metadata by endpoint name.
	Meta map[string]EndpointMeta `json:"meta,omitempty" jsonschema_extras:"x-order=204"`
}

// SharedToggle is a tiny reusable on/off configuration block.
type SharedToggle struct {
	// Enabled toggles one optional feature branch.
	Enabled bool `json:"enabled,omitempty" jsonschema:"default=true" jsonschema_extras:"x-order=42"`
}

// SharedWindow is a nested window with retry-aware timing.
type SharedWindow struct {
	// Start is start minute in a one-hour window.
	Start int `json:"start" jsonschema:"required,minimum=0,maximum=59" jsonschema_extras:"x-order=2"`
	// End is end minute in a one-hour window.
	End int `json:"end" jsonschema:"required,minimum=0,maximum=59" jsonschema_extras:"x-order=999"`
	// Retry uses existing base retry-like duration style.
	RetryDelay time.Duration `json:"retry_delay,omitempty" jsonschema:"default=500ms" jsonschema_extras:"x-order=17"`
}

// SharedEndpointBinding nests existing endpoint metadata into a binding.
type SharedEndpointBinding struct {
	// Name is logical binding name.
	Name string `json:"name" jsonschema:"required,example=primary" jsonschema_extras:"x-order=14"`
	// Meta reuses existing endpoint metadata schema.
	Meta EndpointMeta `json:"meta" jsonschema:"required" jsonschema_extras:"x-order=3"`
}

// SharedOptions is reusable part expected to be merged into app schema.
type SharedOptions struct {
	// TLS is TLS configuration shared across components.
	TLS SharedTLS `json:"tls" jsonschema:"required,title=TLS Options" jsonschema_extras:"x-order=10"`
	// Endpoints are network endpoints shared across components.
	Endpoints SharedEndpoints `json:"endpoints" jsonschema:"required,title=Endpoint Options" jsonschema_extras:"x-order=1"`
	// Ports maps external service names to exposed port.
	Ports map[string]Port `json:"ports,omitempty" jsonschema_extras:"x-order=300"`
	// Labels maps short numeric label IDs to values.
	Labels map[int]string `json:"labels,omitempty" jsonschema_extras:"x-order=21"`
	// LevelBySubsystem maps subsystem name to log level.
	LevelBySubsystem map[string]LogLevel `json:"level_by_subsystem,omitempty" jsonschema_extras:"x-order=4"`
	// TimeoutByEndpoint stores timeout per endpoint name.
	TimeoutByEndpoint map[string]time.Duration `json:"timeout_by_endpoint,omitempty" jsonschema_extras:"x-order=700"`
	// FeatureMatrix stores feature flags as nested map.
	//
	// **Matrix semantics**
	//
	//   - First key: feature group name.
	//   - Second key: feature flag name.
	//   - Value: desired on/off state.
	//
	// Example:
	//
	//   - `auth.mfa_required = true`
	//   - `billing.v2_invoice = false`
	FeatureMatrix map[string]map[string]bool `json:"feature_matrix,omitempty" jsonschema_extras:"x-order=41"`
	// EndpointGroups stores endpoint names grouped by key.
	EndpointGroups map[string][]string `json:"endpoint_groups,omitempty" jsonschema_extras:"x-order=8"`
	// EndpointPorts stores ports by endpoint and protocol name.
	//
	// This field is useful when one logical endpoint exposes multiple listener
	// protocols (for example `http`, `grpc`, `admin`). Consumers can resolve
	// exact port by `(endpoint, protocol)` pair without hardcoding transport
	// assumptions.
	EndpointPorts map[string]map[string]Port `json:"endpoint_ports,omitempty" jsonschema_extras:"x-order=305"`
	// FeatureToggle is a small reusable boolean options block.
	FeatureToggle SharedToggle `json:"feature_toggle,omitempty" jsonschema_extras:"x-order=66"`
	// MaintenanceWindow stores shared maintenance timing.
	MaintenanceWindow SharedWindow `json:"maintenance_window,omitempty" jsonschema_extras:"x-order=67"`
	// EndpointBinding links endpoint identity and metadata.
	EndpointBinding SharedEndpointBinding `json:"endpoint_binding,omitempty" jsonschema_extras:"x-order=65"`
}
