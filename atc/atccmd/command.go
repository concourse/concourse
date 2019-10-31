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

	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc/resource"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/api/containerserver"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/noop"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/lidar"
	"github.com/concourse/concourse/atc/lockrunner"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/pipelines"
	"github.com/concourse/concourse/atc/radar"
	"github.com/concourse/concourse/atc/scheduler"
	"github.com/concourse/concourse/atc/syslog"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/image"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/skymarshal"
	"github.com/concourse/concourse/skymarshal/skycmd"
	"github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/concourse/web"
	"github.com/concourse/flag"
	"github.com/concourse/retryhttp"
	"github.com/cppforlife/go-semi-semantic/version"
	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	// dynamically registered metric emitters
	_ "github.com/concourse/concourse/atc/metric/emitter"

	// dynamically registered credential managers
	_ "github.com/concourse/concourse/atc/creds/conjur"
	_ "github.com/concourse/concourse/atc/creds/credhub"
	_ "github.com/concourse/concourse/atc/creds/dummy"
	_ "github.com/concourse/concourse/atc/creds/kubernetes"
	_ "github.com/concourse/concourse/atc/creds/secretsmanager"
	_ "github.com/concourse/concourse/atc/creds/ssm"
	_ "github.com/concourse/concourse/atc/creds/vault"
)

var defaultDriverName = "postgres"
var retryingDriverName = "too-many-connections-retrying"

const runnerInterval = 10 * time.Second

type ATCCommand struct {
	RunCommand RunCommand `command:"run"`
	Migration  Migration  `command:"migrate"`
}

