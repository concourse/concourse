package atccmd

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/containerserver"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/creds/noop"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/encryption"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/gc"
	"github.com/concourse/atc/lockrunner"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/syslog"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/image"
	"github.com/concourse/atc/wrappa"
	"github.com/concourse/flag"
	"github.com/concourse/retryhttp"
	"github.com/concourse/skymarshal"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/concourse/web"
	"github.com/cppforlife/go-semi-semantic/version"
	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	// dynamically registered metric emitters
	_ "github.com/concourse/atc/metric/emitter"

	// dynamically registered credential managers
	_ "github.com/concourse/atc/creds/credhub"
	_ "github.com/concourse/atc/creds/kubernetes"
	_ "github.com/concourse/atc/creds/secretsmanager"
	_ "github.com/concourse/atc/creds/ssm"
	_ "github.com/concourse/atc/creds/vault"
)

var defaultDriverName = "postgres"
var retryingDriverName = "too-many-connections-retrying"

type ATCCommand struct {
	RunCommand RunCommand `command:"run"`
	Migration  Migration  `command:"migrate"`
}

type RunCommand struct {
	Logger flag.Lager

	BindIP   flag.IP `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16  `long:"bind-port" default:"8080"    description:"Port on which to listen for HTTP traffic."`

	TLSBindPort uint16    `long:"tls-bind-port" description:"Port on which to listen for HTTPS traffic."`
	TLSCert     flag.File `long:"tls-cert"      description:"File containing an SSL certificate."`
	TLSKey      flag.File `long:"tls-key"       description:"File containing an RSA private key, used to encrypt HTTPS traffic."`

	ExternalURL flag.URL `long:"external-url" description:"URL used to reach any ATC from the outside world."`
	PeerURL     flag.URL `long:"peer-url"     description:"URL used to reach this ATC from other ATCs in the cluster."`

	Postgres flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`

	CredentialManagement struct{} `group:"Credential Management"`
	CredentialManagers   creds.Managers

	EncryptionKey    flag.Cipher `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	OldEncryptionKey flag.Cipher `long:"old-encryption-key" description:"Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted."`

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	InterceptIdleTimeout time.Duration `long:"intercept-idle-timeout" default:"0m" description:"Length of time for a intercepted session to be idle before terminating."`

	GlobalResourceCheckTimeout   time.Duration `long:"global-resource-check-timeout" default:"1h" description:"Time limit on checking for new versions of resources."`
	ResourceCheckingInterval     time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	ResourceTypeCheckingInterval time.Duration `long:"resource-type-checking-interval" default:"1m" description:"Interval on which to check for new versions of resource types."`

	ContainerPlacementStrategy        string        `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" description:"Method by which a worker is selected during container placement."`
	BaggageclaimResponseHeaderTimeout time.Duration `long:"baggageclaim-response-header-timeout" default:"1m" description:"How long to wait for Baggageclaim to send the response header."`

	CLIArtifactsDir flag.Dir `long:"cli-artifacts-dir" description:"Directory containing downloadable CLI binaries."`

	Developer struct {
		Noop bool `short:"n" long:"noop"              description:"Don't actually do any automatic scheduling or checking."`
	} `group:"Developer Options"`

	Worker struct {
		GardenURL       flag.URL          `long:"garden-url"       description:"A Garden API endpoint to register as a worker."`
		BaggageclaimURL flag.URL          `long:"baggageclaim-url" description:"A Baggageclaim API endpoint to register with the worker."`
		ResourceTypes   map[string]string `long:"resource"         description:"A resource type to advertise for the worker. Can be specified multiple times." value-name:"TYPE:IMAGE"`
	} `group:"Static Worker (optional)" namespace:"worker"`

	Metrics struct {
		HostName   string            `long:"metrics-host-name"   description:"Host string to attach to emitted metrics."`
		Attributes map[string]string `long:"metrics-attribute"   description:"A key-value attribute to attach to emitted metrics. Can be specified multiple times." value-name:"NAME:VALUE"`
	} `group:"Metrics & Diagnostics"`

	Server struct {
		XFrameOptions string `long:"x-frame-options" description:"The value to set for X-Frame-Options. If omitted, the header is not set."`
	} `group:"Web Server"`

	LogDBQueries bool `long:"log-db-queries" description:"Log database queries."`

	GC struct {
		Interval               time.Duration `long:"interval" default:"30s" description:"Interval on which to perform garbage collection."`
		OneOffBuildGracePeriod time.Duration `long:"one-off-grace-period" default:"5m" description:"Grace period before reaping one-off task containers"`
	} `group:"Garbage Collection" namespace:"gc"`

	BuildTrackerInterval time.Duration `long:"build-tracker-interval" default:"10s" description:"Interval on which to run build tracking."`

	TelemetryOptIn bool `long:"telemetry-opt-in" hidden:"true" description:"Enable anonymous concourse version reporting."`

	DefaultBuildLogsToRetain uint64 `long:"default-build-logs-to-retain" description:"Default build logs to retain, 0 means all"`
	MaxBuildLogsToRetain     uint64 `long:"max-build-logs-to-retain" description:"Maximum build logs to retain, 0 means not specified. Will override values configured in jobs"`

	DefaultCpuLimit    *int    `long:"default-task-cpu-limit" description:"Default max number of cpu shares per task, 0 means unlimited"`
	DefaultMemoryLimit *string `long:"default-task-memory-limit" description:"Default maximum memory per task, 0 means unlimited"`

	Syslog struct {
		Hostname      string        `long:"syslog-hostname" description:"Client hostname with which the build logs will be sent to the syslog server." default:"atc-syslog-drainer"`
		Address       string        `long:"syslog-address" description:"Remote syslog server address with port (Example: 0.0.0.0:514)."`
		Transport     string        `long:"syslog-transport" description:"Transport protocol for syslog messages (Currently supporting tcp, udp & tls)."`
		DrainInterval time.Duration `long:"syslog-drain-interval" description:"Interval over which checking is done for new build logs to send to syslog server (duration measurement units are s/m/h; eg. 30s/30m/1h)" default:"30s"`
		CACerts       []string      `long:"syslog-ca-cert"              description:"Paths to PEM-encoded CA cert files to use to verify the Syslog server SSL cert."`
	} ` group:"Syslog Drainer Configuration"`

	Auth struct {
		AuthFlags     skycmd.AuthFlags
		MainTeamFlags skycmd.AuthTeamFlags `group:"Authentication (Main Team)" namespace:"main-team"`
	} `group:"Authentication"`
}

