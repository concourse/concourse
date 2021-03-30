package atccmd

import (
	"fmt"
	"net"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/conjur"
	"github.com/concourse/concourse/atc/creds/credhub"
	"github.com/concourse/concourse/atc/creds/dummy"
	"github.com/concourse/concourse/atc/creds/kubernetes"
	"github.com/concourse/concourse/atc/creds/secretsmanager"
	"github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/emitter"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/flag"
	"github.com/concourse/concourse/skymarshal/skycmd"
)

type TLSConfig struct {
	BindPort uint16    `yaml:"bind_port,omitempty"`
	Cert     flag.File `yaml:"cert,omitempty"`
	Key      flag.File `yaml:"key,omitempty"`
	CaCert   flag.File `yaml:"ca_cert,omitempty"`
}

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type LetsEncryptConfig struct {
	Enable  bool     `yaml:"enable,omitempty" env:"CONCOURSE_ENABLE_LETS_ENCRYPT,CONCOURSE_LETS_ENCRYPT_ENABLE"`
	ACMEURL flag.URL `yaml:"acme_url,omitempty"`
}

type DatabaseConfig struct {
	Postgres flag.PostgresConfig `yaml:"postgres,omitempty"`

	ConcurrentRequestLimits   map[string]int `yaml:"concurrent_request_limit,omitempty" validate:"dive,keys,limited_route,endkeys"`
	APIMaxOpenConnections     int            `yaml:"api_max_conns,omitempty"`
	BackendMaxOpenConnections int            `yaml:"backend_max_conns,omitempty"`

	EncryptionKey    flag.Cipher `yaml:"encryption_key,omitempty"`
	OldEncryptionKey flag.Cipher `yaml:"old_encryption_key,omitempty"`
}

type CredentialManagersConfig struct {
	Conjur         *conjur.Manager               `yaml:"conjur,omitempty"`
	CredHub        *credhub.CredHubManager       `yaml:"credhub,omitempty"`
	Dummy          *dummy.Manager                `yaml:"dummy_creds,omitempty"`
	Kubernetes     *kubernetes.KubernetesManager `yaml:"kubernetes,omitempty"`
	SecretsManager *secretsmanager.Manager       `yaml:"aws_secretsmanager,omitempty"`
	SSM            *ssm.SsmManager               `yaml:"aws_ssm,omitempty"`
	Vault          *vault.VaultManager           `yaml:"vault,omitempty"`
}

func (c CredentialManagersConfig) ConfiguredCredentialManager() (creds.Manager, error) {
	var configuredManagers []creds.Manager

	if c.Conjur != nil && c.Conjur.IsConfigured() {
		configuredManagers = append(configuredManagers, c.Conjur)
	}

	if c.CredHub != nil && c.CredHub.IsConfigured() {
		configuredManagers = append(configuredManagers, c.CredHub)
	}

	if c.Dummy != nil && c.Dummy.IsConfigured() {
		configuredManagers = append(configuredManagers, c.Dummy)
	}

	if c.Kubernetes != nil && c.Kubernetes.IsConfigured() {
		configuredManagers = append(configuredManagers, c.Kubernetes)
	}

	if c.SecretsManager != nil && c.SecretsManager.IsConfigured() {
		configuredManagers = append(configuredManagers, c.SecretsManager)
	}

	if c.SSM != nil && c.SSM.IsConfigured() {
		configuredManagers = append(configuredManagers, c.SSM)
	}

	if c.Vault != nil && c.Vault.IsConfigured() {
		configuredManagers = append(configuredManagers, c.Vault)
	}

	var configuredManager creds.Manager
	if len(configuredManagers) > 1 {
		return nil, fmt.Errorf("Multiple credential managers configured: %v", configuredManagers)
	}

	if configuredManagers != nil {
		configuredManager = configuredManagers[0]
	}

	return configuredManager, nil
}

type DebugConfig struct {
	BindIP   net.IP `yaml:"bind_ip,omitempty"`
	BindPort uint16 `yaml:"bind_port,omitempty"`
}

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type ResourceCheckingConfig struct {
	ScannerInterval            time.Duration `yaml:"scanner_interval,omitempty" env:"CONCOURSE_RESOURCE_CHECKING_SCANNER_INTERVAL,CONCOURSE_LIDAR_SCANNER_INTERVAL"`
	Timeout                    time.Duration `yaml:"timeout,omitempty" env:"CONCOURSE_RESOURCE_CHECKING_TIMEOUT,CONCOURSE_GLOBAL_RESOURCE_CHECK_TIMEOUT"`
	DefaultInterval            time.Duration `yaml:"default_interval,omitempty" env:"CONCOURSE_RESOURCE_CHECKING_DEFAULT_INTERVAL,CONCOURSE_RESOURCE_CHECKING_INTERVAL"`
	DefaultIntervalWithWebhook time.Duration `yaml:"default_interval_with_webhook,omitempty" env:"CONCOURSE_RESOURCE_CHECKING_DEFAULT_INTERVAL_WITH_WEBHOOK,CONCOURSE_RESOURCE_WITH_WEBHOOK_CHECKING_INTERVAL"`
	MaxChecksPerSecond         int           `yaml:"max_checks_per_second,omitempty" env:"CONCOURSE_RESOURCE_CHECKING_MAX_CHECKS_PER_SECOND,CONCOURSE_MAX_CHECKS_PER_SECOND"`
}