type RunCommand struct {
	Logger flag.Lager

	varSourcePool creds.VarSourcePool

	BindIP   flag.IP `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16  `long:"bind-port" default:"8080"    description:"Port on which to listen for HTTP traffic."`

	TLSBindPort uint16    `long:"tls-bind-port" description:"Port on which to listen for HTTPS traffic."`
	TLSCert     flag.File `long:"tls-cert"      description:"File containing an SSL certificate."`
	TLSKey      flag.File `long:"tls-key"       description:"File containing an RSA private key, used to encrypt HTTPS traffic."`

	LetsEncrypt struct {
		Enable  bool     `long:"enable-lets-encrypt"   description:"Automatically configure TLS certificates via Let's Encrypt/ACME."`
		ACMEURL flag.URL `long:"lets-encrypt-acme-url" description:"URL of the ACME CA directory endpoint." default:"https://acme-v02.api.letsencrypt.org/directory"`
	} `group:"Let's Encrypt Configuration"`

	ExternalURL flag.URL `long:"external-url" description:"URL used to reach any ATC from the outside world."`

	Postgres flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`

	MaxOpenConnections int `long:"max-conns" description:"The maximum number of open connections for a connection pool." default:"32"`

	CredentialManagement creds.CredentialManagementConfig `group:"Credential Management"`
	CredentialManagers   creds.Managers

	EncryptionKey    flag.Cipher `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	OldEncryptionKey flag.Cipher `long:"old-encryption-key" description:"Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted."`

	DebugBindIP   flag.IP `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16  `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	InterceptIdleTimeout time.Duration `long:"intercept-idle-timeout" default:"0m" description:"Length of time for a intercepted session to be idle before terminating."`

	EnableGlobalResources bool          `long:"enable-global-resources" description:"Enable equivalent resources across pipelines and teams to share a single version history."`
	EnableLidar           bool          `long:"enable-lidar" description:"The Future™ of resource checking."`
	LidarScannerInterval  time.Duration `long:"lidar-scanner-interval" default:"1m" description:"Interval on which the resource scanner will run to see if new checks need to be scheduled"`
	LidarCheckerInterval  time.Duration `long:"lidar-checker-interval" default:"10s" description:"Interval on which the resource checker runs any scheduled checks"`

	GlobalResourceCheckTimeout   time.Duration `long:"global-resource-check-timeout" default:"1h" description:"Time limit on checking for new versions of resources."`
	ResourceCheckingInterval     time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	ResourceTypeCheckingInterval time.Duration `long:"resource-type-checking-interval" default:"1m" description:"Interval on which to check for new versions of resource types."`

	ContainerPlacementStrategy        string        `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" choice:"fewest-build-containers" choice:"limit-active-tasks" description:"Method by which a worker is selected during container placement."`
	MaxActiveTasksPerWorker           int           `long:"max-active-tasks-per-worker" default:"0" description:"Maximum allowed number of active build tasks per worker. Has effect only when used with limit-active-tasks placement strategy. 0 means no limit."`
	BaggageclaimResponseHeaderTimeout time.Duration `long:"baggageclaim-response-header-timeout" default:"1m" description:"How long to wait for Baggageclaim to send the response header."`

	GardenRequestTimeout time.Duration `long:"garden-request-timeout" default:"5m" description:"How long to wait for requests to Garden to complete. 0 means no timeout."`

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
		BufferSize          uint32            `long:"metrics-buffer-size" default:"1000" description:"The size of the buffer used in emitting event metrics."`
		CaptureErrorMetrics bool              `long:"capture-error-metrics" description:"Enable capturing of error log metrics"`
	} `group:"Metrics & Diagnostics"`

	Server struct {
		XFrameOptions string `long:"x-frame-options" default:"deny" description:"The value to set for X-Frame-Options."`
		ClusterName   string `long:"cluster-name" description:"A name for this Concourse cluster, to be displayed on the dashboard page."`
	} `group:"Web Server"`

	LogDBQueries   bool `long:"log-db-queries" description:"Log database queries."`
	LogClusterName bool `long:"log-cluster-name" description:"Log cluster name."`

	GC struct {
		Interval time.Duration `long:"interval" default:"30s" description:"Interval on which to perform garbage collection."`

		OneOffBuildGracePeriod time.Duration `long:"one-off-grace-period" default:"5m" description:"Period after which one-off build containers will be garbage-collected."`
		MissingGracePeriod     time.Duration `long:"missing-grace-period" default:"5m" description:"Period after which to reap containers and volumes that were created but went missing from the worker."`
		CheckRecyclePeriod     time.Duration `long:"check-recycle-period" default:"6h" description:"Period after which to reap checks that are completed."`
	} `group:"Garbage Collection" namespace:"gc"`

	BuildTrackerInterval time.Duration `long:"build-tracker-interval" default:"10s" description:"Interval on which to run build tracking."`

	TelemetryOptIn bool `long:"telemetry-opt-in" hidden:"true" description:"Enable anonymous concourse version reporting."`

	DefaultBuildLogsToRetain uint64 `long:"default-build-logs-to-retain" description:"Default build logs to retain, 0 means all"`
	MaxBuildLogsToRetain     uint64 `long:"max-build-logs-to-retain" description:"Maximum build logs to retain, 0 means not specified. Will override values configured in jobs"`

	DefaultDaysToRetainBuildLogs uint64 `long:"default-days-to-retain-build-logs" description:"Default days to retain build logs. 0 means unlimited"`
	MaxDaysToRetainBuildLogs     uint64 `long:"max-days-to-retain-build-logs" description:"Maximum days to retain build logs, 0 means not specified. Will override values configured in jobs"`

	DefaultCpuLimit    *int    `long:"default-task-cpu-limit" description:"Default max number of cpu shares per task, 0 means unlimited"`
	DefaultMemoryLimit *string `long:"default-task-memory-limit" description:"Default maximum memory per task, 0 means unlimited"`

	Auditor struct {
		EnableBuildAuditLog     bool `long:"enable-build-auditing" description:"Enable auditing for all api requests connected to builds."`
		EnableContainerAuditLog bool `long:"enable-container-auditing" description:"Enable auditing for all api requests connected to containers."`
		EnableJobAuditLog       bool `long:"enable-job-auditing" description:"Enable auditing for all api requests connected to jobs."`
		EnablePipelineAuditLog  bool `long:"enable-pipeline-auditing" description:"Enable auditing for all api requests connected to pipelines."`
		EnableResourceAuditLog  bool `long:"enable-resource-auditing" description:"Enable auditing for all api requests connected to resources."`
		EnableSystemAuditLog    bool `long:"enable-system-auditing" description:"Enable auditing for all api requests connected to system transactions."`
		EnableTeamAuditLog      bool `long:"enable-team-auditing" description:"Enable auditing for all api requests connected to teams."`
		EnableWorkerAuditLog    bool `long:"enable-worker-auditing" description:"Enable auditing for all api requests connected to workers."`
		EnableVolumeAuditLog    bool `long:"enable-volume-auditing" description:"Enable auditing for all api requests connected to volumes."`
	}

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

	EnableRedactSecrets bool `long:"enable-redact-secrets" description:"Enable redacting secrets in build logs."`

	ConfigRBAC string `long:"config-rbac" description:"Customize RBAC role-action mapping."`
}

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

	apiConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.MaxOpenConnections, "api", lockFactory)
	if err != nil {
		return nil, err
	}

	backendConn, err := cmd.constructDBConn(retryingDriverName, logger, cmd.MaxOpenConnections, "backend", lockFactory)
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

	cmd.varSourcePool = creds.NewVarSourcePool(5*time.Minute, clock.NewClock())

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

	apiMembers, err := cmd.constructAPIMembers(logger, reconfigurableSink, apiConn, storage, lockFactory, secretManager)
	if err != nil {
		return nil, err
	}

	backendMembers, err := cmd.constructBackendMembers(logger, backendConn, lockFactory, secretManager)
	if err != nil {
		return nil, err
	}

	gcMembers, err := cmd.constructGCMember(logger, gcConn, lockFactory)
	if err != nil {
		return nil, err
	}

	return append(apiMembers, append(backendMembers, gcMembers...)...), nil
}