func (cmd *RunCommand) PeerURLOrDefault() flag.URL {
	if cmd.PeerURL.URL == nil {
		cmd.PeerURL = cmd.defaultURL()
	}
	return cmd.PeerURL
}

var HelpError = errors.New("must specify one of `--current-db-version`, `--supported-db-version`, or `--migrate-db-to-version`")

type Migration struct {
	Postgres           flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`
	EncryptionKey      flag.Cipher         `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	CurrentDBVersion   bool                `long:"current-db-version" description:"Print the current database version and exit"`
	SupportedDBVersion bool                `long:"supported-db-version" description:"Print the max supported database version and exit"`
	MigrateDBToVersion int                 `long:"migrate-db-to-version" description:"Migrate to the specified database version and exit"`
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
	return HelpError
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

	var newKey *encryption.Key
	if cmd.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.EncryptionKey.AEAD)
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
		return errors.New(fmt.Sprintf("Could not migrate to version: %d Reason: %s", version, err.Error()))
	}

	fmt.Println("Successfully migrated to version:", version)
	return nil
}

func (cmd *ATCCommand) WireDynamicFlags(commandFlags *flags.Command) {
	cmd.RunCommand.WireDynamicFlags(commandFlags)
}

func (cmd *RunCommand) WireDynamicFlags(commandFlags *flags.Command) {
	var metricsGroup *flags.Group
	var credsGroup *flags.Group
	var authGroup *flags.Group

	groups := commandFlags.Groups()
	for i := 0; i < len(groups); i++ {
		group := groups[i]

		if credsGroup == nil && group.ShortDescription == "Credential Management" {
			credsGroup = group
		}

		if metricsGroup == nil && group.ShortDescription == "Metrics & Diagnostics" {
			metricsGroup = group
		}

		if authGroup == nil && group.ShortDescription == "Authentication" {
			authGroup = group
		}

		if metricsGroup != nil && credsGroup != nil && authGroup != nil {
			break
		}

		groups = append(groups, group.Groups()...)
	}

	if metricsGroup == nil {
		panic("could not find Metrics & Diagnostics group for registering emitters")
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

	skycmd.WireConnectors(authGroup)
	skycmd.WireTeamConnectors(authGroup.Find("Authentication (Main Team)"))
}

func (cmd *RunCommand) Execute(args []string) error {
	runner, _, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *RunCommand) Runner(positionalArguments []string) (ifrit.Runner, bool, error) {
	if cmd.ExternalURL.URL == nil {
		cmd.ExternalURL = cmd.defaultURL()
	}

	radar.GlobalResourceCheckTimeout = cmd.GlobalResourceCheckTimeout
	//FIXME: These only need to run once for the entire binary. At the moment,
	//they rely on state of the command.
	db.SetupConnectionRetryingDriver("postgres", cmd.Postgres.ConnectionString(), retryingDriverName)
	logger, reconfigurableSink := cmd.Logger.Logger("atc")

	http.HandleFunc("/debug/connections", func(w http.ResponseWriter, r *http.Request) {
		for _, stack := range db.GlobalConnectionTracker.Current() {
			fmt.Fprintln(w, stack)
		}
	})

	if err := cmd.configureMetrics(logger); err != nil {
		return nil, false, err
	}
	go metric.PeriodicallyEmit(logger.Session("periodic-metrics"), 10*time.Second)

	members, err := cmd.constructMembers(positionalArguments, logger, reconfigurableSink)
	if err != nil {
		return nil, false, err
	}

	return onReady(grouper.NewParallel(os.Interrupt, members), func() {
		logData := lager.Data{
			"http":  cmd.nonTLSBindAddr(),
			"debug": cmd.debugBindAddr(),
		}

		if cmd.isTLSEnabled() {
			logData["https"] = cmd.tlsBindAddr()
		}

		logger.Info("listening", logData)
	}), false, nil
}

func (cmd *RunCommand) constructMembers(
	positionalArguments []string,
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
) ([]grouper.Member, error) {

	if len(positionalArguments) != 0 {
		return nil, fmt.Errorf("unexpected positional arguments: %v", positionalArguments)
	}

	err := cmd.validate()
	if err != nil {
		return nil, err
	}
	if cmd.TelemetryOptIn {
		url := fmt.Sprintf("http://telemetry.concourse-ci.org/?version=%s", Version)
		go func() {
			_, err := http.Get(url)
			if err != nil {
				logger.Error("telemetry-version", err)
			}
		}()
	}

	apiMembers, err := cmd.constructAPIMembers(logger, reconfigurableSink)
	if err != nil {
		return nil, err
	}

	backendMembers, err := cmd.constructBackendMembers(logger)
	if err != nil {
		return nil, err
	}

	return append(apiMembers, backendMembers...), nil
}

func (cmd *RunCommand) constructAPIMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
) ([]grouper.Member, error) {
	connectionName := "api"
	maxConns := 32

	lockFactory, err := cmd.lockFactory()
	if err != nil {
		return nil, err
	}
	dbConn, err := cmd.constructDBConn(retryingDriverName, logger, maxConns, connectionName, lockFactory)
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

	httpClient, err := cmd.skyHttpClient()
	if err != nil {
		return nil, err
	}
	authHandler, err := skymarshal.NewServer(&skymarshal.Config{
		Logger:      logger,
		TeamFactory: teamFactory,
		Flags:       cmd.Auth.AuthFlags,
		ExternalURL: cmd.ExternalURL.String(),
		HttpClient:  httpClient,
		Postgres:    cmd.Postgres,
	})
	if err != nil {
		return nil, err
	}

	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	resourceFetcherFactory := resource.NewFetcherFactory(lockFactory, clock.NewClock(), dbResourceCacheFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFetcherFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		clock.NewClock(),
	)

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		image.NewImageFactory(imageResourceFetcherFactory),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
	)

	workerClient := cmd.constructWorkerPool(
		logger,
		workerProvider,
	)

	resourceFetcher := resourceFetcherFactory.FetcherFor(workerClient)
	resourceFactory := resource.NewResourceFactory(workerClient)
	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	variablesFactory, err := cmd.variablesFactory(logger)
	if err != nil {
		return nil, err
	}
	engine := cmd.constructEngine(workerClient, resourceFetcher, resourceFactory, dbResourceCacheFactory, variablesFactory, defaultLimits)

	dbResourceConfigCheckSessionFactory := db.NewResourceConfigCheckSessionFactory(dbConn, lockFactory)
	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		resourceFactory,
		dbResourceConfigCheckSessionFactory,
		cmd.ResourceTypeCheckingInterval,
		cmd.ResourceCheckingInterval,
		engine,
	)

	radarScannerFactory := radar.NewScannerFactory(
		resourceFactory,
		dbResourceConfigCheckSessionFactory,
		cmd.ResourceTypeCheckingInterval,
		cmd.ResourceCheckingInterval,
		cmd.ExternalURL.String(),
		variablesFactory,
	)

	drain := make(chan struct{})
	credsManagers := cmd.CredentialManagers
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbResourceFactory := db.NewResourceFactory(dbConn, lockFactory)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	gcContainerDestroyer := gc.NewDestroyer(logger, dbContainerRepository, dbVolumeRepository)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
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
		engine,
		workerClient,
		workerProvider,
		drain,
		radarSchedulerFactory,
		radarScannerFactory,
		variablesFactory,
		credsManagers,
	)

	if err != nil {
		return nil, err
	}

	accessFactory := accessor.NewAccessFactory(authHandler.PublicKey())
	apiHandler = accessor.NewHandler(apiHandler, accessFactory)
	webHandler, err := webHandler(logger)
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
		)

		httpsHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			apiHandler,
			authHandler,
		)
	} else {
		httpHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			apiHandler,
			authHandler,
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
		tlsConfig, err := cmd.tlsConfig()
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

func (cmd *RunCommand) constructBackendMembers(
	logger lager.Logger,
) ([]grouper.Member, error) {

	if cmd.Syslog.Address != "" && cmd.Syslog.Transport == "" {
		return nil, fmt.Errorf("syslog Drainer is misconfigured, cannot configure a drainer without a transport")
	}

	syslogDrainConfigured := true
	if cmd.Syslog.Address == "" {
		syslogDrainConfigured = false
	}

	connectionName := "backend"
	maxConns := 32

	drain := make(chan struct{})
	lockFactory, err := cmd.lockFactory()
	if err != nil {
		return nil, err
	}
	dbConn, err := cmd.constructDBConn(retryingDriverName, logger, maxConns, connectionName, lockFactory)
	if err != nil {
		return nil, err
	}
	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	resourceFetcherFactory := resource.NewFetcherFactory(lockFactory, clock.NewClock(), dbResourceCacheFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFetcherFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		clock.NewClock(),
	)
	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	dbVolumeRepository := db.NewVolumeRepository(dbConn)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return nil, err
	}
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		image.NewImageFactory(imageResourceFetcherFactory),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeRepository,
		teamFactory,
		dbWorkerFactory,
		workerVersion,
		cmd.BaggageclaimResponseHeaderTimeout,
	)
	workerClient := cmd.constructWorkerPool(
		logger,
		workerProvider,
	)

	resourceFetcher := resourceFetcherFactory.FetcherFor(workerClient)
	resourceFactory := resource.NewResourceFactory(workerClient)
	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	variablesFactory, err := cmd.variablesFactory(logger)
	if err != nil {
		return nil, err
	}
	engine := cmd.constructEngine(workerClient, resourceFetcher, resourceFactory, dbResourceCacheFactory, variablesFactory, defaultLimits)

	dbResourceConfigCheckSessionFactory := db.NewResourceConfigCheckSessionFactory(dbConn, lockFactory)
	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		resourceFactory,
		dbResourceConfigCheckSessionFactory,
		cmd.ResourceTypeCheckingInterval,
		cmd.ResourceCheckingInterval,
		engine,
	)
	dbWorkerLifecycle := db.NewWorkerLifecycle(dbConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(dbConn)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(dbConn)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
	bus := dbConn.Bus()
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	members := []grouper.Member{
		{Name: "drainer", Runner: drainer{
			logger: logger.Session("drain"),
			drain:  drain,
			tracker: builds.NewTracker(
				logger.Session("build-tracker"),
				dbBuildFactory,
				engine,
			),
			bus: bus,
		}},
		{Name: "pipelines", Runner: pipelines.SyncRunner{
			Syncer: cmd.constructPipelineSyncer(
				logger.Session("syncer"),
				dbPipelineFactory,
				radarSchedulerFactory,
				variablesFactory,
			),
			Interval: 10 * time.Second,
			Clock:    clock.NewClock(),
		}},
		{Name: "builds", Runner: builds.TrackerRunner{
			Tracker: builds.NewTracker(
				logger.Session("build-tracker"),
				dbBuildFactory,
				engine,
			),
			ListenBus: bus,
			Interval:  cmd.BuildTrackerInterval,
			Clock:     clock.NewClock(),
			DrainCh:   drain,
			Logger:    logger.Session("tracker-runner"),
		}},
		{Name: "collector", Runner: lockrunner.NewRunner(
			logger.Session("collector"),
			gc.NewCollector(
				gc.NewBuildCollector(dbBuildFactory),
				gc.NewWorkerCollector(dbWorkerLifecycle),
				gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
				gc.NewResourceConfigCollector(dbResourceConfigFactory),
				gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
				gc.NewVolumeCollector(
					dbVolumeRepository,
				),
				gc.NewContainerCollector(
					dbContainerRepository,
					gc.NewWorkerJobRunner(
						logger.Session("container-collector-worker-job-runner"),
						workerProvider,
						time.Minute,
					),
				),
				gc.NewResourceConfigCheckSessionCollector(
					resourceConfigCheckSessionLifecycle,
				),
			),
			"collector",
			lockFactory,
			clock.NewClock(),
			cmd.GC.Interval,
		)},
		// run separately so as to not preempt critical GC
		{Name: "build-log-collector", Runner: lockrunner.NewRunner(
			logger.Session("build-log-collector"),
			gc.NewBuildLogCollector(
				dbPipelineFactory,
				500,
				gc.NewBuildLogRetentionCalculator(
					cmd.DefaultBuildLogsToRetain,
					cmd.MaxBuildLogsToRetain,
				),
				syslogDrainConfigured,
			),
			"build-reaper",
			lockFactory,
			clock.NewClock(),
			30*time.Second,
		)},
	}

	//Syslog Drainer Configuration
	if syslogDrainConfigured {
		members = append(members, grouper.Member{
			Name: "syslog", Runner: lockrunner.NewRunner(
				logger.Session("syslog"),

				syslog.NewDrainer(
					cmd.Syslog.Transport,
					cmd.Syslog.Address,
					cmd.Syslog.Hostname,
					cmd.Syslog.CACerts,
					dbBuildFactory,
				),
				"syslog-drainer",
				lockFactory,
				clock.NewClock(),
				cmd.Syslog.DrainInterval,
			)},
		)
	}
	if cmd.Worker.GardenURL.URL != nil {
		members = cmd.appendStaticWorker(logger, dbWorkerFactory, members)
	}
	return members, nil
}