type JobSchedulingConfig struct {
	MaxInFlight uint64 `yaml:"max_in_flight,omitempty"`
}

type RuntimeConfig struct {
	ContainerPlacementStrategyOptions worker.ContainerPlacementStrategyOptions
	StreamingArtifactsCompression     string `yaml:"streaming_artifacts_compression,omitempty" validate:"sac"`

	BaggageclaimResponseHeaderTimeout time.Duration `yaml:"baggageclaim_response_header_timeout,omitempty"`
	P2pVolumeStreamingTimeout         time.Duration `yaml:"p2p_volume_streaming_timeout,omitempty"`
	GardenRequestTimeout              time.Duration `yaml:"garden_request_timeout,omitempty"`
}

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type MetricsConfig struct {
	HostName            string            `yaml:"host_name,omitempty"`
	Attributes          map[string]string `yaml:"attributes,omitempty"`
	BufferSize          uint32            `yaml:"buffer_size,omitempty"`
	CaptureErrorMetrics bool              `yaml:"capture_errors,omitempty" env:"CONCOURSE_METRICS_CAPTURE_ERRORS,CONCOURSE_CAPTURE_ERROR_METRICS"`

	Emitter MetricsEmitterConfig `yaml:"emitter,omitempty" ignore_env:"true"`
}

type MetricsEmitterConfig struct {
	Datadog    *emitter.DogstatsDBConfig `yaml:"datadog,omitempty"`
	InfluxDB   *emitter.InfluxDBConfig   `yaml:"influxdb,omitempty"`
	Lager      *emitter.LagerConfig      `yaml:"lager,omitempty"`
	NewRelic   *emitter.NewRelicConfig   `yaml:"newrelic,omitempty"`
	Prometheus *emitter.PrometheusConfig `yaml:"prometheus,omitempty"`
}

func (e MetricsEmitterConfig) ConfiguredEmitter() (metric.EmitterFactory, error) {
	var configuredEmitters []metric.EmitterFactory

	if e.Datadog != nil && e.Datadog.IsConfigured() {
		configuredEmitters = append(configuredEmitters, e.Datadog)
	}

	if e.InfluxDB != nil && e.InfluxDB.IsConfigured() {
		configuredEmitters = append(configuredEmitters, e.InfluxDB)
	}

	if e.Lager != nil && e.Lager.IsConfigured() {
		configuredEmitters = append(configuredEmitters, e.Lager)
	}

	if e.NewRelic != nil && e.NewRelic.IsConfigured() {
		configuredEmitters = append(configuredEmitters, e.NewRelic)
	}

	if e.Prometheus != nil && e.Prometheus.IsConfigured() {
		configuredEmitters = append(configuredEmitters, e.Prometheus)
	}

	var configuredEmitter metric.EmitterFactory
	if len(configuredEmitters) > 1 {
		return nil, fmt.Errorf("Multiple emitters configured: %v", configuredEmitters)
	}

	if configuredEmitters != nil {
		configuredEmitter = configuredEmitters[0]
	}

	return configuredEmitter, nil
}

type PolicyCheckersConfig struct {
	Filter policy.Filter `yaml:"filter,omitempty"`
}

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type ServerConfig struct {
	XFrameOptions string `yaml:"x_frame_options,omitempty" env:"CONCOURSE_WEB_SERVER_X_FRAME_OPTIONS,CONCOURSE_X_FRAME_OPTIONS"`
	ClusterName   string `yaml:"cluster_name,omitempty" env:"CONCOURSE_WEB_SERVER_CLUSTER_NAME,CONCOURSE_CLUSTER_NAME"`
	ClientID      string `yaml:"client_id,omitempty" env:"CONCOURSE_WEB_SERVER_CLIENT_ID,CONCOURSE_CLIENT_ID"`
	ClientSecret  string `yaml:"client_secret,omitempty" env:"CONCOURSE_WEB_SERVER_CLIENT_SECRET,CONCOURSE_CLIENT_SECRET"`
}

type LogConfig struct {
	DBQueries   bool `yaml:"db_queries,omitempty"`
	ClusterName bool `yaml:"cluster_name,omitempty"`
}

