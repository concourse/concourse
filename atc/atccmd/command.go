package atccmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/concourse/concourse"
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
	"github.com/concourse/concourse/atc/creds/kubernetes"
	"github.com/concourse/concourse/atc/creds/noop"
	"github.com/concourse/concourse/atc/creds/secretsmanager"
	"github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/engine"
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
	"github.com/concourse/concourse/skymarshal/dexserver"
	"github.com/concourse/concourse/skymarshal/legacyserver"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/skymarshal/token"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/web"
	"github.com/concourse/flag"
	"github.com/concourse/retryhttp"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/cppforlife/go-semi-semantic/version"
	gocache "github.com/patrickmn/go-cache"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v2"

	// dynamically registered metric emitters
	"github.com/concourse/concourse/atc/metric/emitter"
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

var retryingDriverName = "too-many-connections-retrying"

var flyClientID = "fly"
var flyClientSecret = "Zmx5"

type RunConfig struct {
	Logger        flag.Lager
	varSourcePool creds.VarSourcePool

	BindIP      net.IP    `yaml:"bind_ip,omitempty"`
	BindPort    uint16    `yaml:"bind_port,omitempty"`
	TLS         TLSConfig `yaml:"tls,omitempty"`
	ExternalURL flag.URL  `yaml:"external_url,omitempty" validate:"url"`

	Auth                      AuthConfig           `yaml:"auth,omitempty" ignore_env:"true"`
	Server                    ServerConfig         `yaml:"web_server,omitempty"`
	SystemClaim               SystemClaimConfig    `yaml:"system_claim,omitempty"`
	LetsEncrypt               LetsEncryptConfig    `yaml:"lets_encrypt,omitempty"`
	ConfigRBAC                flag.File            `yaml:"config_rbac,omitempty" validate:"rbac"`
	PolicyCheckers            PolicyCheckersConfig `yaml:"policy_check,omitempty"`
	DisplayUserIdPerConnector map[string]string    `yaml:"display_user_id_per_connector,omitempty" validate:"connectors"`

	Database DatabaseConfig `yaml:"database,omitempty" ignore_env:"true"`

	CredentialManagement creds.CredentialManagementConfig `yaml:"credential_management,omitempty"`
	CredentialManagers   CredentialManagersConfig         `yaml:"credential_managers,omitempty"`

	ComponentRunnerInterval time.Duration `yaml:"component_runner_interval,omitempty"`
	BuildTrackerInterval    time.Duration `yaml:"build_tracker_interval,omitempty"`

	ResourceChecking ResourceCheckingConfig `yaml:"resource_checking,omitempty"`
	JobScheduling    JobSchedulingConfig    `yaml:"job_scheduling,omitempty"`
	Runtime          RuntimeConfig          `yaml:"runtime,omitempty" ignore_env:"true"`

	GC                GCConfig                `yaml:"gc,omitempty"`
	BuildLogRetention BuildLogRetentionConfig `yaml:"build_log_retention,omitempty"`

	Debug   DebugConfig    `yaml:"debug,omitempty"`
	Log     LogConfig      `yaml:"log,omitempty"`
	Metrics MetricsConfig  `yaml:"metrics,omitempty"`
	Tracing tracing.Config `yaml:"tracing,omitempty"`
	Auditor AuditorConfig  `yaml:"auditing,omitempty"`
	Syslog  SyslogConfig   `yaml:"syslog,omitempty"`

	DefaultCpuLimit      *int          `yaml:"default_task_cpu_limit,omitempty"`
	DefaultMemoryLimit   *string       `yaml:"default_task_memory_limit,omitempty"`
	InterceptIdleTimeout time.Duration `yaml:"intercept_idle_timeout,omitempty"`

	CLIArtifactsDir flag.Dir `yaml:"cli_artifacts_dir,omitempty"`

	BaseResourceTypeDefaults flag.File `yaml:"base_resource_type_defaults,omitempty"`

	// NOT USED (HIDDEN)
	TelemetryOptIn bool `yaml:"telemetry_opt_in,omitempty"`

	FeatureFlags FeatureFlagsConfig `yaml:"feature_flags,omitempty" ignore_env:"true"`
}