func (cmd *RunCommand) constructAPIMembers(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	dbConn db.Conn,
	storage storage.Storage,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
) ([]grouper.Member, error) {
	teamFactory := db.NewTeamFactory(dbConn, lockFactory)
	userFactory := db.NewUserFactory(dbConn)

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
		UserFactory: userFactory,
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

	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory),
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
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider)

	credsManagers := cmd.CredentialManagers
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbJobFactory := db.NewJobFactory(dbConn, lockFactory)
	dbResourceFactory := db.NewResourceFactory(dbConn, lockFactory)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	gcContainerDestroyer := gc.NewDestroyer(logger, dbContainerRepository, dbVolumeRepository)
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, cmd.GlobalResourceCheckTimeout)

	accessFactory := accessor.NewAccessFactory(authHandler.PublicKey())
	customActionRoleMap := accessor.CustomActionRoleMap{}
	err = accessor.ParseCustomActionRoleMap(cmd.ConfigRBAC, &customActionRoleMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RBAC config file (%s): %s", cmd.ConfigRBAC, err.Error())
	}
	err = accessFactory.CustomizeActionRoleMap(logger, customActionRoleMap)
	if err != nil {
		return nil, fmt.Errorf("failed to customize RBAC: %s", err.Error())
	}

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
	)

	if err != nil {
		return nil, err
	}

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

func (cmd *RunCommand) constructBackendMembers(
	logger lager.Logger,
	dbConn db.Conn,
	lockFactory lock.LockFactory,
	secretManager creds.Secrets,
) ([]grouper.Member, error) {

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

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(dbConn)
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
		resourceFetcher,
		image.NewImageFactory(imageResourceFetcherFactory),
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
	)

	pool := worker.NewPool(workerProvider)
	workerClient := worker.NewClient(pool, workerProvider)

	defaultLimits, err := cmd.parseDefaultLimits()
	if err != nil {
		return nil, err
	}

	buildContainerStrategy, err := cmd.chooseBuildContainerStrategy()
	if err != nil {
		return nil, err
	}
	checkContainerStrategy := worker.NewRandomPlacementStrategy()

	engine := cmd.constructEngine(
		pool,
		workerClient,
		resourceFactory,
		teamFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		secretManager,
		defaultLimits,
		buildContainerStrategy,
		lockFactory,
	)

	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		pool,
		dbResourceConfigFactory,
		cmd.ResourceTypeCheckingInterval,
		cmd.ResourceCheckingInterval,
		checkContainerStrategy,
	)

	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
	dbCheckFactory := db.NewCheckFactory(dbConn, lockFactory, secretManager, cmd.varSourcePool, cmd.GlobalResourceCheckTimeout)
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	componentFactory := db.NewComponentFactory(dbConn)

	err = cmd.configureComponentIntervals(componentFactory)
	if err != nil {
		return nil, err
	}

	bus := dbConn.Bus()

	members := []grouper.Member{
		{Name: "pipelines", Runner: pipelines.SyncRunner{
			Syncer: cmd.constructPipelineSyncer(
				logger.Session("pipelines"),
				dbPipelineFactory,
				componentFactory,
				radarSchedulerFactory,
				secretManager,
				bus,
			),
			Interval: runnerInterval,
			Clock:    clock.NewClock(),
		}},
		{Name: atc.ComponentBuildTracker, Runner: builds.NewRunner(
			logger.Session("tracker-runner"),
			clock.NewClock(),
			builds.NewTracker(
				logger.Session(atc.ComponentBuildTracker),
				dbBuildFactory,
				engine,
			),
			runnerInterval,
			bus,
			lockFactory,
			componentFactory,
		)},
		// run separately so as to not preempt critical GC
		{Name: atc.ComponentBuildReaper, Runner: lockrunner.NewRunner(
			logger.Session(atc.ComponentBuildReaper),
			gc.NewBuildLogCollector(
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
			atc.ComponentBuildReaper,
			lockFactory,
			componentFactory,
			clock.NewClock(),
			runnerInterval,
		)},
	}

	var lidarRunner ifrit.Runner

	if cmd.EnableLidar {
		lidarRunner = lidar.NewRunner(
			logger.Session("lidar"),
			clock.NewClock(),
			lidar.NewScanner(
				logger.Session(atc.ComponentLidarScanner),
				dbCheckFactory,
				secretManager,
				cmd.GlobalResourceCheckTimeout,
				cmd.ResourceCheckingInterval,
			),
			lidar.NewChecker(
				logger.Session(atc.ComponentLidarChecker),
				dbCheckFactory,
				engine,
			),
			runnerInterval,
			bus,
			lockFactory,
			componentFactory,
		)
	} else {
		lidarRunner = lidar.NewCheckerRunner(
			logger.Session("lidar"),
			clock.NewClock(),
			lidar.NewChecker(
				logger.Session(atc.ComponentLidarChecker),
				dbCheckFactory,
				engine,
			),
			runnerInterval,
			bus,
			lockFactory,
			componentFactory,
		)
	}

	members = append(members, grouper.Member{
		Name: "lidar", Runner: lidarRunner,
	})

	if syslogDrainConfigured {
		members = append(members, grouper.Member{
			Name: atc.ComponentSyslogDrainer, Runner: lockrunner.NewRunner(
				logger.Session(atc.ComponentSyslogDrainer),
				syslog.NewDrainer(
					cmd.Syslog.Transport,
					cmd.Syslog.Address,
					cmd.Syslog.Hostname,
					cmd.Syslog.CACerts,
					dbBuildFactory,
				),
				atc.ComponentSyslogDrainer,
				lockFactory,
				componentFactory,
				clock.NewClock(),
				runnerInterval,
			)},
		)
	}
	if cmd.Worker.GardenURL.URL != nil {
		members = cmd.appendStaticWorker(logger, dbWorkerFactory, members)
	}
	return members, nil
}

