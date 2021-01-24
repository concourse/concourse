package atccmd

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/api/containerserver"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/component"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/conjur"
	"github.com/concourse/concourse/atc/creds/credhub"
	"github.com/concourse/concourse/atc/creds/dummy"
	"github.com/concourse/concourse/atc/creds/kubernetes"
	"github.com/concourse/concourse/atc/creds/noop"
	"github.com/concourse/concourse/atc/creds/secretsmanager"
	"github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
	"github.com/concourse/concourse/atc/syslog"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/concourse/atc/wrappa"
	clager "github.com/concourse/concourse/lager"
	"github.com/concourse/concourse/skymarshal/dexserver"
	"github.com/concourse/concourse/skymarshal/legacyserver"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/web"
	"github.com/concourse/retryhttp"
	"github.com/jessevdk/go-flags"

	"github.com/cppforlife/go-semi-semantic/version"
	multierror "github.com/hashicorp/go-multierror"
	gocache "github.com/patrickmn/go-cache"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"

	// dynamically registered metric emitters
	_ "github.com/concourse/concourse/atc/metric/emitter"

	// dynamically registered policy checkers
	_ "github.com/concourse/concourse/atc/policy/opa"

	// dynamically registered credential managers
	_ "github.com/concourse/concourse/atc/creds/conjur"
	_ "github.com/concourse/concourse/atc/creds/credhub"
	_ "github.com/concourse/concourse/atc/creds/dummy"
	_ "github.com/concourse/concourse/atc/creds/kubernetes"
	_ "github.com/concourse/concourse/atc/creds/secretsmanager"
	_ "github.com/concourse/concourse/atc/creds/ssm"
	_ "github.com/concourse/concourse/atc/creds/vault"
)

const algorithmLimitRows = 100

var schedulerCache = gocache.New(10*time.Second, 10*time.Second)

var defaultDriverName = "postgres"
var retryingDriverName = "too-many-connections-retrying"

var flyClientID = "fly"
var flyClientSecret = "Zmx5"

var workerAvailabilityPollingInterval = 5 * time.Second
var workerStatusPublishInterval = 1 * time.Minute
var BatcherInterval = 15 * time.Second

type RunCommand struct {
	Logger clager.Lager

	varSourcePool creds.VarSourcePool

	BindIP   net.IP `yaml:"bind_ip"`
	BindPort uint16 `yaml:"bind_port"`

	TLS struct {
		BindPort uint16 `yaml:"bind_port"`
		Cert     string `yaml:"cert" validate:"file"`
		Key      string `yaml:"key" validate:"file"`
	} `yaml:"tls"`

	LetsEncrypt struct {
		Enable  bool   `yaml:"enable"`
		ACMEURL string `yaml:"acme_url"`
	} `yaml:"lets_encrypt"`

	ExternalURL string `yaml:"external_url"`

	Database struct {
		Postgres struct {
			Host string `yaml:"host"`
			Port uint16 `yaml:"port"`

			Socket string `yaml:"socket"`

			User     string `yaml:"user"`
			Password string `yaml:"password"`

			SSLMode    string `yaml:"sslmode"`
			CACert     string `yaml:"ca_cert" validate:"file"`
			ClientCert string `yaml:"client_cert" validate:"file"`
			ClientKey  string `yaml:"client_key" validate:"file"`

			ConnectTimeout time.Duration `yaml:"connect_timeout"`

			Database string `yaml:"database"`
		} `yaml:"postgres"`

		ConcurrentRequestLimits   map[string]int `yaml:"concurrent_request_limit" validate:"keys,limited_route,endkeys"`
		APIMaxOpenConnections     int            `yaml:"api_max_conns"`
		BackendMaxOpenConnections int            `yaml:"backend_max_conns"`

		// XXX UNMARSHAL CIPHER KEY
		EncryptionKey    string `yaml:"encryption_key"`
		OldEncryptionKey string `yaml:"old_encryption_key"`
	} `yaml:"database"`

	CredentialManagement creds.CredentialManagementConfig `yaml:"credential_management"`
	CredentialManagers   struct {
		Conjur         conjur.Manager               `yaml:"conjur"`
		CredHub        credhub.CredHubManager       `yaml:"credhub"`
		Dummy          dummy.Manager                `yaml:"dummy_creds"`
		Kubernetes     kubernetes.KubernetesManager `yaml:"kubernetes"`
		SecretsManager secretsmanager.Manager       `yaml:"aws_secretsmanager"`
		SSM            ssm.SsmManager               `yaml:"aws_ssm"`
		Vault          vault.VaultManager           `yaml:"vault"`
	} `yaml:"credential_managers"`

	Debug struct {
		BindIP   net.IP `yaml:"bind_ip"`
		BindPort uint16 `yaml:"bind_port"`
	} `yaml:"debug"`

	InterceptIdleTimeout time.Duration `yaml:"intercept_idle_timeout"`

	EnableGlobalResources bool `yaml:"enable_global_resources"`

	ComponentRunnerInterval time.Duration `yaml:"component_runner_interval"`
	BuildTrackerInterval    time.Duration `yaml:"build_tracker_interval"`

	// XXX: Split into it's own package to reduce clutter??
	ResourceChecking struct {
		ScannerInterval time.Duration `yaml:"scanner_interval"`
		CheckerInterval time.Duration `yaml:"checker_interval"`

		Timeout                    time.Duration `yaml:"timeout"`
		DefaultInterval            time.Duration `yaml:"default_interval"`
		DefaultIntervalWithWebhook time.Duration `yaml:"default_interval_with_webhook"`
		MaxChecksPerSecond         int           `yaml:"max_checks_per_second"`
	} `yaml:"resource_checking"`

	JobScheduling struct {
		MaxInFlight uint64 `yaml:"max_in_flight"`
	} `yaml:"job_scheduling"`

	Runtime struct {
		ContainerPlacementStrategy        string        `yaml:"container_placement_strategy"`
		MaxActiveTasksPerWorker           int           `yaml:"max_active_tasks_per_worker"`
		BaggageclaimResponseHeaderTimeout time.Duration `yaml:"baggageclaim_response_header_timeout"`
		StreamingArtifactsCompression     string        `yaml:"streaming_artifacts_compression"`

		GardenRequestTimeout time.Duration `yaml:"garden_request_timeout"`
	} `yaml:"runtime"`

	CLIArtifactsDir string `yaml:"cli_artifacts_dir" validate:"dir"`

	Metrics struct {
		HostName            string            `yaml:"host_name"`
		Attributes          map[string]string `yaml:"attributes"`
		BufferSize          uint32            `yaml:"buffer_size"`
		CaptureErrorMetrics bool              `yaml:"capture_errors"`
	} `yaml:"metrics"`

	Tracing tracing.Config `yaml:"tracing"`

	PolicyCheckers struct {
		Filter policy.Filter `yaml:"filter"`
	} `yaml:"policy_checking"`

	Server struct {
		XFrameOptions string `yaml:"x_frame_options"`
		ClusterName   string `yaml:"cluster_name"`
		ClientID      string `yaml:"client_id"`
		ClientSecret  string `yaml:"client_secret"`
	} `yaml:"web_server"`

	Log struct {
		DBQueries   bool `yaml:"db_queries"`
		ClusterName bool `yaml:"cluster_name"`
	} `yaml:"log"`

	GC struct {
		Interval time.Duration `yaml:"interval"`

		OneOffBuildGracePeriod time.Duration `yaml:"one_off_grace_period"`
		MissingGracePeriod     time.Duration `yaml:"missing_grace_period"`
		HijackGracePeriod      time.Duration `yaml:"hijack_grace_period"`
		FailedGracePeriod      time.Duration `yaml:"failed_grace_period"`
		CheckRecyclePeriod     time.Duration `yaml:"check_recycle_period"`
	} `yaml:"garbage_collection"`

	// XXX NOT USED (HIDDEN)
	TelemetryOptIn bool `yaml:"telemetry_opt_in"`

	BuildLogRetention struct {
		Default uint64 `yaml:"default"`
		Max     uint64 `yaml:"max"`

		DefaultDays uint64 `yaml:"default_days"`
		MaxDays     uint64 `yaml:"max_days"`
	} `yaml:"build_log_retention"`

	DefaultCpuLimit    *int    `yaml:"default_task_cpu_limit"`
	DefaultMemoryLimit *string `yaml:"default_task_memory_limit"`

	// XXX: How to structure this?
	Auditor struct {
		EnableBuildAuditLog     bool `yaml:"enable_build`
		EnableContainerAuditLog bool `yaml:"enable_container`
		EnableJobAuditLog       bool `yaml:"enable_job`
		EnablePipelineAuditLog  bool `yaml:"enable_pipeline`
		EnableResourceAuditLog  bool `yaml:"enable_resource`
		EnableSystemAuditLog    bool `yaml:"enable_system`
		EnableTeamAuditLog      bool `yaml:"enable_team`
		EnableWorkerAuditLog    bool `yaml:"enable_worker`
		EnableVolumeAuditLog    bool `yaml:"enable_volume`
	} `yaml:"auditing"`

	Syslog struct {
		Hostname      string        `yaml:"hostname"`
		Address       string        `yaml:"address"`
		Transport     string        `yaml:"transport"`
		DrainInterval time.Duration `yaml:"drain_interval"`
		CACerts       []string      `yaml:"ca_cert"`
	} `yaml:"syslog"`

	Auth struct {
		AuthFlags     skycmd.AuthFlags
		MainTeamFlags skycmd.AuthTeamFlags
	} `yaml:"auth"`

	EnableRedactSecrets bool `yaml:"enable_redact_secrets"`

	ConfigRBAC string `yaml:"config_rbac" validate:"file"`

	SystemClaim struct {
		Key    string   `yaml:"key"`
		Values []string `yaml:"values"`
	} `yaml:"system_claim"`

	Experimental struct {
		EnableArchivePipeline                bool `yaml:"enable_archive_pipeline"`
		EnableBuildRerunWhenWorkerDisappears bool `yaml:"enable_rerun_when_worker_disappears"`
	} `yaml:"experimental"`
}

