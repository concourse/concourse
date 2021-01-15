package atccmd

import (
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitializeEnvDEPRECATED(c *cobra.Command) {
	viper.SetEnvPrefix("concourse")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func InitializeFlagsDEPRECATED(c *cobra.Command, flags *RunCommand) {
	c.Flags().IPVar(&flags.BindIP, "bind-ip", nil, "IP address on which to listen for web traffic.")
	c.Flags().Uint16Var(&flags.BindPort, "bind-port", 8080, "Port on which to listen for HTTP traffic.")

	InitializeTLSFlags(c, flags)

	InitializeLetsEncryptFlags(c, flags)

	c.Flags().StringVar(&flags.ExternalURL, "external-url", "", "URL used to reach any ATC from the outside world.")

	InitializeDatabaseFlags(c, flags)

	InitializeSecretRetryFlags(c, flags)
	InitializeCachedSecretsFlags(c, flags)

	InitializeManagerFlags(c, flags)

	InitializeDebugFlags(c, flags)

	c.Flags().DurationVar(&flags.InterceptIdleTimeout, "intercept-idle-timeout", 0*time.Minute, "Length of time for a intercepted session to be idle before terminating.")

	c.Flags().BoolVar(&flags.EnableGlobalResources, "enable-global-resources", false, "Enable equivalent resources across pipelines and teams to share a single version history.")

	c.Flags().DurationVar(&flags.ComponentRunnerInterval, "component-runner-interval", 10*time.Second, "Interval on which runners are kicked off for builds, locks, scans, and checks")
	c.Flags().DurationVar(&flags.BuildTrackerInterval, "build-tracker-interval", 10*time.Second, "Interval on which to run build tracking.")

	InitializeResourceCheckingFlags(c, flags)

	InitializeJobSchedulingFlags(c, flags)

	InitializeRuntimeFlags(c, flags)

	c.Flags().StringVar(&flags.CLIArtifactsDir, "cli-artifacts-dir", "", "Directory containing downloadable CLI binaries.")

	InitializeMetricsFlags(c, flags)

	InitializeTracingFlags(c, flags)

	InitializePolicyFlags(c, flags)

	InitializeServerFlags(c, flags)

	InitializeLogFlags(c, flags)

	InitializeGCFlags(c, flags)

	c.Flags().BoolVar(&flags.TelemetryOptIn, "telemetry-opt-in", false, "Enable anonymous concourse version reporting.")
	c.Flags().MarkHidden("telemetry-opt-in")

	InitializeBuildLogRetentionFlags(c, flags)

	c.Flags().IntVar(flags.DefaultCpuLimit, "default-task-cpu-limit", 0, "Default max number of cpu shares per task, 0 means unlimited")
	c.Flags().StringVar(flags.DefaultMemoryLimit, "default-task-memory-limit", "", "Default maximum memory per task, 0 means unlimited")

	InitializeAuditorFlags(c, flags)

	InitializeSyslogFlags(c, flags)

	InitializeAuthFlags(c, flags)

	c.Flags().BoolVar(&flags.EnableRedactSecrets, "enable-redact-secrets", false, "Enable redacting secrets in build logs.")

	c.Flags().StringVar(&flags.ConfigRBAC, "config-rbac", "", "Customize RBAC role-action mapping.")

	InitializeSystemClaimFlags(c, flags)

	InitializeExperimentalFlags(c, flags)
}

func InitializeTLSFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().Uint16Var(&flags.TLS.BindPort, "tls-bind-port", 0, "Port on which to listen for HTTPS traffic.")
	c.Flags().StringVar(&flags.TLS.Cert, "tls-cert", "", "File containing an SSL certificate.")
	c.Flags().StringVar(&flags.TLS.Key, "tls-key", "", "File containing an RSA private key, used to encrypt HTTPS traffic.")
}

func InitializeLetsEncryptFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().BoolVar(&flags.LetsEncrypt.Enable, "enable-lets-encrypt", false, "Automatically configure TLS certificates via Let's Encrypt/ACME.")
	c.Flags().StringVar(&flags.LetsEncrypt.ACMEURL, "lets-encrypt-acme-url", "https://acme-v02.api.letsencrypt.org/directory", "URL of the ACME CA directory endpoint.")
}

func InitializePostgresFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringVar(&flags.Database.Postgres.Host, "postgres-host", "127.0.0.1", "The host to connect to.")
	c.Flags().Uint16Var(&flags.Database.Postgres.Port, "postgres-port", 5432, "The port to connect to.")
	c.Flags().StringVar(&flags.Database.Postgres.Socket, "postgres-socket", "", "Path to a UNIX domain socket to connect to.")
	c.Flags().StringVar(&flags.Database.Postgres.User, "postgres-user", "", "The user to sign in as.")
	c.Flags().StringVar(&flags.Database.Postgres.Password, "postgres-password", "", "The user's password.")
	c.Flags().StringVar(&flags.Database.Postgres.SSLMode, "postgres-sslmode", "disable", "Whether or not to use SSL.")
	c.Flags().StringVar(&flags.Database.Postgres.CACert, "postgres-ca-cert", "", "CA cert file location, to verify when connecting with SSL.")
	c.Flags().StringVar(&flags.Database.Postgres.ClientCert, "postgres-client-cert", "", "Client cert file location.")
	c.Flags().StringVar(&flags.Database.Postgres.ClientKey, "postgres-client-key", "", "Client key file location.")
	c.Flags().DurationVar(&flags.Database.Postgres.ConnectTimeout, "postgres-connect-timeout", 5*time.Minute, "Dialing timeout. (0 means wait indefinitely)")
	c.Flags().StringVar(&flags.Database.Postgres.Database, "postgres-database", "atc", "The name of the database to use.")
}

func InitializeDatabaseFlags(c *cobra.Command, flags *RunCommand) {
	InitializePostgresFlags(c, flags)

	c.Flags().StringToIntVar(&flags.Database.ConcurrentRequestLimits, "concurrent-request-limit", nil, "Limit the number of concurrent requests to an API endpoint (Example: ListAllJobs:5)")

	c.Flags().IntVar(&flags.Database.APIMaxOpenConnections, "api-max-conns", 10, "The maximum number of open connections for the api connection pool.")
	c.Flags().IntVar(&flags.Database.BackendMaxOpenConnections, "backend-max-conns", 50, "The maximum number of open connections for the backend connection pool.")

	c.Flags().StringVar(&flags.Database.EncryptionKey, "encryption-key", "", "A 16 or 32 length key used to encrypt sensitive information before storing it in the database.")
	c.Flags().StringVar(&flags.Database.OldEncryptionKey, "old-encryption-key", "", "Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted.")
}

func InitializeDebugFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().IPVar(&flags.Debug.BindIP, "debug-bind-ip", net.IPv4(127, 0, 0, 1), "IP address on which to listen for the pprof debugger endpoints.")
	c.Flags().Uint16Var(&flags.Debug.BindPort, "debug-bind-port", 8079, "Port on which to listen for the pprof debugger endpoints.")
}

func InitializeResourceCheckingFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().DurationVar(&flags.ResourceChecking.ScannerInterval, "lidar-scanner-interval", 10*time.Second, "Interval on which the resource scanner will run to see if new checks need to be scheduled")
	c.Flags().DurationVar(&flags.ResourceChecking.CheckerInterval, "lidar-checker-interval", 10*time.Second, "Interval on which the resource checker runs any scheduled checks")

	c.Flags().DurationVar(&flags.ResourceChecking.Timeout, "global-resource-check-timeout", 1*time.Hour, "Time limit on checking for new versions of resources.")
	c.Flags().DurationVar(&flags.ResourceChecking.DefaultInterval, "resource-checking-interval", 1*time.Minute, "Interval on which to check for new versions of resources.")
	c.Flags().DurationVar(&flags.ResourceChecking.DefaultIntervalWithWebhook, "resource-with-webhook-checking-interval", 1*time.Minute, "Interval on which to check for new versions of resources that has webhook defined.")
	c.Flags().IntVar(&flags.ResourceChecking.MaxChecksPerSecond, "max-checks-per-second", 0, "Maximum number of checks that can be started per second. If not specified, this will be calculated as (# of resources)/(resource checking interval). -1 value will remove this maximum limit of checks per second.")
}

func InitializeJobSchedulingFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().Uint64Var(&flags.JobScheduling.MaxInFlight, "job-scheduling-max-in-flight", 32, "Maximum number of jobs to be scheduling at the same time")
}

func InitializeRuntimeFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringVar(&flags.Runtime.ContainerPlacementStrategy, "container-placement-strategy", "volume-locality", "Method by which a worker is selected during container placement.")
	c.Flags().IntVar(&flags.Runtime.MaxActiveTasksPerWorker, "max-active-tasks-per-worker", 0, "Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit.")
	c.Flags().DurationVar(&flags.Runtime.BaggageclaimResponseHeaderTimeout, "baggageclaim-response-header-timeout", 1*time.Minute, "How long to wait for Baggageclaim to send the response header.")
	c.Flags().StringVar(&flags.Runtime.StreamingArtifactsCompression, "streaming-artifacts-compression", "gzip", "Compression algorithm for internal streaming.")
	c.Flags().DurationVar(&flags.Runtime.GardenRequestTimeout, "garden-request-timeout", 5*time.Minute, "How long to wait for requests to Garden to complete. 0 means no timeout.")
}

func InitializeMetricsFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringVar(&flags.Metrics.HostName, "metrics-host-name", "", "Host string to attach to emitted metrics.")
	c.Flags().StringToStringVar(&flags.Metrics.Attributes, "metrics-attribute", nil, "A key-value attribute to attach to emitted metrics. Can be specified multiple times. Ex: NAME:VALUE")
	c.Flags().Uint32Var(&flags.Metrics.BufferSize, "metrics-buffer-size", 1000, "The size of the buffer used in emitting event metrics.")
	c.Flags().BoolVar(&flags.Metrics.CaptureErrorMetrics, "capture-error-metrics", false, "Enable capturing of error log metrics")
}

func InitializeSecretRetryFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().IntVar(&flags.CredentialManagement.RetryConfig.Attempts, "secret-retry-attempts", 5, "The number of attempts secret will be retried to be fetched, in case a retryable error happens.")
	c.Flags().DurationVar(&flags.CredentialManagement.RetryConfig.Interval, "secret-retry-interval", 1*time.Second, "The interval between secret retry retrieval attempts.")
}

func InitializeCachedSecretsFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().BoolVar(&flags.CredentialManagement.CacheConfig.Enabled, "secret-cache-enabled", false, "Enable in-memory cache for secrets")
	c.Flags().DurationVar(&flags.CredentialManagement.CacheConfig.Duration, "secret-cache-duration", 1*time.Minute, "If the cache is enabled, secret values will be cached for not longer than this duration (it can be less, if underlying secret lease time is smaller)")
	c.Flags().DurationVar(&flags.CredentialManagement.CacheConfig.DurationNotFound, "secret-cache-duration-notfound", 10*time.Second, "If the cache is enabled, secret not found responses will be cached for this duration")
	c.Flags().DurationVar(&flags.CredentialManagement.CacheConfig.PurgeInterval, "secret-cache-purge-interval", 10*time.Minute, "If the cache is enabled, expired items will be removed on this interval")
}

func InitializeManagerFlags(c *cobra.Command, flags *RunCommand) {
	// Conjur
	type Manager struct {
		ConjurApplianceUrl     string `long:"conjur-appliance-url" description:"URL of the conjur instance"`
		ConjurAccount          string `long:"conjur-account" description:"Conjur Account"`
		ConjurCertFile         string `long:"conjur-cert-file" description:"Cert file used if conjur instance is using a self signed cert. E.g. /path/to/conjur.pem"`
		ConjurAuthnLogin       string `long:"conjur-authn-login" description:"Host username. E.g host/concourse"`
		ConjurAuthnApiKey      string `long:"conjur-authn-api-key" description:"Api key related to the host"`
		ConjurAuthnTokenFile   string `long:"conjur-authn-token-file" description:"Token file used if conjur instance is running in k8s or iam. E.g. /path/to/token_file"`
		PipelineSecretTemplate string `long:"conjur-pipeline-secret-template" description:"Conjur secret identifier template used for pipeline specific parameter" default:"concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"`
		TeamSecretTemplate     string `long:"conjur-team-secret-template" description:"Conjur secret identifier template used for team specific parameter" default:"concourse/{{.Team}}/{{.Secret}}"`
		SecretTemplate         string `long:"conjur-secret-template" description:"Conjur secret identifier template used for full path conjur secrets" default:"vaultName/{{.Secret}}"`
		Conjur                 *Conjur
	}
}

