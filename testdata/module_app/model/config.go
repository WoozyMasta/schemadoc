package model

import (
	"time"

	base "github.com/woozymasta/schemadoc-test-base/model"
)

// RetryPolicy controls retry behavior for transient failures.
type RetryPolicy struct {
	// Attempts is maximum number of retries per operation.
	//
	// **Guidelines**
	//
	//  - Use smaller values for request/response APIs.
	//  - Use larger values for async job processors.
	//  - Keep in mind total retry budget in upstream gateways.
	Attempts int `json:"attempts" jsonschema:"required,default=3,minimum=1,maximum=10,title=Retry Attempts,description=Maximum number of retry attempts for one operation." jsonschema_extras:"x-order=500"`
	// Backoff is delay between attempts in milliseconds.
	Backoff int `json:"backoff_ms,omitempty" jsonschema:"default=250,minimum=10,maximum=30000" jsonschema_extras:"x-order=4"`
	// MaxJitter is random jitter upper bound in milliseconds.
	MaxJitter int `json:"max_jitter_ms,omitempty" jsonschema:"default=100,minimum=0,maximum=10000" jsonschema_extras:"x-order=40"`
}

// QueueOptions describes options for one queue.
type QueueOptions struct {
	// Workers is number of worker goroutines.
	Workers int `json:"workers" jsonschema:"required,minimum=1,maximum=128,default=4" jsonschema_extras:"x-order=1"`
	// BatchSize is number of entries in one batch.
	BatchSize int `json:"batch_size,omitempty" jsonschema:"default=100,minimum=1,maximum=10000" jsonschema_extras:"x-order=80"`
	// VisibilityTimeout is message visibility timeout.
	VisibilityTimeout time.Duration `json:"visibility_timeout,omitempty" jsonschema:"default=30s" jsonschema_extras:"x-order=2"`
}

// StorageBucket describes one storage target.
type StorageBucket struct {
	// Name is bucket name.
	Name string `json:"name" jsonschema:"required,minLength=3,example=artifacts-main" jsonschema_extras:"x-order=70"`
	// Region is bucket region.
	Region string `json:"region,omitempty" jsonschema:"example=us-east-1" jsonschema_extras:"x-order=3"`
	// ReadOnly marks bucket as read-only.
	ReadOnly bool `json:"read_only,omitempty" jsonschema_extras:"x-order=71"`
}

// TLSOverride describes TLS override for one endpoint.
type TLSOverride struct {
	// Enabled overrides TLS state.
	Enabled *bool `json:"enabled,omitempty" jsonschema_extras:"x-order=9"`
	// MinVersion overrides minimal TLS version.
	MinVersion string `json:"min_version,omitempty" jsonschema:"enum=1.2,enum=1.3" jsonschema_extras:"x-order=1"`
}

// AlertTarget describes one alerting destination.
type AlertTarget struct {
	// Channel is target channel name.
	Channel string `json:"channel" jsonschema:"required,example=slack" jsonschema_extras:"x-order=2"`
	// Address is destination address.
	Address string `json:"address" jsonschema:"required,example=https://hooks.slack.com/..." jsonschema_extras:"x-order=200"`
	// Severity is optional minimum severity.
	Severity string `json:"severity,omitempty" jsonschema:"enum=info,enum=warn,enum=error" jsonschema_extras:"x-order=3"`
}

// Window describes daily maintenance time window.
type Window struct {
	// Start is start time (HH:MM).
	Start string `json:"start" jsonschema:"required,pattern=^[0-2][0-9]:[0-5][0-9]$" jsonschema_extras:"x-order=44"`
	// End is end time (HH:MM).
	End string `json:"end" jsonschema:"required,pattern=^[0-2][0-9]:[0-5][0-9]$" jsonschema_extras:"x-order=5"`
}