func SetDefaults(cmd *RunCommand) {
	cmd.BindIP = net.IPv4(0, 0, 0, 0)
	cmd.BindPort = 8080

	// Set postgres defaults
	cmd.Database.Postgres.Host = "127.0.0.1"
	cmd.Database.Postgres.Port = 5432
	cmd.Database.Postgres.SSLMode = "disable"
	cmd.Database.Postgres.ConnectTimeout = 5 * time.Minute
	cmd.Database.Postgres.Database = "atc"

	cmd.Database.APIMaxOpenConnections = 10
	cmd.Database.BackendMaxOpenConnections = 50

	cmd.Debug.BindIP = net.IPv4(127, 0, 0, 1)
	cmd.Debug.BindPort = 8079

	cmd.InterceptIdleTimeout = 0 * time.Minute

	cmd.ComponentRunnerInterval = 10 * time.Second

	// Set resource checking defaults
	cmd.ResourceChecking.ScannerInterval = 10 * time.Second
	cmd.ResourceChecking.CheckerInterval = 10 * time.Second
	cmd.ResourceChecking.Timeout = 1 * time.Hour
	cmd.ResourceChecking.DefaultInterval = 1 * time.Minute
	cmd.ResourceChecking.DefaultIntervalWithWebhook = 1 * time.Minute

	// Set runtime configuration defaults
	cmd.Runtime.ContainerPlacementStrategy = "volume-locality"
	cmd.Runtime.MaxActiveTasksPerWorker = 0
	cmd.Runtime.BaggageclaimResponseHeaderTimeout = 1 * time.Minute
	cmd.Runtime.StreamingArtifactsCompression = "gzip"
	cmd.Runtime.GardenRequestTimeout = 5 * time.Minute

	cmd.Metrics.BufferSize = 1000

	// Set conjur configuration defaults
	cmd.CredentialManagers.Conjur.PipelineSecretTemplate = "concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
	cmd.CredentialManagers.Conjur.TeamSecretTemplate = "concourse/{{.Team}}/{{.Secret}}"
	cmd.CredentialManagers.Conjur.SecretTemplate = "vaultName/{{.Secret}}"

	// Set credhub configuration defaults
	cmd.CredentialManagers.CredHub.PathPrefix = "/concourse"

	// Set kuberenetes configuration defaults
	cmd.CredentialManagers.Kubernetes.NamespacePrefix = "concourse-"

	// Set aws secrets manager configuration defaults
	cmd.CredentialManagers.SecretsManager.PipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
	cmd.CredentialManagers.SecretsManager.TeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

	// Set aws ssm configuration defaults
	cmd.CredentialManagers.SSM.PipelineSecretTemplate = "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}"
	cmd.CredentialManagers.SSM.TeamSecretTemplate = "/concourse/{{.Team}}/{{.Secret}}"

	// Set vault configuration defaults
	cmd.CredentialManagers.Vault.PathPrefix = "/concourse"
	cmd.CredentialManagers.Vault.LookupTemplates = []string{"/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"}
	cmd.CredentialManagers.Vault.Auth.RetryMax = 5 * time.Minute
	cmd.CredentialManagers.Vault.Auth.RetryInitial = 1 * time.Second
}

func (m *Migration) Execute(args []string) error {
	if m.CurrentDBVersion {
		return m.currentDBVersion()
	}
	if m.SupportedDBVersion {
		return m.supportedDBVersion()
	}
	if m.MigrateDBToVersion > 0 {
		return m.migrateDBToVersion()
	}
	return errors.New("must specify one of `--current-db-version`, `--supported-db-version`, or `--migrate-db-to-version`")

}