type GCConfig struct {
	Interval time.Duration `yaml:"interval,omitempty"`

	OneOffBuildGracePeriod time.Duration `yaml:"one_off_grace_period,omitempty"`
	MissingGracePeriod     time.Duration `yaml:"missing_grace_period,omitempty"`
	HijackGracePeriod      time.Duration `yaml:"hijack_grace_period,omitempty"`
	FailedGracePeriod      time.Duration `yaml:"failed_grace_period,omitempty"`
	CheckRecyclePeriod     time.Duration `yaml:"check_recycle_period,omitempty"`
	VarSourceRecyclePeriod time.Duration `yaml:"var_source_recycle_period,omitempty"`
}

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type BuildLogRetentionConfig struct {
	Default uint64 `yaml:"default,omitempty" env:"CONCOURSE_BUILD_LOG_RETENTION_DEFAULT,CONCOURSE_DEFAULT_BUILD_LOGS_TO_RETAIN"`
	Max     uint64 `yaml:"max,omitempty" env:"CONCOURSE_BUILD_LOG_RETENTION_MAX,CONCOURSE_MAX_BUILD_LOGS_TO_RETAIN"`

	DefaultDays uint64 `yaml:"default_days,omitempty" env:"CONCOURSE_BUILD_LOG_RETENTION_DEFAULT_DAYS,CONCOURSE_DEFAULT_DAYS_TO_RETAIN_BUILD_LOGS"`
	MaxDays     uint64 `yaml:"max_days,omitempty" env:"CONCOURSE_BUILD_LOG_RETENTION_MAX_DAYS,CONCOURSE_MAX_DAYS_TO_RETAIN_BUILD_LOGS"`
}

// IMPORTANT! The env tags are only added for backwards compatibility sake. Any
// new fields do NOT require the env tag
type AuditorConfig struct {
	EnableBuildAuditLog     bool `yaml:"enable_build,omitempty" env:"CONCOURSE_AUDITING_ENABLE_BUILD,CONCOURSE_ENABLE_BUILD_AUDITING"`
	EnableContainerAuditLog bool `yaml:"enable_container,omitempty" env:"CONCOURSE_AUDITING_ENABLE_CONTAINER,CONCOURSE_ENABLE_CONTAINER_AUDITING"`
	EnableJobAuditLog       bool `yaml:"enable_job,omitempty" env:"CONCOURSE_AUDITING_ENABLE_JOB,CONCOURSE_ENABLE_JOB_AUDITING"`
	EnablePipelineAuditLog  bool `yaml:"enable_pipeline,omitempty" env:"CONCOURSE_AUDITING_ENABLE_PIPELINE,CONCOURSE_ENABLE_PIPELINE_AUDITING"`
	EnableResourceAuditLog  bool `yaml:"enable_resource,omitempty" env:"CONCOURSE_AUDITING_ENABLE_RESOURCE,CONCOURSE_ENABLE_RESOURCE_AUDITING"`
	EnableSystemAuditLog    bool `yaml:"enable_system,omitempty" env:"CONCOURSE_AUDITING_ENABLE_SYSTEM,CONCOURSE_ENABLE_SYSTEM_AUDITING"`
	EnableTeamAuditLog      bool `yaml:"enable_team,omitempty" env:"CONCOURSE_AUDITING_ENABLE_TEAM,CONCOURSE_ENABLE_TEAM_AUDITING"`
	EnableWorkerAuditLog    bool `yaml:"enable_worker,omitempty" env:"CONCOURSE_AUDITING_ENABLE_WORKER,CONCOURSE_ENABLE_WORKER_AUDITING"`
	EnableVolumeAuditLog    bool `yaml:"enable_volume,omitempty" env:"CONCOURSE_AUDITING_ENABLE_VOLUME,CONCOURSE_ENABLE_VOLUME_AUDITING"`
}

type SyslogConfig struct {
	Hostname      string        `yaml:"hostname,omitempty"`
	Address       string        `yaml:"address,omitempty"`
	Transport     string        `yaml:"transport,omitempty"`
	DrainInterval time.Duration `yaml:"drain_interval,omitempty"`
	CACerts       []string      `yaml:"ca_cert,omitempty"`
}

type AuthConfig struct {
	AuthFlags     skycmd.AuthFlags
	MainTeamFlags skycmd.AuthTeamFlags `yaml:"main_team,omitempty"`
}

type SystemClaimConfig struct {
	Key    string   `yaml:"key,omitempty"`
	Values []string `yaml:"value,omitempty"`
}

type FeatureFlagsConfig struct {
	EnableGlobalResources                bool `yaml:"enable_global_resources,omitempty"`
	EnableRedactSecrets                  bool `yaml:"enable_redact_secrets,omitempty"`
	EnableBuildRerunWhenWorkerDisappears bool `yaml:"enable_rerun_when_worker_disappears,omitempty"`
	EnableAcrossStep                     bool `yaml:"enable_across_step,omitempty"`
	EnablePipelineInstances              bool `yaml:"enable_pipeline_instances,omitempty"`
	EnableP2PVolumeStreaming             bool `yaml:"enable_p2p_volume_streaming,omitempty"`
}
