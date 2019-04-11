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
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/api/containerserver"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/noop"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/lockrunner"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/pipelines"
	"github.com/concourse/concourse/atc/radar"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/syslog"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/skymarshal"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/web"
	"github.com/concourse/concourse/web/indexhandler"
	"github.com/concourse/flag"
	"github.com/concourse/retryhttp"
	"github.com/cppforlife/go-semi-semantic/version"
	multierror "github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"

	// dynamically registered metric emitters
	_ "github.com/concourse/concourse/atc/metric/emitter"

	// dynamically registered credential managers
	_ "github.com/concourse/concourse/atc/creds/credhub"
	_ "github.com/concourse/concourse/atc/creds/kubernetes"
	_ "github.com/concourse/concourse/atc/creds/secretsmanager"
	_ "github.com/concourse/concourse/atc/creds/ssm"
	_ "github.com/concourse/concourse/atc/creds/vault"
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

	Postgres flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`

	CredentialManagement creds.CredentialManagementConfig `group:"Credential Management"`
	CredentialManagers   creds.Managers

	EncryptionKey    flag.Cipher `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	OldEncryptionKey flag.Cipher `long:"old-encryption-key" description:"Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted."`

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	InterceptIdleTimeout time.Duration `long:"intercept-idle-timeout" default:"0m" description:"Length of time for a intercepted session to be idle before terminating."`

	EnableGlobalResources bool `long:"enable-global-resources" description:"Enable equivalent resources across pipelines and teams to share a single version history."`

	GlobalResourceCheckTimeout   time.Duration `long:"global-resource-check-timeout" default:"1h" description:"Time limit on checking for new versions of resources."`
	ResourceCheckingInterval     time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	ResourceTypeCheckingInterval time.Duration `long:"resource-type-checking-interval" default:"1m" description:"Interval on which to check for new versions of resource types."`

	ContainerPlacementStrategy        string        `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" description:"Method by which a worker is selected during container placement."`
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
		HostName            string            `long:"metrics-host-name" description:"Host string to attach to emitted metrics."`
		Attributes          map[string]string `long:"metrics-attribute" description:"A key-value attribute to attach to emitted metrics. Can be specified multiple times." value-name:"NAME:VALUE"`
		CaptureErrorMetrics bool              `long:"capture-error-metrics" description:"Enable capturing of error log metrics"`
	} `group:"Metrics & Diagnostics"`

	Server struct {
		InstanceName  string `long:"instance-name" description:"A name for this Concourse instance, to be displayed on the dashboard page."`
		XFrameOptions string `long:"x-frame-options" description:"The value to set for X-Frame-Options. If omitted, the header is not set."`
	} `group:"Web Server"`

	LogDBQueries bool `long:"log-db-queries" description:"Log database queries."`

	GC struct {
		Interval time.Duration `long:"interval" default:"30s" description:"Interval on which to perform garbage collection."`

		OneOffBuildGracePeriod time.Duration `long:"one-off-grace-period" default:"5m" description:"Period after which one-off build containers will be garbage-collected."`
		MissingGracePeriod     time.Duration `long:"missing-grace-period" default:"5m" description:"Period after which to reap containers and volumes that were created but went missing from the worker."`
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
		return fmt.Errorf("Could not migrate to version: %d Reason: %s", version, err.Error())
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

	commandSession := logger.Session("cmd")
	startTime := time.Now()

	commandSession.Info("start")
	defer commandSession.Info("finish", lager.Data{
		"duration": time.Now().Sub(startTime),
	})

	atc.EnableGlobalResources = cmd.EnableGlobalResources

	radar.GlobalResourceCheckTimeout = cmd.GlobalResourceCheckTimeout
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

	apiConn, err := cmd.constructDBConn(retryingDriverName, logger, 32, "api", lockFactory)
	if err != nil {
		return nil, err
	}

	backendConn, err := cmd.constructDBConn(retryingDriverName, logger, 32, "backend", lockFactory)
	if err != nil {
		return nil, err
	}

	storage, err := storage.NewPostgresStorage(logger, cmd.Postgres)
	if err != nil {
		return nil, err
	}

	variablesFactory, err := cmd.variablesFactory(logger)
	if err != nil {
		return nil, err
	}

	members, err := cmd.constructMembers(logger, reconfigurableSink, apiConn, backendConn, storage, lockFactory, variablesFactory)
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
		for _, closer := range []Closer{lockConn, apiConn, backendConn, storage} {
			closer.Close()
		}
	}

	return run(grouper.NewParallel(os.Interrupt, members), onReady, onExit), nil
}