// AdvancedOptions groups additional optional controls.
type AdvancedOptions struct {
	// CircuitBreaker enables circuit breaker behavior.
	CircuitBreaker bool `json:"circuit_breaker,omitempty" jsonschema_extras:"x-order=100"`
	// Cooldown sets cooldown duration after errors.
	Cooldown time.Duration `json:"cooldown,omitempty" jsonschema:"default=45s" jsonschema_extras:"x-order=7"`
	// Tags adds free-form tags.
	Tags []string `json:"tags,omitempty" jsonschema_extras:"x-order=101"`
	// Metadata keeps arbitrary nested metadata.
	Metadata map[string]map[string]string `json:"metadata,omitempty" jsonschema_extras:"x-order=6"`
}

// DeepProbe is reused on every deep nesting level for path tests.
type DeepProbe struct {
	// Token is stable probe token value.
	Token string `json:"token" jsonschema:"required,default=probe" jsonschema_extras:"x-order=12"`
	// Enabled toggles probe branch.
	Enabled bool `json:"enabled,omitempty" jsonschema:"default=true" jsonschema_extras:"x-order=1"`
}

// Level6 is the deepest level in deep branch.
type Level6 struct {
	// Probe is repeated reusable type on this level.
	Probe DeepProbe `json:"probe" jsonschema:"required" jsonschema_extras:"x-order=20"`
	// FinalValue is terminal deep value.
	FinalValue string `json:"final_value,omitempty" jsonschema:"example=done" jsonschema_extras:"x-order=2"`
}

// Level5 contains one deeper nested level.
type Level5 struct {
	// Probe is repeated reusable type on this level.
	Probe DeepProbe `json:"probe" jsonschema:"required" jsonschema_extras:"x-order=77"`
	// Next points to next nested level.
	Next Level6 `json:"next" jsonschema:"required" jsonschema_extras:"x-order=3"`
}

// Level4 contains one deeper nested level.
type Level4 struct {
	// Probe is repeated reusable type on this level.
	Probe DeepProbe `json:"probe" jsonschema:"required" jsonschema_extras:"x-order=41"`
	// Next points to next nested level.
	Next Level5 `json:"next" jsonschema:"required" jsonschema_extras:"x-order=4"`
}

// Level3 contains one deeper nested level.
type Level3 struct {
	// Probe is repeated reusable type on this level.
	Probe DeepProbe `json:"probe" jsonschema:"required" jsonschema_extras:"x-order=42"`
	// Next points to next nested level.
	Next Level4 `json:"next" jsonschema:"required" jsonschema_extras:"x-order=1"`
}

// Level2 contains one deeper nested level.
type Level2 struct {
	// Probe is repeated reusable type on this level.
	Probe DeepProbe `json:"probe" jsonschema:"required" jsonschema_extras:"x-order=9"`
	// Next points to next nested level.
	Next Level3 `json:"next" jsonschema:"required" jsonschema_extras:"x-order=10"`
}

// Level1 contains one deeper nested level.
type Level1 struct {
	// Probe is repeated reusable type on this level.
	Probe DeepProbe `json:"probe" jsonschema:"required" jsonschema_extras:"x-order=2"`
	// Next points to next nested level.
	Next Level2 `json:"next" jsonschema:"required" jsonschema_extras:"x-order=200"`
}