func (cmd *Migration) currentDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		encryption.NewNoEncryption(),
	)

	version, err := helper.CurrentVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *Migration) supportedDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		encryption.NewNoEncryption(),
	)

	version, err := helper.SupportedVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *Migration) migrateDBToVersion() error {
	version := cmd.MigrateDBToVersion

	// XXX: Can this use runcommand.NewKey ????
	var newKey *encryption.Key
	if cmd.EncryptionKey != nil {
		AEAD, err := parseAEAD(cmd.EncryptionKey)
		if err != nil {
			return err
		}

		newKey = encryption.NewKey(AEAD)
	}

	var strategy encryption.Strategy
	if newKey != nil {
		strategy = newKey
	} else {
		strategy = encryption.NewNoEncryption()
	}

	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		strategy,
	)

	err := helper.MigrateToVersion(version)
	if err != nil {
		return fmt.Errorf("Could not migrate to version: %d Reason: %s", version, err.Error())
	}

	fmt.Println("Successfully migrated to version:", version)
	return nil
}

func (cmd *ATCCommand) WireDynamicFlags(commandFlags *flags.Command) {
	cmd.RunCommand.WireDynamicFlags(commandFlags)
}

func (cmd *RunCommand) WireDynamicFlags(commandFlags *flags.Command) {
	var (
		metricsGroup      *flags.Group
		policyChecksGroup *flags.Group
		credsGroup        *flags.Group
		authGroup         *flags.Group
	)

	groups := commandFlags.Groups()
	for i := 0; i < len(groups); i++ {
		group := groups[i]

		if credsGroup == nil && group.ShortDescription == "Credential Management" {
			credsGroup = group
		}

		if metricsGroup == nil && group.ShortDescription == "Metrics & Diagnostics" {
			metricsGroup = group
		}

		if policyChecksGroup == nil && group.ShortDescription == "Policy Checking" {
			policyChecksGroup = group
		}

		if authGroup == nil && group.ShortDescription == "Authentication" {
			authGroup = group
		}

		if metricsGroup != nil && credsGroup != nil && authGroup != nil && policyChecksGroup != nil {
			break
		}

		groups = append(groups, group.Groups()...)
	}

	if metricsGroup == nil {
		panic("could not find Metrics & Diagnostics group for registering emitters")
	}

	if policyChecksGroup == nil {
		panic("could not find Policy Checking group for registering policy checkers")
	}

	if credsGroup == nil {
		panic("could not find Credential Management group for registering managers")
	}

	if authGroup == nil {
		panic("could not find Authentication group for registering connectors")
	}

	managerConfigs := make(creds.Managers)
	for name, p := range creds.ManagerFactories() {
		managerConfigs[name] = p.AddConfig(credsGroup)
	}
	cmd.CredentialManagers = managerConfigs

	metric.WireEmitters(metricsGroup)

	policy.WireCheckers(policyChecksGroup)

	skycmd.WireConnectors(authGroup)
	skycmd.WireTeamConnectors(authGroup.Find("Authentication (Main Team)"))
}

func (cmd *RunCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *RunCommand) Runner(positionalArguments []string) (ifrit.Runner, error) {
	if cmd.ExternalURL.URL == nil {
		cmd.ExternalURL = cmd.DefaultURL()
	}

	if len(positionalArguments) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments: %v", positionalArguments)
	}

	err := cmd.validate()
	if err != nil {
		return nil, err
	}

	logger, reconfigurableSink := cmd.Logger.Logger("atc")
	if cmd.LogClusterName {
		logger = logger.WithData(lager.Data{
			"cluster": cmd.Server.ClusterName,
		})
	}

	commandSession := logger.Session("cmd")
	startTime := time.Now()

	commandSession.Info("start")
	defer commandSession.Info("finish", lager.Data{
		"duration": time.Now().Sub(startTime),
	})

	atc.EnableGlobalResources = cmd.EnableGlobalResources

	//FIXME: These only need to run once for the entire binary. At the moment,
	//they rely on state of the command.
	db.SetupConnectionRetryingDriver(
		"postgres",
		cmd.Postgres.ConnectionString(),
		retryingDriverName,
	)

	// Register the sink that collects error metrics
	if cmd.Metrics.CaptureErrorMetrics {
		errorSinkCollector := metric.NewErrorSinkCollector(logger)
		logger.RegisterSink(&errorSinkCollector)
	}

	err = cmd.Tracing.Prepare()
	if err != nil {
		return nil, err
	}

	http.HandleFunc("/debug/connections", func(w http.ResponseWriter, r *http.Request) {
		for _, stack := range db.GlobalConnectionTracker.Current() {
			fmt.Fprintln(w, stack)
		}
	})

	if err := cmd.configureMetrics(logger); err != nil {
		return nil, err
	}

	lockConn, err := cmd.constructLockConn(retryingDriverName)
	if err != nil {
		return nil, err
	}

	lockFactory := lock.NewLockFactory(lockConn, metric.LogLockAcquired, metric.LogLockReleased)

	apiConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.APIMaxOpenConnections, "api", lockFactory)
	if err != nil {
		return nil, err
	}

	backendConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.BackendMaxOpenConnections, "backend", lockFactory)
	if err != nil {
		return nil, err
	}

	gcConn, err := cmd.constructDBConn(retryingDriverName, logger, 5, "gc", lockFactory)
	if err != nil {
		return nil, err
	}

	storage, err := storage.NewPostgresStorage(logger, cmd.Postgres)
	if err != nil {
		return nil, err
	}

	secretManager, err := cmd.secretManager(logger)
	if err != nil {
		return nil, err
	}

	cmd.varSourcePool = creds.NewVarSourcePool(
		logger.Session("var-source-pool"),
		5*time.Minute,
		1*time.Minute,
		clock.NewClock(),
	)

	members, err := cmd.constructMembers(logger, reconfigurableSink, apiConn, backendConn, gcConn, storage, lockFactory, secretManager)
	if err != nil {
		return nil, err
	}

	members = append(members, grouper.Member{
		Name: "periodic-metrics",
		Runner: metric.PeriodicallyEmit(
			logger.Session("periodic-metrics"),
			10*time.Second,
		),
	})

	onReady := func() {
		logData := lager.Data{
			"http":  cmd.nonTLSBindAddr(),
			"debug": cmd.debugBindAddr(),
		}

		if cmd.isTLSEnabled() {
			logData["https"] = cmd.tlsBindAddr()
		}

		logger.Info("listening", logData)
	}

	onExit := func() {
		for _, closer := range []Closer{lockConn, apiConn, backendConn, gcConn, storage} {
			closer.Close()
		}

		cmd.varSourcePool.Close()
	}

	return run(grouper.NewParallel(os.Interrupt, members), onReady, onExit), nil
}