func workerVersion() (*version.Version, error) {
	var workerVersion *version.Version
	if len(WorkerVersion) != 0 {
		version, err := version.NewVersionFromString(WorkerVersion)
		if err != nil {
			return nil, err
		}

		workerVersion = &version
	}
	return workerVersion, nil
}

func (cmd *RunCommand) variablesFactory(logger lager.Logger) (creds.VariablesFactory, error) {
	var variablesFactory creds.VariablesFactory = noop.NewNoopFactory()
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

		variablesFactory, err = manager.NewVariablesFactory(credsLogger)
		if err != nil {
			return nil, err
		}

		break
	}
	return variablesFactory, nil
}

func (cmd *RunCommand) lockFactory() (lock.LockFactory, error) {
	lockConn, err := cmd.constructLockConn(retryingDriverName)
	if err != nil {
		return nil, err
	}

	return lock.NewLockFactory(lockConn), nil
}

func (cmd *RunCommand) newKey() *encryption.Key {
	var newKey *encryption.Key
	if cmd.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.EncryptionKey.AEAD)
	}
	return newKey
}

func (cmd *RunCommand) oldKey() *encryption.Key {
	var oldKey *encryption.Key
	if cmd.OldEncryptionKey.AEAD != nil {
		oldKey = encryption.NewKey(cmd.OldEncryptionKey.AEAD)
	}
	return oldKey
}