var CmdDefaults RunConfig = RunConfig{
	Logger: flag.Lager{
		LogLevel: "info",
	},

	BindIP:   net.IPv4(0, 0, 0, 0),
	BindPort: 8080,

	TLS: TLSConfig{},
	Auth: AuthConfig{
		AuthFlags: skycmd.AuthFlags{
			Expiration: 24 * time.Hour,
			Connectors: skycmd.ConnectorsConfig{
				OAuth: skycmd.OAuthFlags{
					GroupsKey:   "groups",
					UserIDKey:   "user_id",
					UserNameKey: "user_name",
				},

				OIDC: skycmd.OIDCFlags{
					GroupsKey:   "groups",
					UserNameKey: "username",
				},

				SAML: skycmd.SAMLFlags{
					UsernameAttr: "name",
					EmailAttr:    "email",
					GroupsAttr:   "groups",
				},
			},
		},
	},

	Server: ServerConfig{
		XFrameOptions: "deny",
		ClientID:      "concourse-web",
	},

	SystemClaim: SystemClaimConfig{
		Key:    "aud",
		Values: []string{"concourse-worker"},
	},

	LetsEncrypt: LetsEncryptConfig{
		ACMEURL: ignoreErrParseURL("https://acme-v02.api.letsencrypt.org/directory"),
	},

	Database: DatabaseConfig{
		Postgres: flag.PostgresConfig{
			Host:           "127.0.0.1",
			Port:           5432,
			SSLMode:        "disable",
			ConnectTimeout: 5 * time.Minute,
			Database:       "atc",
		},
		APIMaxOpenConnections:     10,
		BackendMaxOpenConnections: 50,
	},

	CredentialManagement: creds.CredentialManagementConfig{
		RetryConfig: creds.SecretRetryConfig{
			Attempts: 5,
			Interval: 1 * time.Second,
		},

		CacheConfig: creds.SecretCacheConfig{
			Duration:         1 * time.Minute,
			DurationNotFound: 10 * time.Second,
			PurgeInterval:    10 * time.Minute,
		},
	},

	CredentialManagers: CredentialManagersConfig{
		Conjur: conjur.Manager{
			PipelineSecretTemplate: conjur.DefaultPipelineSecretTemplate,
			TeamSecretTemplate:     conjur.DefaultTeamSecretTemplate,
			SecretTemplate:         "vaultName/{{.Secret}}",
		},
		CredHub: credhub.CredHubManager{
			PathPrefix: "/concourse",
		},
		Kubernetes: kubernetes.KubernetesManager{
			NamespacePrefix: "concourse-",
		},
		SecretsManager: secretsmanager.Manager{
			PipelineSecretTemplate: "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
			TeamSecretTemplate:     "/concourse/{{.Team}}/{{.Secret}}",
		},
		SSM: ssm.SsmManager{
			PipelineSecretTemplate: "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
			TeamSecretTemplate:     "/concourse/{{.Team}}/{{.Secret}}",
		},
		Vault: vault.VaultManager{
			PathPrefix:      "/concourse",
			LookupTemplates: []string{"/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"},
			LoginTimeout:    60 * time.Second,
			QueryTimeout:    60 * time.Second,
			Auth: vault.AuthConfig{
				RetryMax:     5 * time.Minute,
				RetryInitial: 1 * time.Second,
			},
		},
	},

	ComponentRunnerInterval: 10 * time.Second,
	BuildTrackerInterval:    10 * time.Second,

	ResourceChecking: ResourceCheckingConfig{
		ScannerInterval:            10 * time.Second,
		Timeout:                    1 * time.Hour,
		DefaultInterval:            1 * time.Minute,
		DefaultIntervalWithWebhook: 1 * time.Minute,
	},

	JobScheduling: JobSchedulingConfig{
		MaxInFlight: 32,
	},

	Runtime: RuntimeConfig{
		ContainerPlacementStrategyOptions: worker.ContainerPlacementStrategyOptions{
			ContainerPlacementStrategy:   []string{"volume-locality"},
			MaxActiveTasksPerWorker:      0,
			MaxActiveContainersPerWorker: 0,
			MaxActiveVolumesPerWorker:    0,
		},
		BaggageclaimResponseHeaderTimeout: 1 * time.Minute,
		StreamingArtifactsCompression:     "gzip",
		GardenRequestTimeout:              5 * time.Minute,
		P2pVolumeStreamingTimeout:         15 * time.Minute,
	},

	GC: GCConfig{
		Interval:               30 * time.Second,
		OneOffBuildGracePeriod: 5 * time.Minute,
		MissingGracePeriod:     5 * time.Minute,
		HijackGracePeriod:      5 * time.Minute,
		FailedGracePeriod:      120 * time.Hour,
		CheckRecyclePeriod:     1 * time.Minute,
		VarSourceRecyclePeriod: 5 * time.Minute,
	},

	Debug: DebugConfig{
		BindIP:   net.IPv4(127, 0, 0, 1),
		BindPort: 8079,
	},

	Metrics: MetricsConfig{
		BufferSize: 1000,

		Emitter: MetricsEmitterConfig{
			InfluxDB: emitter.InfluxDBConfig{
				BatchSize:     5000,
				BatchDuration: 300 * time.Second,
			},
			NewRelic: emitter.NewRelicConfig{
				Url:           "https://insights-collector.newrelic.com",
				ServicePrefix: "",
				BatchSize:     2000,
				BatchDuration: 60 * time.Second,
			},
		},
	},

	Tracing: tracing.Config{
		ServiceName: "concourse-web",

		Honeycomb: tracing.Honeycomb{
			ServiceName: "concourse",
		},

		Jaeger: tracing.Jaeger{
			Service: "web",
		},
	},

	Syslog: SyslogConfig{
		Hostname:      "atc-syslog-drainer",
		DrainInterval: 30 * time.Second,
	},

	InterceptIdleTimeout: 0 * time.Minute,
}