func (cmd *RunCommand) constructMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	apiConn db.Conn,
	backendConn db.Conn,
	gcConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
) ([]grouper.Member, error) {
	if cmd.TelemetryOptIn {
		url := fmt.Sprintf("http://telemetry.concourse-ci.org/?version=%s", concourse.Version)
		go func() {
			_, err := http.Get(url)
			if err != nil {
				logger.Error("telemetry-version", err)
			}
		}()
	}

	policyChecker, err := policy.Initialize(logger, cmd.Server.ClusterName, concourse.Version, cmd.PolicyCheckers.Filter)
	if err != nil {
		return nil, err
	}

	apiMembers, err := cmd.constructAPIMembers(logger, reconfigurableSink, apiConn, storage, lockFactory, secretManager, policyChecker)
	if err != nil {
		return nil, err
	}

	backendComponents, err := cmd.backendComponents(logger, backendConn, lockFactory, secretManager, policyChecker)
	if err != nil {
		return nil, err
	}

	gcComponents, err := cmd.gcComponents(logger, gcConn, lockFactory)
	if err != nil {
		return nil, err
	}

	// use backendConn so that the Component objects created by the factory uses
	// the backend connection pool when reloading.
	componentFactory := db.NewComponentFactory(backendConn)
	bus := backendConn.Bus()

	members := apiMembers
	components := append(backendComponents, gcComponents...)
	for _, c := range components {
		dbComponent, err := componentFactory.CreateOrUpdate(c.Component)
		if err != nil {
			return nil, err
		}

		componentLogger := logger.Session(c.Component.Name)

		members = append(members, grouper.Member{
			Name: c.Component.Name,
			Runner: &component.Runner{
				Logger:    componentLogger,
				Interval:  cmd.ComponentRunnerInterval,
				Component: dbComponent,
				Bus:       bus,
				Schedulable: &component.Coordinator{
					Locker:    lockFactory,
					Component: dbComponent,
					Runnable:  c.Runnable,
				},
			},
		})

		if drainable, ok := c.Runnable.(component.Drainable); ok {
			members = append(members, grouper.Member{
				Name: c.Component.Name + "-drainer",
				Runner: drainRunner{
					logger:  componentLogger.Session("drain"),
					drainer: drainable,
				},
			})
		}
	}

	return members, nil
}

func (cmd *RunCommand) constructAPIMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	dbConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
	policyChecker *policy.Checker,
) ([]grouper.Member, error) {

	httpClient, err := cmd.skyHttpClient()
	if err != nil {
		return nil, err
	}

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	_, err = teamFactory.CreateDefaultTeamIfNotExists()
	if err != nil {
		return nil, err
	}

	err = cmd.configureAuthForDefaultTeam(teamFactory)
	if err != nil {
		return nil, err
	}

	userFactory := db.NewUserFactory(dbConn)

	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := worker.NewFetchSourceFactory(dbResourceCacheFactory)
	resourceFetcher := worker.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
	)

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}

	compressionLib := compression.NewGzipCompression()
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory, compressionLib),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
		cmd.GardenRequestTimeout,
		policyChecker,
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider, compressionLib, workerAvailabilityPollingInterval, workerStatusPublishInterval)

	credsManagers := cmd.CredentialManagers
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbResourceFactory := db.NewResourceFactory(dbConn, lockFactory)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	gcContainerDestroyer := gc.NewDestroyer(logger, dbContainerRepository, dbVolumeRepository)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, cmd.GlobalResourceCheckTimeout)
	dbClock := db.NewClock()
	dbWall := db.NewWall(dbConn, &dbClock)

	tokenVerifier := cmd.constructTokenVerifier(httpClient)

	accessFactory := accessor.NewAccessFactory(
		cmd.SystemClaimKey,
		cmd.SystemClaimValues,
	)

	middleware := token.NewMiddleware(cmd.Auth.AuthFlags.SecureCookies)

	apiHandler, err := cmd.constructAPIHandler(
		logger,
		reconfigurableSink,
		teamFactory,
		dbPipelineFactory,
		dbJobFactory,
		dbResourceFactory,
		dbWorkerFactory,
		dbVolumeRepository,
		dbContainerRepository,
		gcContainerDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		dbResourceConfigFactory,
		userFactory,
		workerClient,
		secretManager,
		credsManagers,
		accessFactory,
		dbWall,
		tokenVerifier,
		dbConn.Bus(),
		policyChecker,
	)
	if err != nil {
		return nil, err
	}

	webHandler, err := cmd.constructWebHandler(logger)
	if err != nil {
		return nil, err
	}

	authHandler, err := cmd.constructAuthHandler(
		logger,
		storage,
	)
	if err != nil {
		return nil, err
	}

	loginHandler, err := cmd.constructLoginHandler(
		logger,
		httpClient,
		middleware,
	)
	if err != nil {
		return nil, err
	}

	legacyHandler, err := cmd.constructLegacyHandler(
		logger,
	)
	if err != nil {
		return nil, err
	}

	var httpHandler, httpsHandler http.Handler
	if cmd.isTLSEnabled() {
		httpHandler = cmd.constructHTTPHandler(
			logger,

			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   webHandler,
			},

			// note: intentionally not wrapping API; redirecting is more trouble than
			// it's worth.

			// we're mainly interested in having the web UI consistently https:// -
			// API requests will likely not respect the redirected https:// URI upon
			// the next request, plus the payload will have already been sent in
			// plaintext
			apiHandler,

			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   authHandler,
			},
			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   loginHandler,
			},
			tlsRedirectHandler{
				matchHostname: cmd.ExternalURL.URL.Hostname(),
				externalHost:  cmd.ExternalURL.URL.Host,
				baseHandler:   legacyHandler,
			},
			middleware,
		)

		httpsHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			apiHandler,
			authHandler,
			loginHandler,
			legacyHandler,
			middleware,
		)
	} else {
		httpHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			apiHandler,
			authHandler,
			loginHandler,
			legacyHandler,
			middleware,
		)
	}

	members := []grouper.Member{
		{Name: "debug", Runner: http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},
		{Name: "web", Runner: http_server.New(
			cmd.nonTLSBindAddr(),
			httpHandler,
		)},
	}

	if httpsHandler != nil {
		tlsConfig, err := cmd.tlsConfig(logger, dbConn)
		if err != nil {
			return nil, err
		}
		members = append(members, grouper.Member{Name: "web-tls", Runner: http_server.NewTLSServer(
			cmd.tlsBindAddr(),
			httpsHandler,
			tlsConfig,
		)})
	}

	return members, nil
}