func webHandler(logger lager.Logger) (http.Handler, error) {
	webHandler, err := web.NewHandler(logger)
	if err != nil {
		return nil, err
	}
	return metric.WrapHandler(logger, "web", webHandler), nil
}

func (cmd *RunCommand) skyHttpClient() (*http.Client, error) {
	httpClient := http.DefaultClient

	if cmd.isTLSEnabled() {
		cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
		if err != nil {
			return nil, err
		}

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, err
		}

		certpool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		certpool.AddCert(x509Cert)

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
		TargetURL:  cmd.defaultURL().URL,
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

func (cmd *RunCommand) tlsConfig() (*tls.Config, error) {
	var tlsConfig *tls.Config

	if cmd.isTLSEnabled() {
		cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
		if err != nil {
			return nil, err
		}

		tlsConfig = &tls.Config{
			Certificates:     []tls.Certificate{cert},
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			},
			PreferServerCipherSuites: true,
			NextProtos:               []string{"h2"},
		}
	}
	return tlsConfig, nil
}

func (cmd *RunCommand) parseDefaultLimits() (atc.ContainerLimits, error) {
	return atc.ContainerLimitsParser(map[string]interface{}{
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

func (cmd *RunCommand) defaultURL() flag.URL {
	return flag.URL{
		URL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", cmd.defaultBindIP().String(), cmd.BindPort),
		},
	}
}

func onReady(runner ifrit.Runner, cb func()) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		process := ifrit.Background(runner)

		subExited := process.Wait()
		subReady := process.Ready()

		for {
			select {
			case <-subReady:
				cb()
				close(ready)
				subReady = nil
			case err := <-subExited:
				return err
			case sig := <-signals:
				process.Signal(sig)
			}
		}
	})
}