// ServiceConfig is root schema used in golden docs tests.
type ServiceConfig struct {
	// Name is human-readable service name.
	//
	// **Naming conventions**
	//
	//  1. Include product and environment.
	//  1. Keep it stable across rollouts.
	//  1. Avoid random suffixes in long-lived services.
	//
	// Example: `payments-eu-prod`.
	Name string `json:"name" jsonschema:"required,minLength=3,default=demo-service,title=Service Name,description=Human-readable service name." jsonschema_extras:"x-order=10"`
	// Mode is execution mode.
	Mode string `json:"mode,omitempty" jsonschema:"default=safe,enum=safe,enum=fast" jsonschema_extras:"x-order=400"`
	// Retry is transient retry strategy.
	Retry RetryPolicy `json:"retry" jsonschema:"required,title=Retry Policy" jsonschema_extras:"x-order=11"`
	// Shared is shared options imported from base module.
	Shared base.SharedOptions `json:"shared" jsonschema:"required,title=Shared Options" jsonschema_extras:"x-order=1"`
	// Queues configures processing queues by queue name.
	Queues map[string]QueueOptions `json:"queues,omitempty" jsonschema_extras:"x-order=150"`
	// NamedBuckets configures storage by logical bucket key.
	NamedBuckets map[string]StorageBucket `json:"named_buckets,omitempty" jsonschema_extras:"x-order=151"`
	// BucketsByPriority maps numeric priority to bucket config.
	BucketsByPriority map[int]StorageBucket `json:"buckets_by_priority,omitempty" jsonschema_extras:"x-order=152"`
	// BucketGroups stores grouped bucket lists.
	BucketGroups map[string][]StorageBucket `json:"bucket_groups,omitempty" jsonschema_extras:"x-order=153"`
	// QueueWorkersByZone stores workers count by zone.
	QueueWorkersByZone map[string]map[string]int `json:"queue_workers_by_zone,omitempty" jsonschema_extras:"x-order=154"`
	// AlertTargets stores alerting destinations by level.
	AlertTargets map[string][]AlertTarget `json:"alert_targets,omitempty" jsonschema_extras:"x-order=155"`
	// EndpointTLSOverrides overrides TLS by endpoint name.
	EndpointTLSOverrides map[string]TLSOverride `json:"endpoint_tls_overrides,omitempty" jsonschema_extras:"x-order=14"`
	// Schedule is fixed-size weekly maintenance windows.
	Schedule [7]Window `json:"schedule,omitempty" jsonschema_extras:"x-order=13"`
	// MirrorEndpoints is optional explicit mirror list.
	MirrorEndpoints *[]string `json:"mirror_endpoints,omitempty" jsonschema_extras:"x-order=900"`
	// OptionalRetry allows temporary retry override.
	OptionalRetry *RetryPolicy `json:"optional_retry,omitempty" jsonschema_extras:"x-order=15"`
	// BaseToggle references tiny base reusable toggle structure.
	BaseToggle base.SharedToggle `json:"base_toggle,omitempty" jsonschema_extras:"x-order=16"`
	// BaseWindow references nested base timing structure.
	BaseWindow base.SharedWindow `json:"base_window,omitempty" jsonschema_extras:"x-order=1200"`
	// BaseBinding references base structure that nests existing EndpointMeta.
	BaseBinding base.SharedEndpointBinding `json:"base_binding,omitempty" jsonschema_extras:"x-order=17"`
	// DirectProbe is root-level direct reusable probe.
	DirectProbe DeepProbe `json:"direct_probe,omitempty" jsonschema_extras:"x-order=18"`
	// DeepPrimary is first deep branch with six levels.
	DeepPrimary Level1 `json:"deep_primary,omitempty" jsonschema_extras:"x-order=19"`
	// DeepSecondary is second deep branch using same deep chain.
	DeepSecondary Level1 `json:"deep_secondary,omitempty" jsonschema_extras:"x-order=20"`
	// DeepByEnv stores deep branches by environment key.
	DeepByEnv map[string]Level1 `json:"deep_by_env,omitempty" jsonschema_extras:"x-order=21"`
	// Advanced groups optional advanced behavior flags.
	Advanced AdvancedOptions `json:"advanced,omitempty" jsonschema_extras:"x-order=22"`
	// Extensions stores external integration raw values.
	//
	// This section is intentionally flexible and can be used by external
	// delivery teams. The core service should not rely on these fields for
	// baseline startup decisions.
	//
	// **Allowed usage patterns**
	//
	//   - Feature toggles owned by integration team.
	//   - Provider-specific adapter hints.
	//   - Temporary migration markers.
	//
	// **Do not store**
	//
	//   - Secrets and tokens.
	//   - Large binary payloads.
	//   - Contract-breaking values that redefine core behavior.
	//
	// > Keep values small, explicit, and disposable.
	//
	// Example:
	//
	//   {
	//     "x-provider": "acme-cloud",
	//     "x-migration-stage": "phase-2",
	//     "x-flags": ["shadow-write", "dry-run"]
	//   }
	Extensions map[string]any `json:"extensions,omitempty" jsonschema_extras:"x-order=1000"`
}