func (cmd *RunCommand) backendComponents(
	logger lager.Logger,
	dbConn db.Conn,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
	policyChecker *policy.Checker,
) ([]RunnableComponent, error) {

	if cmd.Syslog.Address != "" && cmd.Syslog.Transport == "" {
		return nil, fmt.Errorf("syslog Drainer is misconfigured, cannot configure a drainer without a transport")
	}

	syslogDrainConfigured := true
	if cmd.Syslog.Address == "" {
		syslogDrainConfigured = false
	}

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := worker.NewFetchSourceFactory(dbResourceCacheFactory)
	resourceFetcher := worker.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
	)

	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, cmd.GlobalResourceCheckTimeout)
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbCheckableCounter := db.NewCheckableCounter(dbConn)

	alg := algorithm.New(db.NewVersionsDB(dbConn, algorithmLimitRows, schedulerCache))

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}

	var compressionLib compression.Compression
	if cmd.StreamingArtifactsCompression == "zstd" {
		compressionLib = compression.NewZstdCompression()
	} else {
		compressionLib = compression.NewGzipCompression()
	}
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory, compressionLib),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
		cmd.GardenRequestTimeout,
		policyChecker,
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool,
		workerProvider,
		compressionLib,
		workerAvailabilityPollingInterval,
		workerStatusPublishInterval)

	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	buildContainerStrategy, err := cmd.chooseBuildContainerStrategy()
	if err != nil {
		return nil, err
	}

	engine := cmd.constructEngine(
		pool,
		workerClient,
		resourceFactory,
		teamFactory,
		dbBuildFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		secretManager,
		defaultLimits,
		buildContainerStrategy,
		lockFactory,
	)

	// In case that a user configures resource-checking-interval, but forgets to
	// configure resource-with-webhook-checking-interval, keep both checking-
	// intervals consistent. Even if both intervals are configured, there is no
	// reason webhooked resources take shorter checking interval than normal
	// resources.
	if cmd.ResourceWithWebhookCheckingInterval < cmd.ResourceCheckingInterval {
		logger.Info("update-resource-with-webhook-checking-interval",
			lager.Data{
				"oldValue": cmd.ResourceWithWebhookCheckingInterval,
				"newValue": cmd.ResourceCheckingInterval,
			})
		cmd.ResourceWithWebhookCheckingInterval = cmd.ResourceCheckingInterval
	}

	components := []RunnableComponent{
		{
			Component: atc.Component{
				Name:     atc.ComponentLidarScanner,
				Interval: cmd.LidarScannerInterval,
			},
			Runnable: lidar.NewScanner(
				logger.Session(atc.ComponentLidarScanner),
				dbCheckFactory,
				secretManager,
				cmd.GlobalResourceCheckTimeout,
				cmd.ResourceCheckingInterval,
				cmd.ResourceWithWebhookCheckingInterval,
			),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentLidarChecker,
				Interval: cmd.LidarCheckerInterval,
			},
			Runnable: lidar.NewChecker(
				logger.Session(atc.ComponentLidarChecker),
				dbCheckFactory,
				engine,
				lidar.CheckRateCalculator{
					MaxChecksPerSecond:       cmd.MaxChecksPerSecond,
					ResourceCheckingInterval: cmd.ResourceCheckingInterval,
					CheckableCounter:         dbCheckableCounter,
				},
			),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentScheduler,
				Interval: 10 * time.Second,
			},
			Runnable: scheduler.NewRunner(
				logger.Session("scheduler"),
				dbJobFactory,
				&scheduler.Scheduler{
					Algorithm: alg,
					BuildStarter: scheduler.NewBuildStarter(
						builds.NewPlanner(
							atc.NewPlanFactory(time.Now().Unix()),
						),
						alg),
				},
				cmd.JobSchedulingMaxInFlight,
			),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentBuildTracker,
				Interval: cmd.BuildTrackerInterval,
			},
			Runnable: builds.NewTracker(dbBuildFactory, engine),
		},
		{
			Component: atc.Component{
				Name:     atc.ComponentBuildReaper,
				Interval: 30 * time.Second,
			},
			Runnable: gc.NewBuildLogCollector(
				dbPipelineFactory,
				500,
				gc.NewBuildLogRetentionCalculator(
					cmd.DefaultBuildLogsToRetain,
					cmd.MaxBuildLogsToRetain,
					cmd.DefaultDaysToRetainBuildLogs,
					cmd.MaxDaysToRetainBuildLogs,
				),
				syslogDrainConfigured,
			),
		},
	}

	if syslogDrainConfigured {
		components = append(components, RunnableComponent{
			Component: atc.Component{
				Name:     atc.ComponentSyslogDrainer,
				Interval: cmd.Syslog.DrainInterval,
			},
			Runnable: syslog.NewDrainer(
				cmd.Syslog.Transport,
				cmd.Syslog.Address,
				cmd.Syslog.Hostname,
				cmd.Syslog.CACerts,
				dbBuildFactory,
			),
		})
	}

	return components, err
}