func InitializeTracingFlags(c *cobra.Command, flags *RunCommand) {
	// Jaeger
	c.Flags().StringVar(&flags.Tracing.Jaeger.Endpoint, "tracing-jaeger-endpoint", "", "jaeger http-based thrift collector")
	c.Flags().StringToStringVar(&flags.Tracing.Jaeger.Tags, "tracing-jaeger-tags", nil, "tags to add to the components")
	c.Flags().StringVar(&flags.Tracing.Jaeger.Service, "tracing-jaeger-service", "web", "jaeger process service name")

	// Jaeger
	c.Flags().StringVar(&flags.Tracing.Stackdriver.ProjectID, "tracing-stackdriver-projectid", "", "GCP's Project ID")
}

func InitializePolicyFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringSliceVar(&flags.PolicyCheckers.Filter.HttpMethods, "policy-check-filter-http-method", nil, "API http method to go through policy check")
	c.Flags().StringSliceVar(&flags.PolicyCheckers.Filter.Actions, "policy-check-filter-action", nil, "Actions in the list will go through policy check")
	c.Flags().StringSliceVar(&flags.PolicyCheckers.Filter.ActionsToSkip, "policy-check-filter-action-skip", nil, "Actions the list will not go through policy check")
}

func InitializeServerFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringVar(&flags.Server.XFrameOptions, "x-frame-options", "deny", "The value to set for X-Frame-Options.")
	c.Flags().StringVar(&flags.Server.ClusterName, "cluster-name", "", "A name for this Concourse cluster, to be displayed on the dashboard page.")
	c.Flags().StringVar(&flags.Server.ClientID, "client-id", "concourse-web", "Client ID to use for login flow")
	c.Flags().StringVar(&flags.Server.ClientSecret, "client-secret", "", "Client secret to use for login flow")
}

func InitializeLogFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().BoolVar(&flags.Log.DBQueries, "log-db-queries", false, "Log database queries.")
	c.Flags().BoolVar(&flags.Log.ClusterName, "log-cluster-name", false, "Log cluster name.")
}

func InitializeGCFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().DurationVar(&flags.GC.Interval, "gc-interval", 30*time.Second, "Interval on which to perform garbage collection.")
	c.Flags().DurationVar(&flags.GC.OneOffBuildGracePeriod, "gc-one-off-grace-period", 5*time.Minute, "Period after which one-off build containers will be garbage-collected.")
	c.Flags().DurationVar(&flags.GC.MissingGracePeriod, "gc-missing-grace-period", 5*time.Minute, "Period after which to reap containers and volumes that were created but went missing from the worker.")
	c.Flags().DurationVar(&flags.GC.HijackGracePeriod, "gc-hijack-grace-period", 5*time.Minute, "Period after which hijacked containers will be garbage collected")
	c.Flags().DurationVar(&flags.GC.FailedGracePeriod, "gc-failed-grace-period", 120*time.Hour, "Period after which failed containers will be garbage collected")
	c.Flags().DurationVar(&flags.GC.CheckRecyclePeriod, "gc-check-recycle-period", 1*time.Minute, "Period after which to reap checks that are completed.")
}

func InitializeBuildLogRetentionFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().Uint64Var(&flags.BuildLogRetention.Default, "default-build-logs-to-retain", 0, "Default build logs to retain, 0 means all")
	c.Flags().Uint64Var(&flags.BuildLogRetention.Max, "max-build-logs-to-retain", 0, "Maximum build logs to retain, 0 means not specified. Will override values configured in jobs")

	c.Flags().Uint64Var(&flags.BuildLogRetention.DefaultDays, "default-days-to-retain-build-logs", 0, "Default days to retain build logs. 0 means unlimited")
	c.Flags().Uint64Var(&flags.BuildLogRetention.MaxDays, "max-days-to-retain-build-logs", 0, "Maximum days to retain build logs, 0 means not specified. Will override values configured in jobs")
}

func InitializeAuditorFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().BoolVar(&flags.Auditor.EnableBuildAuditLog, "enable-build-auditing", false, "Enable auditing for all api requests connected to builds.")
	c.Flags().BoolVar(&flags.Auditor.EnableContainerAuditLog, "enable-container-auditing", false, "Enable auditing for all api requests connected to containers.")
	c.Flags().BoolVar(&flags.Auditor.EnableJobAuditLog, "enable-job-auditing", false, "Enable auditing for all api requests connected to jobs.")
	c.Flags().BoolVar(&flags.Auditor.EnablePipelineAuditLog, "enable-pipeline-auditing", false, "Enable auditing for all api requests connected to pipelines.")
	c.Flags().BoolVar(&flags.Auditor.EnableResourceAuditLog, "enable-resource-auditing", false, "Enable auditing for all api requests connected to resources.")
	c.Flags().BoolVar(&flags.Auditor.EnableSystemAuditLog, "enable-system-auditing", false, "Enable auditing for all api requests connected to system transactions.")
	c.Flags().BoolVar(&flags.Auditor.EnableTeamAuditLog, "enable-team-auditing", false, "Enable auditing for all api requests connected to teams.")
	c.Flags().BoolVar(&flags.Auditor.EnableWorkerAuditLog, "enable-worker-auditing", false, "Enable auditing for all api requests connected to workers.")
	c.Flags().BoolVar(&flags.Auditor.EnableVolumeAuditLog, "enable-volume-auditing", false, "Enable auditing for all api requests connected to volumes.")
}

func InitializeSyslogFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringVar(&flags.Syslog.Hostname, "syslog-hostname", "atc-syslog-drainer", "Client hostname with which the build logs will be sent to the syslog server.")
	c.Flags().StringVar(&flags.Syslog.Address, "syslog-address", "", "Remote syslog server address with port (Example: 0.0.0.0:514).")
	c.Flags().StringVar(&flags.Syslog.Transport, "syslog-transport", "", "Transport protocol for syslog messages (Currently supporting tcp, udp & tls).")
	c.Flags().DurationVar(&flags.Syslog.DrainInterval, "syslog-drain-interval", 30*time.Second, "Interval over which checking is done for new build logs to send to syslog server (duration measurement units are s/m/h; eg. 30s/30m/1h)")
	c.Flags().StringSliceVar(&flags.Syslog.CACerts, "syslog-ca-cert", nil, "Paths to PEM-encoded CA cert files to use to verify the Syslog server SSL cert.")
}

func InitializeAuthFlags(c *cobra.Command, flags *RunCommand) {
	// Auth Flags
	c.Flags().BoolVar(&flags.Auth.AuthFlags.SecureCookies, "cookie-secure", false, "Force sending secure flag on http cookies")
	c.Flags().DurationVar(&flags.Auth.AuthFlags.Expiration, "auth-duration", 24*time.Hour, "Length of time for which tokens are valid. Afterwards, users will have to log back in.")
	c.Flags().StringVar(&flags.Auth.AuthFlags.SigningKey, "session-signing-key", "", "File containing an RSA private key, used to sign auth tokens.")
	c.Flags().StringToStringVar(&flags.Auth.AuthFlags.LocalUsers, "add-local-user", nil, "List of username:password combinations for all your local users. The password can be bcrypted - if so, it must have a minimum cost of 10. Ex. USERNAME:PASSWORD")
	c.Flags().StringToStringVar(&flags.Auth.AuthFlags.Clients, "add-client", nil, "List of client_id:client_secret combinations. Ex. CLIENT_ID:CLIENT_SECRET")

	// Main Team Flags
	c.Flags().StringSliceVar(&flags.Auth.MainTeamFlags.LocalUsers, "main-team-local-user", nil, "A whitelisted local concourse user. These are the users you've added at web startup with the --add-local-user flag. Ex. USERNAME")
	c.Flags().StringVarP(&flags.Auth.MainTeamFlags.Config, "main-team-config", "c", "", "Configuration file for specifying team params")
}

func InitializeSystemClaimFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().StringVar(&flags.SystemClaim.Key, "system-claim-key", "aud", "The token claim key to use when matching system-claim-values")
	c.Flags().StringSliceVar(&flags.SystemClaim.Values, "system-claim-value", []string{"concourse-worker"}, "Configure which token requests should be considered 'system' requests.")
}

func InitializeExperimentalFlags(c *cobra.Command, flags *RunCommand) {
	c.Flags().BoolVar(&flags.Experimental.EnableArchivePipeline, "enable-archive-pipeline", false, "Enable /api/v1/teams/{team}/pipelines/{pipeline}/archive endpoint.")

	c.Flags().BoolVar(&flags.Experimental.EnableBuildRerunWhenWorkerDisappears, "enable-rerun-when-worker-disappears", false, "Enable automatically build rerun when worker disappears")
}