func (cmd *RunCommand) validate() error {
	var errs *multierror.Error

	tlsFlagCount := 0
	if cmd.TLSBindPort != 0 {
		tlsFlagCount++
	}
	if cmd.TLSCert != "" {
		tlsFlagCount++
	}
	if cmd.TLSKey != "" {
		tlsFlagCount++
	}

	if tlsFlagCount == 3 {
		if cmd.ExternalURL.URL.Scheme != "https" {
			errs = multierror.Append(
				errs,
				errors.New("must specify HTTPS external-url to use TLS"),
			)
		}
	} else if tlsFlagCount != 0 {
		errs = multierror.Append(
			errs,
			errors.New("must specify --tls-bind-port, --tls-cert, --tls-key to use TLS"),
		)
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

	return metric.Initialize(logger.Session("metrics"), host, cmd.Metrics.Attributes)
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

	return dbConn, nil
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

func (cmd *RunCommand) constructWorkerPool(
	logger lager.Logger,
	workerProvider worker.WorkerProvider,
) worker.Client {

	var strategy worker.ContainerPlacementStrategy
	switch cmd.ContainerPlacementStrategy {
	case "random":
		strategy = worker.NewRandomPlacementStrategy()
	default:
		strategy = worker.NewVolumeLocalityPlacementStrategy()
	}

	return worker.NewPool(
		workerProvider,
		strategy,
	)
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
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	variablesFactory creds.VariablesFactory,
	defaultLimits atc.ContainerLimits,
) engine.Engine {
	gardenFactory := exec.NewGardenFactory(
		workerClient,
		resourceFetcher,
		resourceFactory,
		dbResourceCacheFactory,
		variablesFactory,
		defaultLimits,
	)

	execV2Engine := engine.NewExecEngine(
		gardenFactory,
		engine.NewBuildDelegateFactory(),
		cmd.ExternalURL.String(),
	)

	execV1Engine := engine.NewExecV1DummyEngine()

	return engine.NewDBEngine(engine.Engines{execV2Engine, execV1Engine}, cmd.PeerURLOrDefault().String())
}

func (cmd *RunCommand) constructHTTPHandler(
	logger lager.Logger,
	webHandler http.Handler,
	apiHandler http.Handler,
	authHandler http.Handler,
) http.Handler {
	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", apiHandler)
	webMux.Handle("/sky/", authHandler)
	webMux.Handle("/auth/", authHandler)
	webMux.Handle("/login", authHandler)
	webMux.Handle("/logout", authHandler)
	webMux.Handle("/", webHandler)

	httpHandler := wrappa.LoggerHandler{
		Logger: logger,

		Handler: wrappa.SecurityHandler{
			XFrameOptions: cmd.Server.XFrameOptions,

			// proxy Authorization header to/from auth cookie,
			// to support auth from JS (EventSource) and custom JWT auth
			Handler: auth.CookieSetHandler{
				Handler: webMux,
			},
		},
	}

	return httpHandler
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
	engine engine.Engine,
	workerClient worker.Client,
	workerProvider worker.WorkerProvider,
	drain <-chan struct{},
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	radarScannerFactory radar.ScannerFactory,
	variablesFactory creds.VariablesFactory,
	credsManagers creds.Managers,
) (http.Handler, error) {

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(teamFactory)
	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)
	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)
	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewAPIAuthWrappa(
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),
		wrappa.NewConcourseVersionWrappa(Version),
	}

	return api.NewHandler(
		logger,
		cmd.ExternalURL.String(),
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

		cmd.PeerURLOrDefault().String(),
		buildserver.NewEventHandler,
		drain,

		engine,
		workerClient,
		workerProvider,
		radarSchedulerFactory,
		radarScannerFactory,

		reconfigurableSink,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		Version,
		WorkerVersion,
		variablesFactory,
		credsManagers,
		containerserver.NewInterceptTimeoutFactory(cmd.InterceptIdleTimeout),
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

func (cmd *RunCommand) constructPipelineSyncer(
	logger lager.Logger,
	pipelineFactory db.PipelineFactory,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	variablesFactory creds.VariablesFactory,
) *pipelines.Syncer {
	return pipelines.NewSyncer(
		logger,
		pipelineFactory,
		func(pipeline db.Pipeline) ifrit.Runner {
			variables := variablesFactory.NewVariables(pipeline.TeamName(), pipeline.Name())
			return grouper.NewParallel(os.Interrupt, grouper.Members{
				{
					pipeline.ScopedName("radar"),
					radar.NewRunner(
						logger.Session(pipeline.ScopedName("radar")),
						cmd.Developer.Noop,
						radarSchedulerFactory.BuildScanRunnerFactory(pipeline, cmd.ExternalURL.String(), variables),
						pipeline,
						1*time.Minute,
					),
				},
				{
					pipeline.ScopedName("scheduler"),
					&scheduler.Runner{
						Logger:    logger.Session(pipeline.ScopedName("scheduler")),
						Pipeline:  pipeline,
						Scheduler: radarSchedulerFactory.BuildScheduler(pipeline, cmd.ExternalURL.String(), variables),
						Noop:      cmd.Developer.Noop,
						Interval:  10 * time.Second,
					},
				},
			})
		},
	)
}

func (cmd *RunCommand) appendStaticWorker(
	logger lager.Logger,
	workerFactory db.WorkerFactory,
	members []grouper.Member,
) []grouper.Member {
	var resourceTypes []atc.WorkerResourceType
	for t, resourcePath := range cmd.Worker.ResourceTypes {
		resourceTypes = append(resourceTypes, atc.WorkerResourceType{
			Type:  t,
			Image: resourcePath,
		})
	}

	return append(members,
		grouper.Member{
			Name: "static-worker",
			Runner: worker.NewHardcoded(
				logger,
				workerFactory,
				clock.NewClock(),
				cmd.Worker.GardenURL.URL.Host,
				cmd.Worker.BaggageclaimURL.String(),
				resourceTypes,
			),
		},
	)
}

func (cmd *RunCommand) isTLSEnabled() bool {
	return cmd.TLSBindPort != 0
}

func init() {
}