func (cmd *RunCommand) gcComponents(
	logger lager.Logger,
	gcConn db.Conn,
	lockFactory lock.LockFactory,
) ([]RunnableComponent, error) {
	dbWorkerLifecycle := db.NewWorkerLifecycle(gcConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(gcConn)
	dbContainerRepository := db.NewContainerRepository(gcConn)
	dbArtifactLifecycle := db.NewArtifactLifecycle(gcConn)
	dbCheckLifecycle := db.NewCheckLifecycle(gcConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(gcConn)
	dbBuildFactory := db.NewBuildFactory(gcConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbResourceConfigFactory := db.NewResourceConfigFactory(gcConn, lockFactory)

	dbVolumeRepository := db.NewVolumeRepository(gcConn)

	collectors := map[string]component.Runnable{
		atc.ComponentCollectorBuilds:            gc.NewBuildCollector(dbBuildFactory),
		atc.ComponentCollectorWorkers:           gc.NewWorkerCollector(dbWorkerLifecycle),
		atc.ComponentCollectorResourceConfigs:   gc.NewResourceConfigCollector(dbResourceConfigFactory),
		atc.ComponentCollectorResourceCaches:    gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorResourceCacheUses: gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorArtifacts:         gc.NewArtifactCollector(dbArtifactLifecycle),
		atc.ComponentCollectorChecks:            gc.NewCheckCollector(dbCheckLifecycle, cmd.GC.CheckRecyclePeriod),
		atc.ComponentCollectorVolumes:           gc.NewVolumeCollector(dbVolumeRepository, cmd.GC.MissingGracePeriod),
		atc.ComponentCollectorContainers:        gc.NewContainerCollector(dbContainerRepository, cmd.GC.MissingGracePeriod, cmd.GC.HijackGracePeriod),
		atc.ComponentCollectorCheckSessions:     gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle),
	}

	var components []RunnableComponent
	for collectorName, collector := range collectors {
		components = append(components, RunnableComponent{
			Component: atc.Component{
				Name:     collectorName,
				Interval: cmd.GC.Interval,
			},
			Runnable: collector,
		})
	}

	return components, nil
}

func (cmd *RunCommand) validateCustomRoles() error {
	path := cmd.ConfigRBAC.Path()
	if path == "" {
		return nil
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to open RBAC config file (%s): %w", cmd.ConfigRBAC, err)
	}

	var data map[string][]string
	if err = yaml.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("failed to parse RBAC config file (%s): %w", cmd.ConfigRBAC, err)
	}

	allKnownRoles := map[string]bool{}
	for _, roleName := range accessor.DefaultRoles {
		allKnownRoles[roleName] = true
	}

	for role, actions := range data {
		if _, ok := allKnownRoles[role]; !ok {
			return fmt.Errorf("failed to customize roles: %w", fmt.Errorf("unknown role %s", role))
		}

		for _, action := range actions {
			if _, ok := accessor.DefaultRoles[action]; !ok {
				return fmt.Errorf("failed to customize roles: %w", fmt.Errorf("unknown action %s", action))
			}
		}
	}

	return nil
}

func (cmd *RunCommand) parseCustomRoles() (map[string]string, error) {
	mapping := map[string]string{}

	path := cmd.ConfigRBAC.Path()
	if path == "" {
		return mapping, nil
	}

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data map[string][]string
	if err = yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	for role, actions := range data {
		for _, action := range actions {
			mapping[action] = role
		}
	}

	return mapping, nil
}

func workerVersion() (version.Version, error) {
	return version.NewVersionFromString(concourse.WorkerVersion)
}

func (cmd *RunCommand) secretManager(logger lager.Logger) (creds.Secrets, error) {
	var secretsFactory creds.SecretsFactory = noop.NewNoopFactory()
	for name, manager := range cmd.CredentialManagers {
		if !manager.IsConfigured() {
			continue
		}

		credsLogger := logger.Session("credential-manager", lager.Data{
			"name": name,
		})

		credsLogger.Info("configured credentials manager")

		err := manager.Init(credsLogger)
		if err != nil {
			return nil, err
		}

		err = manager.Validate()
		if err != nil {
			return nil, fmt.Errorf("credential manager '%s' misconfigured: %s", name, err)
		}

		secretsFactory, err = manager.NewSecretsFactory(credsLogger)
		if err != nil {
			return nil, err
		}

		break
	}

	result := secretsFactory.NewSecrets()
	result = creds.NewRetryableSecrets(result, cmd.CredentialManagement.RetryConfig)
	if cmd.CredentialManagement.CacheConfig.Enabled {
		result = creds.NewCachedSecrets(result, cmd.CredentialManagement.CacheConfig)
	}
	return result, nil
}

func (cmd *RunCommand) newKey() (*encryption.Key, error) {
	var newKey *encryption.Key
	if cmd.EncryptionKey != "" {
		AEAD, err := parseAEAD(cmd.EncryptionKey)
		if err != nil {
			return nil, err
		}

		newKey = encryption.NewKey(AEAD)
	}

	return newKey, nil
}

func (cmd *RunCommand) oldKey() (*encryption.Key, error) {
	var oldKey *encryption.Key
	if cmd.OldEncryptionKey != "" {
		AEAD, err := parseAEAD(cmd.OldEncryptionKey)
		if err != nil {
			return nil, err
		}

		oldKey = encryption.NewKey(AEAD)
	}
	return oldKey, nil
}

func (cmd *RunCommand) constructWebHandler(logger lager.Logger) (http.Handler, error) {
	webHandler, err := web.NewHandler(logger)
	if err != nil {
		return nil, err
	}
	return metric.WrapHandler(logger, "web", webHandler), nil
}

func (cmd *RunCommand) skyHttpClient() (*http.Client, error) {
	httpClient := http.DefaultClient

	if cmd.isTLSEnabled() {
		certpool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		if !cmd.LetsEncrypt.Enable {
			// XXX: do this for file
			// abs, err := filepath.Abs(value)
			// if err != nil {
			// 	return false
			// }

			cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
			if err != nil {
				return nil, err
			}

			x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				return nil, err
			}

			certpool.AddCert(x509Cert)
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certpool,
			},
		}
	} else {
		httpClient.Transport = http.DefaultTransport
	}

	httpClient.Transport = mitmRoundTripper{
		RoundTripper: httpClient.Transport,

		SourceHost: cmd.ExternalURL.URL.Host,
		TargetURL:  cmd.DefaultURL().URL,
	}

	return httpClient, nil
}

type mitmRoundTripper struct {
	http.RoundTripper

	SourceHost string
	TargetURL  *url.URL
}

func (tripper mitmRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == tripper.SourceHost {
		req.URL.Scheme = tripper.TargetURL.Scheme
		req.URL.Host = tripper.TargetURL.Host
	}

	return tripper.RoundTripper.RoundTrip(req)
}

func (cmd *RunCommand) tlsConfig(logger lager.Logger, dbConn db.Conn) (*tls.Config, error) {
	var tlsConfig *tls.Config
	tlsConfig = atc.DefaultTLSConfig()

	if cmd.isTLSEnabled() {
		tlsLogger := logger.Session("tls-enabled")
		if cmd.LetsEncrypt.Enable {
			tlsLogger.Debug("using-autocert-manager")

			cache, err := newDbCache(dbConn)
			if err != nil {
				return nil, err
			}
			m := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      cache,
				HostPolicy: autocert.HostWhitelist(cmd.ExternalURL.URL.Hostname()),
				Client:     &acme.Client{DirectoryURL: cmd.LetsEncrypt.ACMEURL.String()},
			}
			tlsConfig.NextProtos = append(tlsConfig.NextProtos, acme.ALPNProto)
			tlsConfig.GetCertificate = m.GetCertificate
		} else {
			tlsLogger.Debug("loading-tls-certs")
			cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsConfig, nil
}

func (cmd *RunCommand) parseDefaultLimits() (atc.ContainerLimits, error) {
	return atc.ParseContainerLimits(map[string]interface{}{
		"cpu":    cmd.DefaultCpuLimit,
		"memory": cmd.DefaultMemoryLimit,
	})
}

func (cmd *RunCommand) defaultBindIP() net.IP {
	URL := cmd.BindIP.String()
	if URL == "0.0.0.0" {
		URL = "127.0.0.1"
	}

	return net.ParseIP(URL)
}

func (cmd *RunCommand) DefaultURL() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", cmd.defaultBindIP().String(), cmd.BindPort),
	}
}

func run(runner ifrit.Runner, onReady func(), onExit func()) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		process := ifrit.Background(runner)

		subExited := process.Wait()
		subReady := process.Ready()

		for {
			select {
			case <-subReady:
				onReady()
				close(ready)
				subReady = nil
			case err := <-subExited:
				onExit()
				return err
			case sig := <-signals:
				process.Signal(sig)
			}
		}
	})
}