func (cmd *RunCommand) constructMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	apiConn db.Conn,
	backendConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	variablesFactory creds.VariablesFactory,
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

	apiMembers, err := cmd.constructAPIMembers(logger, reconfigurableSink, apiConn, storage, lockFactory, variablesFactory)
	if err != nil {
		return nil, err
	}

	backendMembers, err := cmd.constructBackendMembers(logger, backendConn, lockFactory, variablesFactory)
	if err != nil {
		return nil, err
	}

	return append(apiMembers, backendMembers...), nil
}

func (cmd *RunCommand) constructAPIMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	dbConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	variablesFactory creds.VariablesFactory,
) ([]grouper.Member, error) {
	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	_, err := teamFactory.CreateDefaultTeamIfNotExists()
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
		HTTPClient:  httpClient,
		Storage:     storage,
	})
	if err != nil {
		return nil, err
	}

	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := resource.NewFetchSourceFactory(dbResourceCacheFactory, resourceFactory)
	resourceFetcher := resource.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
		resourceFactory,
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

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider)

	checkContainerStrategy := worker.NewRandomPlacementStrategy()

	radarScannerFactory := radar.NewScannerFactory(
		pool,
		resourceFactory,
		dbResourceConfigFactory,
		cmd.ResourceTypeCheckingInterval,
		cmd.ResourceCheckingInterval,
		cmd.ExternalURL.String(),
		variablesFactory,
		checkContainerStrategy,
	)

	drain := make(chan struct{})
	credsManagers := cmd.CredentialManagers
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbResourceFactory := db.NewResourceFactory(dbConn, lockFactory)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	gcContainerDestroyer := gc.NewDestroyer(logger, dbContainerRepository, dbVolumeRepository)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
	accessFactory := accessor.NewAccessFactory(authHandler.PublicKey())

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
		dbResourceConfigFactory,
		workerClient,
		drain,
		radarScannerFactory,
		variablesFactory,
		credsManagers,
		accessFactory,
	)

	if err != nil {
		return nil, err
	}

	indexhandler.InstanceName = cmd.Server.InstanceName
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
	dbConn db.Conn,
	lockFactory lock.LockFactory,
	variablesFactory creds.VariablesFactory,
) ([]grouper.Member, error) {

	if cmd.Syslog.Address != "" && cmd.Syslog.Transport == "" {
		return nil, fmt.Errorf("syslog Drainer is misconfigured, cannot configure a drainer without a transport")
	}

	syslogDrainConfigured := true
	if cmd.Syslog.Address == "" {
		syslogDrainConfigured = false
	}

	drain := make(chan struct{})

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)

	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn, lockFactory)
	fetchSourceFactory := resource.NewFetchSourceFactory(dbResourceCacheFactory, resourceFactory)
	resourceFetcher := resource.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
		resourceFactory,
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

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider)

	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	buildContainerStrategy := cmd.chooseBuildContainerStrategy()
	checkContainerStrategy := worker.NewRandomPlacementStrategy()

	engine := cmd.constructEngine(
		pool,
		workerClient,
		resourceFetcher,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		variablesFactory,
		defaultLimits,
		buildContainerStrategy,
		resourceFactory,
	)

	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		pool,
		resourceFactory,
		dbResourceConfigFactory,
		cmd.ResourceTypeCheckingInterval,
		cmd.ResourceCheckingInterval,
		engine,
		checkContainerStrategy,
	)
	dbWorkerLifecycle := db.NewWorkerLifecycle(dbConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(dbConn)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	dbArtifactLifecycle := db.NewArtifactLifecycle(dbConn)
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
				logger.Session("pipelines"),
				dbPipelineFactory,
				radarSchedulerFactory,
				variablesFactory,
				bus,
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
			Notifications: bus,
			Interval:      cmd.BuildTrackerInterval,
			Clock:         clock.NewClock(),
			DrainCh:       drain,
			Logger:        logger.Session("tracker-runner"),
		}},
		{Name: "collector", Runner: lockrunner.NewRunner(
			logger.Session("collector"),
			gc.NewCollector(
				gc.NewBuildCollector(dbBuildFactory),
				gc.NewWorkerCollector(dbWorkerLifecycle),
				gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
				gc.NewResourceConfigCollector(dbResourceConfigFactory),
				gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
				gc.NewArtifactCollector(dbArtifactLifecycle),
				gc.NewVolumeCollector(
					dbVolumeRepository,
					cmd.GC.MissingGracePeriod,
				),
				gc.NewContainerCollector(
					dbContainerRepository,
					gc.NewWorkerJobRunner(
						logger.Session("container-collector-worker-job-runner"),
						workerProvider,
						time.Minute,
					),
					cmd.GC.MissingGracePeriod,
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

func workerVersion() (version.Version, error) {
	return version.NewVersionFromString(concourse.WorkerVersion)
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
	return creds.NewRetryableVariablesFactory(variablesFactory, cmd.CredentialManagement.RetryConfig), nil
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

func (cmd *RunCommand) DefaultURL() flag.URL {
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

func (cmd *RunCommand) chooseBuildContainerStrategy() worker.ContainerPlacementStrategy {
	var strategy worker.ContainerPlacementStrategy
	switch cmd.ContainerPlacementStrategy {
	case "random":
		strategy = worker.NewRandomPlacementStrategy()
	case "fewest-build-containers":
		strategy = worker.NewFewestBuildContainersPlacementStrategy()
	default:
		strategy = worker.NewVolumeLocalityPlacementStrategy()
	}

	return strategy
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

	err = team.UpdateProviderAuth(atc.TeamAuth(auth))
	if err != nil {
		return err
	}

	return nil
}

func (cmd *RunCommand) constructEngine(
	workerPool worker.Pool,
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	variablesFactory creds.VariablesFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	resourceFactory resource.ResourceFactory,
) engine.Engine {
	gardenFactory := exec.NewGardenFactory(
		workerPool,
		workerClient,
		resourceFetcher,
		resourceCacheFactory,
		resourceConfigFactory,
		variablesFactory,
		defaultLimits,
		strategy,
		resourceFactory,
	)

	execV2Engine := engine.NewExecEngine(
		gardenFactory,
		engine.NewBuildDelegateFactory(),
		cmd.ExternalURL.String(),
	)

	execV1Engine := engine.NewExecV1DummyEngine()

	return engine.NewDBEngine(engine.Engines{execV2Engine, execV1Engine})
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
	resourceConfigFactory db.ResourceConfigFactory,
	workerClient worker.Client,
	drain <-chan struct{},
	radarScannerFactory radar.ScannerFactory,
	variablesFactory creds.VariablesFactory,
	credsManagers creds.Managers,
	accessFactory accessor.AccessFactory,
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
		wrappa.NewConcourseVersionWrappa(concourse.Version),
		wrappa.NewAccessorWrappa(accessFactory),
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
		resourceConfigFactory,

		buildserver.NewEventHandler,
		drain,

		workerClient,
		radarScannerFactory,

		reconfigurableSink,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		concourse.Version,
		concourse.WorkerVersion,
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
	bus db.NotificationsBus,
) *pipelines.Syncer {
	return pipelines.NewSyncer(
		logger,
		pipelineFactory,
		func(pipeline db.Pipeline) ifrit.Runner {
			variables := variablesFactory.NewVariables(pipeline.TeamName(), pipeline.Name())
			return grouper.NewParallel(os.Interrupt, grouper.Members{
				{
					Name: fmt.Sprintf("radar:%d", pipeline.ID()),
					Runner: radar.NewRunner(
						logger.Session("radar").WithData(lager.Data{
							"team":     pipeline.TeamName(),
							"pipeline": pipeline.Name(),
						}),
						cmd.Developer.Noop,
						radarSchedulerFactory.BuildScanRunnerFactory(pipeline, cmd.ExternalURL.String(), variables, bus),
						pipeline,
						1*time.Minute,
					),
				},
				{
					Name: fmt.Sprintf("scheduler:%d", pipeline.ID()),
					Runner: &scheduler.Runner{
						Logger: logger.Session("scheduler", lager.Data{
							"team":     pipeline.TeamName(),
							"pipeline": pipeline.Name(),
						}),
						Pipeline:  pipeline,
						Scheduler: radarSchedulerFactory.BuildScheduler(pipeline),
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