func (cmd *RunCommand) constructGCMember(
	logger lager.Logger,
	gcConn db.Conn,
	lockFactory lock.LockFactory,
) ([]grouper.Member, error) {

	var members []grouper.Member

	componentFactory := db.NewComponentFactory(gcConn)
	dbWorkerLifecycle := db.NewWorkerLifecycle(gcConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(gcConn)
	dbContainerRepository := db.NewContainerRepository(gcConn)
	dbArtifactLifecycle := db.NewArtifactLifecycle(gcConn)
	dbCheckLifecycle := db.NewCheckLifecycle(gcConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(gcConn)
	dbBuildFactory := db.NewBuildFactory(gcConn, lockFactory, cmd.GC.OneOffBuildGracePeriod)
	resourceFactory := resource.NewResourceFactory()
	dbResourceCacheFactory := db.NewResourceCacheFactory(gcConn, lockFactory)
	fetchSourceFactory := fetcher.NewFetchSourceFactory(dbResourceCacheFactory, resourceFactory)
	resourceFetcher := fetcher.NewFetcher(clock.NewClock(), lockFactory, fetchSourceFactory)
	dbResourceConfigFactory := db.NewResourceConfigFactory(gcConn, lockFactory)
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		resourceFetcher,
		resourceFactory,
	)

	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(gcConn)
	dbTaskCacheFactory := db.NewTaskCacheFactory(gcConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(gcConn)
	dbVolumeRepository := db.NewVolumeRepository(gcConn)
	dbWorkerFactory := db.NewWorkerFactory(gcConn)
	workerVersion, err := workerVersion()
	if err != nil {
		return members, err
	}

	teamFactory := db.NewTeamFactory(gcConn, lockFactory)
	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		image.NewImageFactory(imageResourceFetcherFactory),
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
	)

	jobRunner := gc.NewWorkerJobRunner(
		logger.Session("container-collector-worker-job-runner"),
		workerProvider,
		time.Minute,
	)

	collectors := map[string]lockrunner.Task{
		atc.ComponentCollectorBuilds:            gc.NewBuildCollector(dbBuildFactory),
		atc.ComponentCollectorWorkers:           gc.NewWorkerCollector(dbWorkerLifecycle),
		atc.ComponentCollectorResourceConfigs:   gc.NewResourceConfigCollector(dbResourceConfigFactory),
		atc.ComponentCollectorResourceCaches:    gc.NewResourceCacheCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorResourceCacheUses: gc.NewResourceCacheUseCollector(dbResourceCacheLifecycle),
		atc.ComponentCollectorArtifacts:         gc.NewArtifactCollector(dbArtifactLifecycle),
		atc.ComponentCollectorChecks:            gc.NewCheckCollector(dbCheckLifecycle, cmd.GC.CheckRecyclePeriod),
		atc.ComponentCollectorVolumes:           gc.NewVolumeCollector(dbVolumeRepository, cmd.GC.MissingGracePeriod),
		atc.ComponentCollectorContainers:        gc.NewContainerCollector(dbContainerRepository, jobRunner, cmd.GC.MissingGracePeriod),
		atc.ComponentCollectorCheckSessions:     gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle),
		atc.ComponentCollectorVarSources:        gc.NewCollectorTask(cmd.varSourcePool.(gc.Collector)),
	}

	for collectorName, collector := range collectors {
		members = append(members, grouper.Member{
			Name: collectorName, Runner: lockrunner.NewRunner(
				logger.Session(collectorName),
				collector,
				collectorName,
				lockFactory,
				componentFactory,
				clock.NewClock(),
				runnerInterval,
			)},
		)
	}

	return members, nil
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
		certpool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		if !cmd.LetsEncrypt.Enable {
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

	err = team.UpdateProviderAuth(atc.TeamAuth(auth))
	if err != nil {
		return err
	}

	return nil
}

func (cmd *RunCommand) configureComponentIntervals(componentFactory db.ComponentFactory) error {
	return componentFactory.UpdateIntervals(
		[]atc.Component{
			{
				Name:     atc.ComponentBuildTracker,
				Interval: cmd.BuildTrackerInterval,
			}, {
				Name:     atc.ComponentScheduler,
				Interval: 10 * time.Second,
			}, {
				Name:     atc.ComponentLidarChecker,
				Interval: cmd.LidarCheckerInterval,
			}, {
				Name:     atc.ComponentLidarScanner,
				Interval: cmd.LidarScannerInterval,
			}, {
				Name:     atc.ComponentBuildReaper,
				Interval: 30 * time.Second,
			}, {
				Name:     atc.ComponentSyslogDrainer,
				Interval: cmd.Syslog.DrainInterval,
			}, {
				Name:     atc.ComponentCollectorArtifacts,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorBuilds,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorChecks,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorCheckSessions,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorContainers,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorResourceCaches,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorResourceCacheUses,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorResourceConfigs,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorVolumes,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorWorkers,
				Interval: cmd.GC.Interval,
			}, {
				Name:     atc.ComponentCollectorVarSources,
				Interval: 60 * time.Second,
			},
		})
}

func (cmd *RunCommand) constructEngine(
	workerPool worker.Pool,
	workerClient worker.Client,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
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
		resourceCacheFactory,
		resourceConfigFactory,
		defaultLimits,
		strategy,
		lockFactory,
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
			Handler: auth.WebAuthHandler{
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
	dbCheckFactory db.CheckFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	dbUserFactory db.UserFactory,
	workerClient worker.Client,
	secretManager creds.Secrets,
	credsManagers creds.Managers,
	accessFactory accessor.AccessFactory,
) (http.Handler, error) {

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(teamFactory)
	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)
	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)
	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

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
	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewAPIAuthWrappa(
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
			checkWorkerTeamAccessHandlerFactory,
		),
		wrappa.NewConcourseVersionWrappa(concourse.Version),
		wrappa.NewAccessorWrappa(accessFactory, aud),
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
	componentFactory db.ComponentFactory,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	secretManager creds.Secrets,
	bus db.NotificationsBus,
) *pipelines.Syncer {
	return pipelines.NewSyncer(
		logger,
		pipelineFactory,
		componentFactory,
		func(pipeline db.Pipeline) ifrit.Runner {
			return grouper.NewParallel(os.Interrupt, grouper.Members{
				{
					Name: fmt.Sprintf("radar:%d", pipeline.ID()),
					Runner: radar.NewRunner(
						logger.Session("radar").WithData(lager.Data{
							"team":     pipeline.TeamName(),
							"pipeline": pipeline.Name(),
						}),
						(cmd.Developer.Noop || cmd.EnableLidar),
						radarSchedulerFactory.BuildScanRunnerFactory(pipeline, cmd.ExternalURL.String(), secretManager, cmd.varSourcePool, bus),
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