func (cmd *RunCommand) validate() error {
	var errs *multierror.Error

	switch {
	case cmd.TLSBindPort == 0:
		if cmd.TLSCert != "" || cmd.TLSKey != "" || cmd.LetsEncrypt.Enable {
			errs = multierror.Append(
				errs,
				errors.New("must specify --tls-bind-port to use TLS"),
			)
		}
	case cmd.LetsEncrypt.Enable:
		if cmd.TLSCert != "" || cmd.TLSKey != "" {
			errs = multierror.Append(
				errs,
				errors.New("cannot specify --enable-lets-encrypt if --tls-cert or --tls-key are set"),
			)
		}
	case cmd.TLSCert != "" && cmd.TLSKey != "":
		if cmd.ExternalURL.URL.Scheme != "https" {
			errs = multierror.Append(
				errs,
				errors.New("must specify HTTPS external-url to use TLS"),
			)
		}
	default:
		errs = multierror.Append(
			errs,
			errors.New("must specify --tls-cert and --tls-key, or --enable-lets-encrypt to use TLS"),
		)
	}

	if err := cmd.validateCustomRoles(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

func (cmd *RunCommand) nonTLSBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
}

func (cmd *RunCommand) tlsBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.TLSBindPort)
}

func (cmd *RunCommand) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.DebugBindIP, cmd.DebugBindPort)
}

func (cmd *RunCommand) configureMetrics(logger lager.Logger) error {
	host := cmd.Metrics.HostName
	if host == "" {
		host, _ = os.Hostname()
	}

	return metric.Initialize(logger.Session("metrics"), host, cmd.Metrics.Attributes, cmd.Metrics.BufferSize)
}

func (cmd *RunCommand) constructDBConn(
	driverName string,
	logger lager.Logger,
	maxConn int,
	connectionName string,
	lockFactory lock.LockFactory,
) (db.Conn, error) {
	dbConn, err := db.Open(logger.Session("db"), driverName, cmd.Postgres.ConnectionString(), cmd.newKey(), cmd.oldKey(), connectionName, lockFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	// Instrument with Metrics
	dbConn = metric.CountQueries(dbConn)
	metric.Databases = append(metric.Databases, dbConn)

	// Instrument with Logging
	if cmd.LogDBQueries {
		dbConn = db.Log(logger.Session("log-conn"), dbConn)
	}

	// Prepare
	dbConn.SetMaxOpenConns(maxConn)
	dbConn.SetMaxIdleConns(maxConn / 2)

	return dbConn, nil
}

type Closer interface {
	Close() error
}

func (cmd *RunCommand) constructLockConn(driverName string) (*sql.DB, error) {
	dbConn, err := sql.Open(driverName, cmd.Postgres.ConnectionString())
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)
	dbConn.SetConnMaxLifetime(0)

	return dbConn, nil
}

func (cmd *RunCommand) chooseBuildContainerStrategy() (worker.ContainerPlacementStrategy, error) {
	var strategy worker.ContainerPlacementStrategy
	if cmd.ContainerPlacementStrategy != "limit-active-tasks" && cmd.MaxActiveTasksPerWorker != 0 {
		return nil, errors.New("max-active-tasks-per-worker has only effect with limit-active-tasks strategy")
	}
	if cmd.MaxActiveTasksPerWorker < 0 {
		return nil, errors.New("max-active-tasks-per-worker must be greater or equal than 0")
	}
	switch cmd.ContainerPlacementStrategy {
	case "random":
		strategy = worker.NewRandomPlacementStrategy()
	case "fewest-build-containers":
		strategy = worker.NewFewestBuildContainersPlacementStrategy()
	case "limit-active-tasks":
		strategy = worker.NewLimitActiveTasksPlacementStrategy(cmd.MaxActiveTasksPerWorker)
	default:
		strategy = worker.NewVolumeLocalityPlacementStrategy()
	}

	return strategy, nil
}

func (cmd *RunCommand) configureAuthForDefaultTeam(teamFactory db.TeamFactory) error {
	team, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("default team not found")
	}

	auth, err := cmd.Auth.MainTeamFlags.Format()
	if err != nil {
		return fmt.Errorf("default team auth not configured: %v", err)
	}

	err = team.UpdateProviderAuth(auth)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *RunCommand) constructEngine(
	workerPool worker.Pool,
	workerClient worker.Client,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	secretManager creds.Secrets,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
) engine.Engine {

	stepFactory := builder.NewStepFactory(
		workerPool,
		workerClient,
		resourceFactory,
		teamFactory,
		buildFactory,
		resourceCacheFactory,
		resourceConfigFactory,
		defaultLimits,
		strategy,
		lockFactory,
		cmd.EnableBuildRerunWhenWorkerDisappears,
	)

	stepBuilder := builder.NewStepBuilder(
		stepFactory,
		builder.NewDelegateFactory(),
		cmd.ExternalURL.String(),
		secretManager,
		cmd.varSourcePool,
		cmd.EnableRedactSecrets,
	)

	return engine.NewEngine(stepBuilder)
}

func (cmd *RunCommand) constructHTTPHandler(
	logger lager.Logger,
	webHandler http.Handler,
	apiHandler http.Handler,
	authHandler http.Handler,
	loginHandler http.Handler,
	legacyHandler http.Handler,
	middleware token.Middleware,
) http.Handler {

	csrfHandler := auth.CSRFValidationHandler(
		apiHandler,
		middleware,
	)

	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", csrfHandler)
	webMux.Handle("/sky/issuer/", authHandler)
	webMux.Handle("/sky/", loginHandler)
	webMux.Handle("/auth/", legacyHandler)
	webMux.Handle("/login", legacyHandler)
	webMux.Handle("/logout", legacyHandler)
	webMux.Handle("/", webHandler)

	httpHandler := wrappa.LoggerHandler{
		Logger: logger,

		Handler: wrappa.SecurityHandler{
			XFrameOptions: cmd.Server.XFrameOptions,

			// proxy Authorization header to/from auth cookie,
			// to support auth from JS (EventSource) and custom JWT auth
			Handler: auth.WebAuthHandler{
				Handler:    webMux,
				Middleware: middleware,
			},
		},
	}

	return httpHandler
}

func (cmd *RunCommand) constructLegacyHandler(
	logger lager.Logger,
) (http.Handler, error) {
	return legacyserver.NewLegacyServer(&legacyserver.LegacyConfig{
		Logger: logger.Session("legacy"),
	})
}

func (cmd *RunCommand) constructAuthHandler(
	logger lager.Logger,
	storage storage.Storage,
) (http.Handler, error) {

	issuerPath, _ := url.Parse("/sky/issuer")
	redirectPath, _ := url.Parse("/sky/callback")

	issuerURL := cmd.ExternalURL.URL.ResolveReference(issuerPath)
	redirectURL := cmd.ExternalURL.URL.ResolveReference(redirectPath)

	// Add public fly client
	cmd.Auth.AuthFlags.Clients[flyClientID] = flyClientSecret

	dexServer, err := dexserver.NewDexServer(&dexserver.DexConfig{
		Logger:      logger.Session("dex"),
		Users:       cmd.Auth.AuthFlags.LocalUsers,
		Clients:     cmd.Auth.AuthFlags.Clients,
		Expiration:  cmd.Auth.AuthFlags.Expiration,
		IssuerURL:   issuerURL.String(),
		RedirectURL: redirectURL.String(),
		WebHostURL:  "/sky/issuer",
		SigningKey:  cmd.Auth.AuthFlags.SigningKey.PrivateKey,
		Storage:     storage,
	})
	if err != nil {
		return nil, err
	}

	return dexServer, nil
}