func (cmd *RunConfig) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *RunConfig) Runner(positionalArguments []string) (ifrit.Runner, error) {
	if cmd.ExternalURL.URL == nil {
		cmd.ExternalURL = cmd.DefaultURL()
	}

	if len(positionalArguments) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments: %v", positionalArguments)
	}

	logger, reconfigurableSink := cmd.Logger.Logger("atc")
	if cmd.Log.ClusterName {
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

	atc.EnableGlobalResources = cmd.FeatureFlags.EnableGlobalResources
	atc.EnableRedactSecrets = cmd.FeatureFlags.EnableRedactSecrets
	atc.EnableBuildRerunWhenWorkerDisappears = cmd.FeatureFlags.EnableBuildRerunWhenWorkerDisappears
	atc.EnableAcrossStep = cmd.FeatureFlags.EnableAcrossStep
	atc.EnablePipelineInstances = cmd.FeatureFlags.EnablePipelineInstances

	if cmd.BaseResourceTypeDefaults.Path() != "" {
		content, err := ioutil.ReadFile(cmd.BaseResourceTypeDefaults.Path())
		if err != nil {
			return nil, err
		}

		defaults := map[string]atc.Source{}
		err = yaml.Unmarshal(content, &defaults)
		if err != nil {
			return nil, err
		}

		atc.LoadBaseResourceTypeDefaults(defaults)
	}

	//FIXME: These only need to run once for the entire binary. At the moment,
	//they rely on state of the command.
	db.SetupConnectionRetryingDriver(
		"postgres",
		cmd.Database.Postgres.ConnectionString(),
		retryingDriverName,
	)

	// Register the sink that collects error metrics
	if cmd.Metrics.CaptureErrorMetrics {
		errorSinkCollector := metric.NewErrorSinkCollector(
			logger,
			metric.Metrics,
		)
		logger.RegisterSink(&errorSinkCollector)
	}

	err := cmd.Tracing.Prepare()
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

	lockConn, err := lock.ConstructLockConn(retryingDriverName, cmd.Database.Postgres.ConnectionString())
	if err != nil {
		return nil, err
	}

	lockFactory := lock.NewLockFactory(lockConn, metric.LogLockAcquired, metric.LogLockReleased)

	apiConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.Database.APIMaxOpenConnections, cmd.Database.APIMaxOpenConnections/2, "api", lockFactory)
	if err != nil {
		return nil, err
	}

	backendConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.Database.BackendMaxOpenConnections, cmd.Database.BackendMaxOpenConnections/2, "backend", lockFactory)
	if err != nil {
		return nil, err
	}

	gcConn, err := cmd.constructDBConn(retryingDriverName, logger, 5, 2, "gc", lockFactory)
	if err != nil {
		return nil, err
	}

	workerConn, err := cmd.constructDBConn(retryingDriverName, logger, 1, 1, "worker", lockFactory)
	if err != nil {
		return nil, err
	}

	storage, err := storage.NewPostgresStorage(logger, cmd.Database.Postgres)
	if err != nil {
		return nil, err
	}

	secretManager, err := cmd.secretManager(logger)
	if err != nil {
		return nil, err
	}

	cmd.varSourcePool = creds.NewVarSourcePool(
		logger.Session("var-source-pool"),
		cmd.CredentialManagement,
		cmd.GC.VarSourceRecyclePeriod,
		1*time.Minute,
		clock.NewClock(),
	)

	members, err := cmd.constructMembers(logger, reconfigurableSink, apiConn, workerConn, backendConn, gcConn, storage, lockFactory, secretManager)
	if err != nil {
		return nil, err
	}

	members = append(members, grouper.Member{
		Name: "periodic-metrics",
		Runner: metric.PeriodicallyEmit(
			logger.Session("periodic-metrics"),
			metric.Metrics,
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
		for _, closer := range []Closer{lockConn, apiConn, backendConn, gcConn, storage, workerConn} {
			closer.Close()
		}

		cmd.varSourcePool.Close()
	}

	return run(grouper.NewParallel(os.Interrupt, members), onReady, onExit), nil
}

func (cmd *RunConfig) constructMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	apiConn db.Conn,
	workerConn db.Conn,
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

	apiMembers, err := cmd.constructAPIMembers(logger, reconfigurableSink, apiConn, workerConn, storage, lockFactory, secretManager, policyChecker)
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

func (cmd *RunConfig) constructAPIMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	dbConn db.Conn,
	workerConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
	policyChecker policy.Checker,
) ([]grouper.Member, error) {

	httpClient, err := cmd.skyHttpClient()
	if err != nil {
		return nil, err
	}

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)
	workerTeamFactory := db.NewTeamFactory(workerConn, lockFactory)

	_, err = teamFactory.CreateDefaultTeamIfNotExists()
	if err != nil {
		return nil, err
	}

	err = cmd.configureAuthForDefaultTeam(teamFactory)
	if err != nil {
		return nil, err
	}

	userFactory := db.NewUserFactory(dbConn)

	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := worker.NewFetchSourceFactory(dbResourceCacheFactory)
	resourceFetcher := worker.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(workerConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}

	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.Runtime.BaggageclaimResponseHeaderTimeout,
		cmd.Runtime.GardenRequestTimeout,
	)

	pool := worker.NewPool(workerProvider)

	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbResourceFactory := db.NewResourceFactory(dbConn, lockFactory)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	gcContainerDestroyer := gc.NewDestroyer(logger, dbContainerRepository, dbVolumeRepository)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, db.CheckDurations{
		Interval:            cmd.ResourceChecking.DefaultInterval,
		IntervalWithWebhook: cmd.ResourceChecking.DefaultIntervalWithWebhook,
		Timeout:             cmd.ResourceChecking.Timeout,
	})
	dbAccessTokenFactory := db.NewAccessTokenFactory(dbConn)
	dbClock := db.NewClock()
	dbWall := db.NewWall(dbConn, &dbClock)

	tokenVerifier := cmd.constructTokenVerifier(dbAccessTokenFactory)

	teamsCacher := accessor.NewTeamsCacher(
		logger,
		dbConn.Bus(),
		teamFactory,
		time.Minute,
		time.Minute,
	)

	displayUserIdGenerator := skycmd.NewSkyDisplayUserIdGenerator(cmd.DisplayUserIdPerConnector)

	accessFactory := accessor.NewAccessFactory(
		tokenVerifier,
		teamsCacher,
		cmd.SystemClaim.Key,
		cmd.SystemClaim.Values,
		displayUserIdGenerator,
	)

	middleware := token.NewMiddleware(cmd.Auth.AuthFlags.SecureCookies)

	credsManager, err := cmd.CredentialManagers.ConfiguredCredentialManager()
	if err != nil {
		return nil, err
	}

	apiHandler, err := cmd.constructAPIHandler(
		logger,
		reconfigurableSink,
		teamFactory,
		workerTeamFactory,
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
		pool,
		secretManager,
		credsManager,
		accessFactory,
		dbWall,
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
		dbAccessTokenFactory,
		userFactory,
		displayUserIdGenerator,
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

func (cmd *RunConfig) backendComponents(
	logger lager.Logger,
	dbConn db.Conn,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
	policyChecker policy.Checker,
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

	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, db.CheckDurations{
		Interval:            cmd.ResourceChecking.DefaultInterval,
		IntervalWithWebhook: cmd.ResourceChecking.DefaultIntervalWithWebhook,
		Timeout:             cmd.ResourceChecking.Timeout,
	})
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbPipelineLifecycle := db.NewPipelineLifecycle(dbConn, lockFactory)

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
	if cmd.Runtime.StreamingArtifactsCompression == "zstd" {
		compressionLib = compression.NewZstdCompression()
	} else {
		compressionLib = compression.NewGzipCompression()
	}
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbTaskCacheFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.Runtime.BaggageclaimResponseHeaderTimeout,
		cmd.Runtime.GardenRequestTimeout,
	)

	pool := worker.NewPool(workerProvider)
	artifactStreamer := worker.NewArtifactStreamer(pool, compressionLib)
	artifactSourcer := worker.NewArtifactSourcer(compressionLib, pool, cmd.FeatureFlags.EnableP2PVolumeStreaming, cmd.Runtime.P2pVolumeStreamingTimeout)

	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	buildContainerStrategy, err := cmd.chooseBuildContainerStrategy()
	if err != nil {
		return nil, err
	}

	rateLimiter := db.NewResourceCheckRateLimiter(
		rate.Limit(cmd.ResourceChecking.MaxChecksPerSecond),
		cmd.ResourceChecking.DefaultInterval,
		dbConn,
		time.Minute,
		clock.NewClock(),
	)

	engine := cmd.constructEngine(
		pool,
		artifactStreamer,
		artifactSourcer,
		resourceFactory,
		dbWorkerFactory,
		teamFactory,
		dbBuildFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		secretManager,
		defaultLimits,
		buildContainerStrategy,
		lockFactory,
		rateLimiter,
		policyChecker,
	)

	// In case that a user configures resource-checking-interval, but forgets to
	// configure resource-with-webhook-checking-interval, keep both checking-
	// intervals consistent. Even if both intervals are configured, there is no
	// reason webhooked resources take shorter checking interval than normal
	// resources.
	if cmd.ResourceChecking.DefaultIntervalWithWebhook < cmd.ResourceChecking.DefaultInterval {
		logger.Info("update-resource-with-webhook-checking-interval",
			lager.Data{
				"oldValue": cmd.ResourceChecking.DefaultIntervalWithWebhook,
				"newValue": cmd.ResourceChecking.DefaultInterval,
			})
		cmd.ResourceChecking.DefaultIntervalWithWebhook = cmd.ResourceChecking.DefaultInterval
	}

	components := []RunnableComponent{
		{
			Component: atc.Component{
				Name:     atc.ComponentLidarScanner,
				Interval: cmd.ResourceChecking.ScannerInterval,
			},
			Runnable: lidar.NewScanner(dbCheckFactory),
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
				cmd.JobScheduling.MaxInFlight,
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
				dbPipelineLifecycle,
				500,
				gc.NewBuildLogRetentionCalculator(
					cmd.BuildLogRetention.Default,
					cmd.BuildLogRetention.Max,
					cmd.BuildLogRetention.DefaultDays,
					cmd.BuildLogRetention.MaxDays,
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

func (cmd *RunConfig) gcComponents(
	logger lager.Logger,
	gcConn db.Conn,
	lockFactory lock.LockFactory,
) ([]RunnableComponent, error) {
	dbWorkerLifecycle := db.NewWorkerLifecycle(gcConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(gcConn)
	dbContainerRepository := db.NewContainerRepository(gcConn)
	dbArtifactLifecycle := db.NewArtifactLifecycle(gcConn)
	dbAccessTokenLifecycle := db.NewAccessTokenLifecycle(gcConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(gcConn)
	dbBuildFactory := db.NewBuildFactory(gcConn, lockFactory, cmd.GC.OneOffBuildGracePeriod, cmd.GC.FailedGracePeriod)
	dbResourceConfigFactory := db.NewResourceConfigFactory(gcConn, lockFactory)
	dbPipelineLifecycle := db.NewPipelineLifecycle(gcConn, lockFactory)
	dbCheckLifecycle := db.NewCheckLifecycle(gcConn)

	dbVolumeRepository := db.NewVolumeRepository(gcConn)

	// set the 'unreferenced resource config' grace period to be the longer than
	// the check timeout, just to make sure it doesn't get removed out from under
	// a running check
	//
	// 5 minutes is arbitrary - this really shouldn't matter a whole lot, but
	// exposing a config specifically for it is a little risky, since you don't
	// want to set it too low.
	unreferencedConfigGracePeriod := cmd.ResourceChecking.Timeout + 5*time.Minute

	collectors := map[string]component.Runnable{
		atc.ComponentCollectorBuilds:            gc.NewBuildCollector(dbBuildFactory),
		atc.ComponentCollectorWorkers:           gc.NewWorkerCollector(dbWorkerLifecycle),
		atc.ComponentCollectorResourceConfigs:   gc.NewResourceConfigCollector(dbResourceConfigFactory, unreferencedConfigGracePeriod),
		atc.ComponentCollectorResourceCaches:    gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorResourceCacheUses: gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorArtifacts:         gc.NewArtifactCollector(dbArtifactLifecycle),
		atc.ComponentCollectorVolumes:           gc.NewVolumeCollector(dbVolumeRepository, cmd.GC.MissingGracePeriod),
		atc.ComponentCollectorContainers:        gc.NewContainerCollector(dbContainerRepository, cmd.GC.MissingGracePeriod, cmd.GC.HijackGracePeriod),
		atc.ComponentCollectorCheckSessions:     gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle),
		atc.ComponentCollectorPipelines:         gc.NewPipelineCollector(dbPipelineLifecycle),
		atc.ComponentCollectorAccessTokens:      gc.NewAccessTokensCollector(dbAccessTokenLifecycle, jwt.DefaultLeeway),
		atc.ComponentCollectorChecks:            gc.NewChecksCollector(dbCheckLifecycle),
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

func (cmd *RunConfig) parseCustomRoles() (map[string]string, error) {
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

func (cmd *RunConfig) secretManager(logger lager.Logger) (creds.Secrets, error) {
	var secretsFactory creds.SecretsFactory = noop.NewNoopFactory()
	manager, err := cmd.CredentialManagers.ConfiguredCredentialManager()
	if err != nil {
		return nil, err
	}

	if manager == nil {
		return nil, nil
	}

	credsLogger := logger.Session("credential-manager", lager.Data{
		"name": manager.Name(),
	})

	credsLogger.Info("configured credentials manager")

	err = manager.Init(credsLogger)
	if err != nil {
		return nil, err
	}

	err = manager.Validate()
	if err != nil {
		return nil, fmt.Errorf("credential manager '%s' misconfigured: %s", manager.Name(), err)
	}

	secretsFactory, err = manager.NewSecretsFactory(credsLogger)
	if err != nil {
		return nil, err
	}

	return cmd.CredentialManagement.NewSecrets(secretsFactory), nil
}

func (cmd *RunConfig) newKey() *encryption.Key {
	var newKey *encryption.Key
	if cmd.Database.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.Database.EncryptionKey.AEAD)
	}

	return newKey
}

func (cmd *RunConfig) oldKey() *encryption.Key {
	var oldKey *encryption.Key
	if cmd.Database.OldEncryptionKey.AEAD != nil {
		oldKey = encryption.NewKey(cmd.Database.OldEncryptionKey.AEAD)
	}

	return oldKey
}

func (cmd *RunConfig) constructWebHandler(logger lager.Logger) (http.Handler, error) {
	webHandler, err := web.NewHandler(logger)
	if err != nil {
		return nil, err
	}
	return metric.WrapHandler(logger, metric.Metrics, "web", webHandler), nil
}

func (cmd *RunConfig) skyHttpClient() (*http.Client, error) {
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

			cert, err := tls.LoadX509KeyPair(string(cmd.TLS.Cert), string(cmd.TLS.Key))
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

func (cmd *RunConfig) tlsConfig(logger lager.Logger, dbConn db.Conn) (*tls.Config, error) {
	var tlsConfig *tls.Config
	tlsConfig = atc.DefaultTLSConfig()

	if cmd.isTLSEnabled() {
		tlsLogger := logger.Session("tls-enabled")

		if cmd.isMTLSEnabled() {
			tlsLogger.Debug("mTLS-Enabled")
			clientCACert, err := ioutil.ReadFile(string(cmd.TLS.CaCert))
			if err != nil {
				return nil, err
			}
			clientCertPool := x509.NewCertPool()
			clientCertPool.AppendCertsFromPEM(clientCACert)

			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			tlsConfig.ClientCAs = clientCertPool
		}

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
			cert, err := tls.LoadX509KeyPair(string(cmd.TLS.Cert), string(cmd.TLS.Key))
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}
	return tlsConfig, nil
}

func (cmd *RunConfig) parseDefaultLimits() (atc.ContainerLimits, error) {
	limits := atc.ContainerLimits{}
	if cmd.DefaultCpuLimit != nil {
		cpu := atc.CPULimit(*cmd.DefaultCpuLimit)
		limits.CPU = &cpu
	}
	if cmd.DefaultMemoryLimit != nil {
		memory, err := atc.ParseMemoryLimit(*cmd.DefaultMemoryLimit)
		if err != nil {
			return atc.ContainerLimits{}, err
		}
		limits.Memory = &memory
	}
	return limits, nil
}

func (cmd *RunConfig) defaultBindIP() net.IP {
	URL := cmd.BindIP.String()
	if URL == "0.0.0.0" {
		URL = "127.0.0.1"
	}

	return net.ParseIP(URL)
}

func (cmd *RunConfig) DefaultURL() flag.URL {
	return flag.URL{
		URL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", cmd.defaultBindIP().String(), cmd.BindPort),
		},
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

func (cmd *RunConfig) nonTLSBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
}

func (cmd *RunConfig) tlsBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.TLS.BindPort)
}

func (cmd *RunConfig) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.Debug.BindIP, cmd.Debug.BindPort)
}

func (cmd *RunConfig) configureMetrics(logger lager.Logger) error {
	host := cmd.Metrics.HostName
	if host == "" {
		host, _ = os.Hostname()
	}

	configuredEmitter, err := cmd.Metrics.Emitter.ConfiguredEmitter()
	if err != nil {
		return err
	}

	if configuredEmitter != nil {
		err = configuredEmitter.Validate()
		if err != nil {
			return fmt.Errorf("validate emitter %s: %w", configuredEmitter.Description(), err)
		}

		err = metric.Metrics.Initialize(logger.Session("metrics"), configuredEmitter, host, cmd.Metrics.Attributes, cmd.Metrics.BufferSize)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *RunConfig) constructDBConn(
	driverName string,
	logger lager.Logger,
	maxConns int,
	idleConns int,
	connectionName string,
	lockFactory lock.LockFactory,
) (db.Conn, error) {
	dbConn, err := db.Open(logger.Session("db"), driverName, cmd.Database.Postgres.ConnectionString(), cmd.newKey(), cmd.oldKey(), connectionName, lockFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %s", err)
	}

	// Instrument with Metrics
	dbConn = metric.CountQueries(dbConn)
	metric.Metrics.Databases = append(metric.Metrics.Databases, dbConn)

	// Instrument with Logging
	if cmd.Log.DBQueries {
		dbConn = db.Log(logger.Session("log-conn"), dbConn)
	}

	// Prepare
	dbConn.SetMaxOpenConns(maxConns)
	dbConn.SetMaxIdleConns(idleConns)

	return dbConn, nil
}

type Closer interface {
	Close() error
}

func (cmd *RunConfig) chooseBuildContainerStrategy() (worker.ContainerPlacementStrategy, error) {
	return worker.NewContainerPlacementStrategy(cmd.Runtime.ContainerPlacementStrategyOptions)
}

func (cmd *RunConfig) configureAuthForDefaultTeam(teamFactory db.TeamFactory) error {
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

func (cmd *RunConfig) constructEngine(
	workerPool worker.Pool,
	artifactStreamer worker.ArtifactStreamer,
	artifactSourcer worker.ArtifactSourcer,
	resourceFactory resource.ResourceFactory,
	workerFactory db.WorkerFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	secretManager creds.Secrets,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
	rateLimiter engine.RateLimiter,
	policyChecker policy.Checker,
) engine.Engine {
	return engine.NewEngine(
		engine.NewStepperFactory(
			engine.NewCoreStepFactory(
				workerPool,
				artifactStreamer,
				artifactSourcer,
				resourceFactory,
				teamFactory,
				buildFactory,
				resourceCacheFactory,
				resourceConfigFactory,
				defaultLimits,
				strategy,
				cmd.ResourceChecking.Timeout,
			),
			cmd.ExternalURL.String(),
			rateLimiter,
			policyChecker,
			artifactSourcer,
			workerFactory,
			lockFactory,
		),
		secretManager,
		cmd.varSourcePool,
	)
}

func (cmd *RunConfig) constructHTTPHandler(
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

func (cmd *RunConfig) constructLegacyHandler(
	logger lager.Logger,
) (http.Handler, error) {
	return legacyserver.NewLegacyServer(&legacyserver.LegacyConfig{
		Logger: logger.Session("legacy"),
	})
}

func (cmd *RunConfig) constructAuthHandler(
	logger lager.Logger,
	storage storage.Storage,
	accessTokenFactory db.AccessTokenFactory,
	userFactory db.UserFactory,
	displayUserIdGenerator atc.DisplayUserIdGenerator,
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
		Connectors:  cmd.Auth.AuthFlags.Connectors,
	})
	if err != nil {
		return nil, err
	}

	return token.StoreAccessToken(
		logger.Session("dex-server"),
		dexServer,
		token.Factory{},
		token.NewClaimsParser(),
		accessTokenFactory,
		userFactory,
		displayUserIdGenerator,
	), nil
}

func (cmd *RunConfig) constructLoginHandler(
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
		TokenParser:     token.Factory{},
		OAuthConfig:     oauth2Config,
		HTTPClient:      httpClient,
	})
	if err != nil {
		return nil, err
	}

	return skyserver.NewSkyHandler(skyServer), nil
}

func (cmd *RunConfig) constructTokenVerifier(accessTokenFactory db.AccessTokenFactory) accessor.TokenVerifier {

	validClients := []string{flyClientID}
	for clientId := range cmd.Auth.AuthFlags.Clients {
		validClients = append(validClients, clientId)
	}

	MiB := 1024 * 1024
	claimsCacher := accessor.NewClaimsCacher(accessTokenFactory, 1*MiB)

	return accessor.NewVerifier(claimsCacher, validClients)
}

func (cmd *RunConfig) constructAPIHandler(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	teamFactory db.TeamFactory,
	workerTeamFactory db.TeamFactory,
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
	workerPool worker.Pool,
	secretManager creds.Secrets,
	credsManager creds.Manager,
	accessFactory accessor.AccessFactory,
	dbWall db.Wall,
	policyChecker policy.Checker,
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

	customRoles, err := cmd.parseCustomRoles()
	if err != nil {
		return nil, err
	}

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewConcurrentRequestLimitsWrappa(
			logger,
			wrappa.NewConcurrentRequestPolicy(cmd.Database.ConcurrentRequestLimits),
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
		workerTeamFactory,
		dbVolumeRepository,
		dbContainerRepository,
		gcContainerDestroyer,
		dbBuildFactory,
		dbCheckFactory,
		resourceConfigFactory,
		dbUserFactory,

		buildserver.NewEventHandler,

		workerPool,

		reconfigurableSink,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		concourse.Version,
		concourse.WorkerVersion,
		secretManager,
		cmd.varSourcePool,
		credsManager,
		containerserver.NewInterceptTimeoutFactory(cmd.InterceptIdleTimeout),
		time.Minute,
		dbWall,
		clock.NewClock(),
	)
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

func (cmd *RunConfig) isTLSEnabled() bool {
	return cmd.TLS.BindPort != 0
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

func (cmd *RunConfig) isMTLSEnabled() bool {
	return string(cmd.TLS.CaCert) != ""
}

func ignoreErrParseURL(urlString string) flag.URL {
	parsedURL, _ := url.Parse(urlString)
	return flag.URL{parsedURL}
}