func (cmd *RunCommand) constructLoginHandler(
	logger lager.Logger,
	httpClient *http.Client,
	middleware token.Middleware,
) (http.Handler, error) {

	authPath, _ := url.Parse("/sky/issuer/auth")
	tokenPath, _ := url.Parse("/sky/issuer/token")
	redirectPath, _ := url.Parse("/sky/callback")

	authURL := cmd.ExternalURL.URL.ResolveReference(authPath)
	tokenURL := cmd.ExternalURL.URL.ResolveReference(tokenPath)
	redirectURL := cmd.ExternalURL.URL.ResolveReference(redirectPath)

	endpoint := oauth2.Endpoint{
		AuthURL:   authURL.String(),
		TokenURL:  tokenURL.String(),
		AuthStyle: oauth2.AuthStyleInHeader,
	}

	oauth2Config := &oauth2.Config{
		Endpoint:     endpoint,
		ClientID:     cmd.Server.ClientID,
		ClientSecret: cmd.Server.ClientSecret,
		RedirectURL:  redirectURL.String(),
		Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
	}

	skyServer, err := skyserver.NewSkyServer(&skyserver.SkyConfig{
		Logger:          logger.Session("sky"),
		TokenMiddleware: middleware,
		OAuthConfig:     oauth2Config,
		HTTPClient:      httpClient,
	})
	if err != nil {
		return nil, err
	}

	return skyserver.NewSkyHandler(skyServer), nil
}

func (cmd *RunCommand) constructTokenVerifier(httpClient *http.Client) accessor.TokenVerifier {

	publicKeyPath, _ := url.Parse("/sky/issuer/keys")
	publicKeyURL := cmd.ExternalURL.URL.ResolveReference(publicKeyPath)

	validClients := []string{flyClientID}
	for clientId, _ := range cmd.Auth.AuthFlags.Clients {
		validClients = append(validClients, clientId)
	}

	return accessor.NewVerifier(httpClient, publicKeyURL, validClients)
}

func (cmd *RunCommand) constructAPIHandler(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	teamFactory db.TeamFactory,
	dbPipelineFactory db.PipelineFactory,
	dbJobFactory db.JobFactory,
	dbResourceFactory db.ResourceFactory,
	dbWorkerFactory db.WorkerFactory,
	dbVolumeRepository db.VolumeRepository,
	dbContainerRepository db.ContainerRepository,
	gcContainerDestroyer gc.Destroyer,
	dbBuildFactory db.BuildFactory,
	dbCheckFactory db.CheckFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	dbUserFactory db.UserFactory,
	workerClient worker.Client,
	secretManager creds.Secrets,
	credsManagers creds.Managers,
	accessFactory accessor.AccessFactory,
	dbWall db.Wall,
	tokenVerifier accessor.TokenVerifier,
	notifications db.NotificationsBus,
	policyChecker *policy.Checker,
) (http.Handler, error) {

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(teamFactory)
	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)
	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)
	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	rejectArchivedHandlerFactory := pipelineserver.NewRejectArchivedHandlerFactory(teamFactory)

	aud := auditor.NewAuditor(
		cmd.Auditor.EnableBuildAuditLog,
		cmd.Auditor.EnableContainerAuditLog,
		cmd.Auditor.EnableJobAuditLog,
		cmd.Auditor.EnablePipelineAuditLog,
		cmd.Auditor.EnableResourceAuditLog,
		cmd.Auditor.EnableSystemAuditLog,
		cmd.Auditor.EnableTeamAuditLog,
		cmd.Auditor.EnableWorkerAuditLog,
		cmd.Auditor.EnableVolumeAuditLog,
		logger,
	)

	batcher := accessor.NewBatcher(
		logger,
		dbUserFactory,
		BatcherInterval,
		100,
	)

	cacher := accessor.NewCacher(
		logger,
		notifications,
		teamFactory,
		time.Minute,
		time.Minute,
	)

	customRoles, err := cmd.parseCustomRoles()
	if err != nil {
		return nil, err
	}

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewConcurrentRequestLimitsWrappa(
			logger,
			wrappa.NewConcurrentRequestPolicy(cmd.ConcurrentRequestLimits),
		),
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewPolicyCheckWrappa(logger, policychecker.NewApiPolicyChecker(policyChecker)),
		wrappa.NewAPIAuthWrappa(
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),
		wrappa.NewRejectArchivedWrappa(rejectArchivedHandlerFactory),
		wrappa.NewConcourseVersionWrappa(concourse.Version),
		wrappa.NewAccessorWrappa(
			logger,
			accessFactory,
			tokenVerifier,
			cacher,
			batcher,
			aud,
			customRoles,
		),
		wrappa.NewCompressionWrappa(logger),
	}

	return api.NewHandler(
		logger,
		cmd.ExternalURL.String(),
		cmd.Server.ClusterName,
		apiWrapper,

		teamFactory,
		dbPipelineFactory,
		dbJobFactory,
		dbResourceFactory,
		dbWorkerFactory,
		dbVolumeRepository,
		dbContainerRepository,
		gcContainerDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		resourceConfigFactory,
		dbUserFactory,

		buildserver.NewEventHandler,

		workerClient,

		reconfigurableSink,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		concourse.Version,
		concourse.WorkerVersion,
		secretManager,
		cmd.varSourcePool,
		credsManagers,
		containerserver.NewInterceptTimeoutFactory(cmd.InterceptIdleTimeout),
		time.Minute,
		dbWall,
		clock.NewClock(),

		cmd.EnableArchivePipeline,
	)
}

func parseAEAD(key string) (cipher.AEAD, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to construct AES cipher: %s", err)
	}

	AEAD, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to construct GCM: %s", err)
	}

	return AEAD, nil
}

type tlsRedirectHandler struct {
	matchHostname string
	externalHost  string
	baseHandler   http.Handler
}

func (h tlsRedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Host, h.matchHostname) && (r.Method == "GET" || r.Method == "HEAD") {
		u := url.URL{
			Scheme:   "https",
			Host:     h.externalHost,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}

		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
	} else {
		h.baseHandler.ServeHTTP(w, r)
	}
}

func (cmd *RunCommand) isTLSEnabled() bool {
	return cmd.TLSBindPort != 0
}

type drainRunner struct {
	logger  lager.Logger
	drainer component.Drainable
}

func (runner drainRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	<-signals
	runner.drainer.Drain(lagerctx.NewContext(context.Background(), runner.logger))
	return nil
}

type RunnableComponent struct {
	atc.Component
	component.Runnable
}
