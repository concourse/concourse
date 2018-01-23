package atccmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/containerserver"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/creds/noop"
	"github.com/concourse/atc/db"
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
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/image"
	"github.com/concourse/atc/wrappa"
	"github.com/concourse/retryhttp"
	"github.com/concourse/skymarshal"
	"github.com/concourse/web"
	"github.com/cppforlife/go-semi-semantic/version"
	jwt "github.com/dgrijalva/jwt-go"
	multierror "github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/xoebus/zest"

	"github.com/concourse/skymarshal/provider"

	// dynamically registered auth providers
	_ "github.com/concourse/skymarshal/bitbucket/cloud"
	_ "github.com/concourse/skymarshal/bitbucket/server"
	_ "github.com/concourse/skymarshal/genericoauth"
	_ "github.com/concourse/skymarshal/github"
	_ "github.com/concourse/skymarshal/gitlab"
	_ "github.com/concourse/skymarshal/uaa"

	// dynamically registered metric emitters
	_ "github.com/concourse/atc/metric/emitter"

	// dynamically registered credential managers
	_ "github.com/concourse/atc/creds/credhub"
	_ "github.com/concourse/atc/creds/kubernetes"
	_ "github.com/concourse/atc/creds/ssm"
	_ "github.com/concourse/atc/creds/vault"
)

var defaultDriverName = "postgres"
var retryingDriverName = "too-many-connections-retrying"

type ATCCommand struct {
	Migration Migration `group:"Migration Options"`

	Logger LagerFlag

	BindIP   IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16 `long:"bind-port" default:"8080"    description:"Port on which to listen for HTTP traffic."`

	TLSBindPort uint16   `long:"tls-bind-port" description:"Port on which to listen for HTTPS traffic."`
	TLSCert     FileFlag `long:"tls-cert"      description:"File containing an SSL certificate."`
	TLSKey      FileFlag `long:"tls-key"       description:"File containing an RSA private key, used to encrypt HTTPS traffic."`

	ExternalURL URLFlag `long:"external-url" default:"http://127.0.0.1:8080" description:"URL used to reach any ATC from the outside world."`
	PeerURL     URLFlag `long:"peer-url"     default:"http://127.0.0.1:8080" description:"URL used to reach this ATC from other ATCs in the cluster."`

	Authentication atc.AuthFlags `group:"Authentication"`
	ProviderAuth   provider.AuthConfigs

	AuthDuration time.Duration `long:"auth-duration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`
	OAuthBaseURL URLFlag       `long:"oauth-base-url" description:"URL used as the base of OAuth redirect URIs. If not specified, the external URL is used."`

	Postgres PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`

	CredentialManagement struct{} `group:"Credential Management"`
	CredentialManagers   creds.Managers

	EncryptionKey    CipherFlag `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	OldEncryptionKey CipherFlag `long:"old-encryption-key" description:"Encryption key previously used for encrypting sensitive information. If provided without a new key, data is encrypted. If provided with a new key, data is re-encrypted."`

	DebugBindIP   IPFlag `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16 `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	SessionSigningKey FileFlag `long:"session-signing-key" description:"File containing an RSA private key, used to sign session tokens."`

	InterceptIdleTimeout              time.Duration `long:"intercept-idle-timeout" default:"0m" description:"Length of time for a intercepted session to be idle before terminating."`
	ResourceCheckingInterval          time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	OldResourceGracePeriod            time.Duration `long:"old-resource-grace-period" default:"5m" description:"How long to cache the result of a get step after a newer version of the resource is found."`
	ResourceCacheCleanupInterval      time.Duration `long:"resource-cache-cleanup-interval" default:"30s" description:"Interval on which to cleanup old caches of resources."`
	ContainerPlacementStrategy        string        `long:"container-placement-strategy" default:"volume-locality" choice:"volume-locality" choice:"random" description:"Method by which a worker is selected during container placement."`
	BaggageclaimResponseHeaderTimeout time.Duration `long:"baggageclaim-response-header-timeout" default:"1m" description:"How long to wait for Baggageclaim to send the response header."`

	CLIArtifactsDir DirFlag `long:"cli-artifacts-dir" description:"Directory containing downloadable CLI binaries."`

	Developer struct {
		Noop bool `short:"n" long:"noop"              description:"Don't actually do any automatic scheduling or checking."`
	} `group:"Developer Options"`

	Worker struct {
		GardenURL       URLFlag           `long:"garden-url"       description:"A Garden API endpoint to register as a worker."`
		BaggageclaimURL URLFlag           `long:"baggageclaim-url" description:"A Baggageclaim API endpoint to register with the worker."`
		ResourceTypes   map[string]string `long:"resource"         description:"A resource type to advertise for the worker. Can be specified multiple times." value-name:"TYPE:IMAGE"`
	} `group:"Static Worker (optional)" namespace:"worker"`

	Metrics struct {
		HostName   string            `long:"metrics-host-name"   description:"Host string to attach to emitted metrics."`
		Attributes map[string]string `long:"metrics-attribute"   description:"A key-value attribute to attach to emitted metrics. Can be specified multiple times." value-name:"NAME:VALUE"`

		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`
	} `group:"Metrics & Diagnostics"`

	Server struct {
		XFrameOptions string `long:"x-frame-options" description:"The value to set for X-Frame-Options. If omitted, the header is not set."`
	} `group:"Web Server"`

	LogDBQueries bool `long:"log-db-queries" description:"Log database queries."`

	GC struct {
		Interval          time.Duration `long:"interval" default:"30s" description:"Interval on which to perform garbage collection."`
		WorkerConcurrency int           `long:"worker-concurrency" default:"50" description:"Maximum number of delete operations to have in flight per worker."`
	} `group:"Garbage Collection" namespace:"gc"`

	BuildTrackerInterval time.Duration `long:"build-tracker-interval" default:"10s" description:"Interval on which to run build tracking."`

	TelemetryOptIn bool `long:"telemetry-opt-in" hidden:"true" description:"Enable anonymous concourse version reporting."`
}

type Migration struct {
	CurrentDBVersion   bool `long:"current-db-version" description:"Print the current database version and exit"`
	SupportedDBVersion bool `long:"supported-db-version" description:"Print the max supported database version and exit"`
	MigrateDBToVersion int  `long:"migrate-db-to-version" description:"Migrate to the specified database version and exit"`
}

func (m *Migration) CommandProvided() bool {
	return m.CurrentDBVersion || m.SupportedDBVersion || m.MigrateDBToVersion > 0
}

func (cmd *ATCCommand) RunMigrationCommand() error {
	if cmd.Migration.CurrentDBVersion {
		return cmd.currentDBVersion()
	}
	if cmd.Migration.SupportedDBVersion {
		return cmd.supportedDBVersion()
	}
	if cmd.Migration.MigrateDBToVersion > 0 {
		return cmd.migrateDBToVersion()
	}
	return nil
}

func (cmd *ATCCommand) currentDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
	)

	version, err := helper.CurrentVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *ATCCommand) supportedDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
	)

	version, err := helper.SupportedVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *ATCCommand) migrateDBToVersion() error {
	version := cmd.Migration.MigrateDBToVersion

	lockConn, err := cmd.constructLockConn(defaultDriverName)
	if err != nil {
		return err
	}
	defer lockConn.Close()

	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		lock.NewLockFactory(lockConn),
	)

	err = helper.MigrateToVersion(version)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not migrate to version: %d Reason: %s", version, err.Error()))
	}

	fmt.Println("Successfully migrated to version:", version)
	return nil
}

func (cmd *ATCCommand) WireDynamicFlags(commandFlags *flags.Command) {
	var authGroup *flags.Group
	var metricsGroup *flags.Group
	var credsGroup *flags.Group

	groups := commandFlags.Groups()
	for i := 0; i < len(groups); i++ {
		group := groups[i]

		if authGroup == nil && group.ShortDescription == "Authentication" {
			authGroup = group
		}

		if credsGroup == nil && group.ShortDescription == "Credential Management" {
			credsGroup = group
		}

		if metricsGroup == nil && group.ShortDescription == "Metrics & Diagnostics" {
			metricsGroup = group
		}

		if metricsGroup != nil && authGroup != nil && credsGroup != nil {
			break
		}

		groups = append(groups, group.Groups()...)
	}

	if authGroup == nil {
		panic("could not find Authentication group for registering providers")
	}

	if metricsGroup == nil {
		panic("could not find Metrics & Diagnostics group for registering emitters")
	}

	if credsGroup == nil {
		panic("could not find Credential Management group for registering managers")
	}

	authConfigs := make(provider.AuthConfigs)
	for name, p := range provider.GetProviders() {
		authConfigs[name] = p.AddAuthGroup(authGroup)
	}
	cmd.ProviderAuth = authConfigs

	managerConfigs := make(creds.Managers)
	for name, p := range creds.ManagerFactories() {
		managerConfigs[name] = p.AddConfig(credsGroup)
	}
	cmd.CredentialManagers = managerConfigs

	metric.WireEmitters(metricsGroup)

}

func (cmd *ATCCommand) Execute(args []string) error {

	if cmd.Migration.CommandProvided() {
		return cmd.RunMigrationCommand()
	}

	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *ATCCommand) constructMembers(
	positionalArguments []string,
	requiredMemberNames []string,
	maxConns int,
	connectionName string,
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

	var workerVersion *version.Version
	if len(WorkerVersion) != 0 {
		version, err := version.NewVersionFromString(WorkerVersion)
		if err != nil {
			return nil, err
		}

		workerVersion = &version
	}

	var variablesFactory creds.VariablesFactory = noop.NewNoopFactory()
	for name, manager := range cmd.CredentialManagers {
		if !manager.IsConfigured() {
			continue
		}

		err := manager.Validate()
		if err != nil {
			return nil, fmt.Errorf("credential manager '%s' misconfigured: %s", name, err)
		}

		variablesFactory, err = manager.NewVariablesFactory(logger.Session("credential-manager", lager.Data{
			"name": name,
		}))
		if err != nil {
			return nil, err
		}

		break
	}

	var newKey *db.EncryptionKey
	if cmd.EncryptionKey.AEAD != nil {
		newKey = db.NewEncryptionKey(cmd.EncryptionKey.AEAD)
	}

	var oldKey *db.EncryptionKey
	if cmd.OldEncryptionKey.AEAD != nil {
		oldKey = db.NewEncryptionKey(cmd.OldEncryptionKey.AEAD)
	}

	lockConn, err := cmd.constructLockConn(retryingDriverName)
	if err != nil {
		return nil, err
	}

	lockFactory := lock.NewLockFactory(lockConn)

	dbConn, err := cmd.constructDBConn(retryingDriverName, logger, newKey, oldKey, maxConns, connectionName, lockFactory)
	if err != nil {
		return nil, err
	}

	bus := dbConn.Bus()

	teamFactory := db.NewTeamFactory(dbConn, lockFactory)
	resourceFactoryFactory := resource.NewResourceFactoryFactory()
	dbBuildFactory := db.NewBuildFactory(dbConn, lockFactory)
	dbVolumeFactory := db.NewVolumeFactory(dbConn)
	dbContainerRepository := db.NewContainerRepository(dbConn)
	dbPipelineFactory := db.NewPipelineFactory(dbConn, lockFactory)
	dbWorkerFactory := db.NewWorkerFactory(dbConn)
	dbWorkerLifecycle := db.NewWorkerLifecycle(dbConn)
	resourceConfigCheckSessionLifecycle := db.NewResourceConfigCheckSessionLifecycle(dbConn)
	dbResourceCacheFactory := db.NewResourceCacheFactory(dbConn)
	dbResourceCacheLifecycle := db.NewResourceCacheLifecycle(dbConn)
	dbResourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
	dbResourceConfigCheckSessionFactory := db.NewResourceConfigCheckSessionFactory(dbConn, lockFactory)
	dbWorkerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
	dbWorkerTaskCacheFactory := db.NewWorkerTaskCacheFactory(dbConn)
	resourceFetcherFactory := resource.NewFetcherFactory(lockFactory, clock.NewClock(), dbResourceCacheFactory)

	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFetcherFactory,
		resourceFactoryFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		clock.NewClock(),
	)

	workerProvider := worker.NewDBWorkerProvider(
		lockFactory,
		retryhttp.NewExponentialBackOffFactory(5*time.Minute),
		image.NewImageFactory(imageResourceFetcherFactory),
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbWorkerTaskCacheFactory,
		dbVolumeFactory,
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
	resourceFactory := resourceFactoryFactory.FactoryFor(workerClient)
	engine := cmd.constructEngine(workerClient, resourceFetcher, resourceFactory, dbResourceCacheFactory, variablesFactory)

	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		resourceFactory,
		dbResourceConfigCheckSessionFactory,
		cmd.ResourceCheckingInterval,
		engine,
	)

	radarScannerFactory := radar.NewScannerFactory(
		resourceFactory,
		dbResourceConfigCheckSessionFactory,
		cmd.ResourceCheckingInterval,
		cmd.ExternalURL.String(),
		variablesFactory,
	)

	signingKey, err := cmd.loadOrGenerateSigningKey()
	if err != nil {
		return nil, err
	}

	_, err = teamFactory.CreateDefaultTeamIfNotExists()
	if err != nil {
		return nil, err
	}

	err = cmd.configureAuthForDefaultTeam(teamFactory)
	if err != nil {
		return nil, err
	}

	drain := make(chan struct{})

	apiHandler, err := cmd.constructAPIHandler(
		logger,
		reconfigurableSink,
		teamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		dbVolumeFactory,
		dbContainerRepository,
		dbBuildFactory,
		signingKey,
		engine,
		workerClient,
		workerProvider,
		drain,
		radarSchedulerFactory,
		radarScannerFactory,
		variablesFactory,
	)

	if err != nil {
		return nil, err
	}

	authHandler, err := skymarshal.NewHandler(&skymarshal.Config{
		cmd.ExternalURL.String(),
		cmd.oauthBaseURL(),
		signingKey,
		cmd.AuthDuration,
		cmd.isTLSEnabled(),
		teamFactory,
		logger,
	})

	if err != nil {
		return nil, err
	}

	webHandler, err := web.NewHandler(logger)
	if err != nil {
		return nil, err
	}

	webHandler = metric.WrapHandler(logger, "web", webHandler)

	var httpHandler, httpsHandler http.Handler
	if cmd.isTLSEnabled() {
		httpHandler = cmd.constructHTTPHandler(
			logger,

			tlsRedirectHandler{
				externalHost: cmd.ExternalURL.URL().Host,
				baseHandler:  webHandler,
			},

			// note: intentionally not wrapping API; redirecting is more trouble than
			// it's worth.

			// we're mainly interested in having the web UI consistently https:// -
			// API requests will likely not respect the redirected https:// URI upon
			// the next request, plus the payload will have already been sent in
			// plaintext
			apiHandler,

			tlsRedirectHandler{
				externalHost: cmd.ExternalURL.URL().Host,
				baseHandler:  authHandler,
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
		{"drainer", drainer{
			logger: logger.Session("drain"),
			drain:  drain,
			tracker: builds.NewTracker(
				logger.Session("build-tracker"),
				dbBuildFactory,
				engine,
			),
			bus: bus,
		}},

		{"debug", http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},

		{"pipelines", pipelines.SyncRunner{
			Syncer: cmd.constructPipelineSyncer(
				logger.Session("syncer"),
				dbPipelineFactory,
				radarSchedulerFactory,
				variablesFactory,
			),
			Interval: 10 * time.Second,
			Clock:    clock.NewClock(),
		}},

		{"builds", builds.TrackerRunner{
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

		{"collector", lockrunner.NewRunner(
			logger.Session("collector-runner"),
			gc.NewCollector(
				logger.Session("ng-collector"),
				gc.NewBuildCollector(
					logger.Session("build-collector"),
					dbBuildFactory,
				),
				gc.NewWorkerCollector(
					logger.Session("worker-collector"),
					dbWorkerLifecycle,
				),
				gc.NewResourceCacheUseCollector(
					logger.Session("resource-cache-use-collector"),
					dbResourceCacheLifecycle,
				),
				gc.NewResourceConfigCollector(
					logger.Session("resource-config-collector"),
					dbResourceConfigFactory,
				),
				gc.NewResourceCacheCollector(
					logger.Session("resource-cache-collector"),
					dbResourceCacheLifecycle,
				),
				gc.NewVolumeCollector(
					logger.Session("volume-collector"),
					dbVolumeFactory,
					gc.NewWorkerJobRunner(
						logger.Session("volume-collector-worker-job-runner"),
						workerClient,
						time.Minute,
						cmd.GC.WorkerConcurrency,
						func(logger lager.Logger, workerName string) {
							metric.GarbageCollectionVolumeCollectorJobDropped{
								WorkerName: workerName,
							}.Emit(logger)
						},
					),
				),
				gc.NewContainerCollector(
					logger.Session("container-collector"),
					dbContainerRepository,
					gc.NewWorkerJobRunner(
						logger.Session("container-collector-worker-job-runner"),
						workerClient,
						time.Minute,
						cmd.GC.WorkerConcurrency,
						func(logger lager.Logger, workerName string) {
							metric.GarbageCollectionContainerCollectorJobDropped{
								WorkerName: workerName,
							}.Emit(logger)
						},
					),
				),
				gc.NewResourceConfigCheckSessionCollector(
					logger.Session("resource-config-check-session-collector"),
					resourceConfigCheckSessionLifecycle,
				),
			),
			"collector",
			lockFactory,
			clock.NewClock(),
			cmd.GC.Interval,
		)},

		{"build-reaper", lockrunner.NewRunner(
			logger.Session("build-reaper-runner"),
			gc.NewBuildReaper(
				logger.Session("build-reaper"),
				dbPipelineFactory,
				500,
			),
			"build-reaper",
			lockFactory,
			clock.NewClock(),
			30*time.Second,
		)},
	}

	if cmd.TelemetryOptIn {
		url := fmt.Sprintf("http://telemetry.concourse.ci/?version=%s", Version)
		go func() {
			_, err := http.Get(url)
			if err != nil {
				logger.Error("telemetry-version", err)
			}
		}()
	}

	if cmd.Worker.GardenURL.URL() != nil {
		members = cmd.appendStaticWorker(logger, dbWorkerFactory, members)
	}

	if httpsHandler != nil {
		cert, err := tls.LoadX509KeyPair(string(cmd.TLSCert), string(cmd.TLSKey))
		if err != nil {
			return nil, err
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2"},
		}

		members = append(members, grouper.Member{"web-tls", http_server.NewTLSServer(
			cmd.tlsBindAddr(),
			httpsHandler,
			tlsConfig,
		)})
	}

	members = append(members, grouper.Member{"web", http_server.New(
		cmd.nonTLSBindAddr(),
		httpHandler,
	)})

	var filteredMembers []grouper.Member
	for _, member := range members {
		for _, requiredMemberName := range requiredMemberNames {
			if member.Name == requiredMemberName {
				filteredMembers = append(filteredMembers, member)
				break
			}
		}
	}

	return filteredMembers, nil
}

func (cmd *ATCCommand) Runner(positionalArguments []string) (ifrit.Runner, error) {
	var members []grouper.Member

	//FIXME: These only need to run once for the entire binary. At the moment,
	//they rely on state of the command.
	db.SetupConnectionRetryingDriver("postgres", cmd.Postgres.ConnectionString(), retryingDriverName)
	logger, reconfigurableSink := cmd.constructLogger()
	http.HandleFunc("/debug/connections", func(w http.ResponseWriter, r *http.Request) {
		for _, stack := range db.GlobalConnectionTracker.Current() {
			fmt.Fprintln(w, stack)
		}
	})

	if err := cmd.configureMetrics(logger); err != nil {
		return nil, err
	}
	go metric.PeriodicallyEmit(logger.Session("periodic-metrics"), 10*time.Second)

	apiMembers, err := cmd.constructMembers(positionalArguments, []string{
		"debug",
		"web-tls",
		"web",
	},
		32,
		"api",
		logger,
		reconfigurableSink,
	)
	if err != nil {
		return nil, err
	}

	serviceMembers, err := cmd.constructMembers(positionalArguments, []string{
		"drainer",
		"pipelines",
		"builds",
		"collector",
		"build-reaper",
		"static-worker",
	},
		32,
		"backend",
		logger,
		reconfigurableSink,
	)
	if err != nil {
		return nil, err
	}

	members = append(members, apiMembers...)
	members = append(members, serviceMembers...)

	return onReady(grouper.NewParallel(os.Interrupt, members), func() {
		logData := lager.Data{
			"http":  cmd.nonTLSBindAddr(),
			"debug": cmd.debugBindAddr(),
		}

		if cmd.isTLSEnabled() {
			logData["https"] = cmd.tlsBindAddr()
		}

		logger.Info("listening", logData)
	}), nil
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
				subReady = nil
			case err := <-subExited:
				return err
			case sig := <-signals:
				process.Signal(sig)
			}
		}
	})
}

func (cmd *ATCCommand) oauthBaseURL() string {
	baseURL := cmd.OAuthBaseURL.String()
	if baseURL == "" {
		baseURL = cmd.ExternalURL.String()
	}
	return baseURL
}

func (cmd *ATCCommand) validate() error {
	var errs *multierror.Error
	isConfigured := false

	for _, p := range cmd.ProviderAuth {
		if p.IsConfigured() {
			err := p.Validate()

			if err != nil {
				errs = multierror.Append(errs, err)
			}

			isConfigured = true
		}
	}

	if cmd.Authentication.BasicAuth.IsConfigured() {
		err := cmd.Authentication.BasicAuth.Validate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		isConfigured = true
	}

	if !isConfigured && !cmd.Authentication.NoAuth {
		errs = multierror.Append(
			errs,
			errors.New("must configure basic auth, OAuth, UAAAuth, or provide no-auth flag"),
		)
	}

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
		if cmd.ExternalURL.URL().Scheme != "https" {
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

func (cmd *ATCCommand) nonTLSBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
}

func (cmd *ATCCommand) tlsBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.TLSBindPort)
}

func (cmd *ATCCommand) debugBindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.DebugBindIP, cmd.DebugBindPort)
}

func (cmd *ATCCommand) constructLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cmd.Logger.Logger("atc")

	if cmd.Metrics.YellerAPIKey != "" {
		yellerSink := zest.NewYellerSink(cmd.Metrics.YellerAPIKey, cmd.Metrics.YellerEnvironment)
		logger.RegisterSink(yellerSink)
	}

	return logger, reconfigurableSink
}

func (cmd *ATCCommand) configureMetrics(logger lager.Logger) error {
	host := cmd.Metrics.HostName
	if host == "" {
		host, _ = os.Hostname()
	}

	return metric.Initialize(logger.Session("metrics"), host, cmd.Metrics.Attributes)
}

func (cmd *ATCCommand) constructDBConn(
	driverName string,
	logger lager.Logger,
	newKey *db.EncryptionKey,
	oldKey *db.EncryptionKey,
	maxConn int,
	connectionName string,
	lockFactory lock.LockFactory,
) (db.Conn, error) {
	dbConn, err := db.Open(logger.Session("db"), driverName, cmd.Postgres.ConnectionString(), newKey, oldKey, connectionName, lockFactory)
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

func (cmd *ATCCommand) constructLockConn(driverName string) (*sql.DB, error) {
	dbConn, err := sql.Open(driverName, cmd.Postgres.ConnectionString())
	if err != nil {
		return nil, err
	}

	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)
	dbConn.SetConnMaxLifetime(0)

	return dbConn, nil
}

func (cmd *ATCCommand) constructWorkerPool(
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

func (cmd *ATCCommand) loadOrGenerateSigningKey() (*rsa.PrivateKey, error) {
	var signingKey *rsa.PrivateKey

	if cmd.SessionSigningKey == "" {
		generatedKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate session signing key: %s", err)
		}

		signingKey = generatedKey
	} else {
		rsaKeyBlob, err := ioutil.ReadFile(string(cmd.SessionSigningKey))
		if err != nil {
			return nil, fmt.Errorf("failed to read session signing key file: %s", err)
		}

		signingKey, err = jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
		if err != nil {
			return nil, fmt.Errorf("failed to parse session signing key as RSA: %s", err)
		}
	}

	return signingKey, nil
}

func (cmd *ATCCommand) configureAuthForDefaultTeam(teamFactory db.TeamFactory) error {
	team, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("default team not found")
	}

	var basicAuth *atc.BasicAuth
	if cmd.Authentication.BasicAuth.IsConfigured() {
		basicAuth = &atc.BasicAuth{
			BasicAuthUsername: cmd.Authentication.BasicAuth.Username,
			BasicAuthPassword: cmd.Authentication.BasicAuth.Password,
		}
	}

	err = team.UpdateBasicAuth(basicAuth)
	if err != nil {
		return err
	}

	teamAuth := make(map[string]*json.RawMessage)
	for name, config := range cmd.ProviderAuth {
		if config.IsConfigured() {
			data, err := json.Marshal(config)
			if err != nil {
				return err
			}

			teamAuth[name] = (*json.RawMessage)(&data)
		}
	}

	err = team.UpdateProviderAuth(teamAuth)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *ATCCommand) constructEngine(
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	variablesFactory creds.VariablesFactory,
) engine.Engine {
	gardenFactory := exec.NewGardenFactory(
		workerClient,
		resourceFetcher,
		resourceFactory,
		dbResourceCacheFactory,
		variablesFactory,
	)

	execV2Engine := engine.NewExecEngine(
		gardenFactory,
		engine.NewBuildDelegateFactory(),
		cmd.ExternalURL.String(),
	)

	execV1Engine := engine.NewExecV1DummyEngine()

	return engine.NewDBEngine(engine.Engines{execV2Engine, execV1Engine})
}

func (cmd *ATCCommand) constructHTTPHandler(
	logger lager.Logger,
	webHandler http.Handler,
	apiHandler http.Handler,
	authHandler http.Handler,
) http.Handler {
	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", apiHandler)
	webMux.Handle("/oauth/", authHandler)
	webMux.Handle("/auth/", authHandler)
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

func (cmd *ATCCommand) constructAPIHandler(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	teamFactory db.TeamFactory,
	dbPipelineFactory db.PipelineFactory,
	dbWorkerFactory db.WorkerFactory,
	dbVolumeFactory db.VolumeFactory,
	dbContainerRepository db.ContainerRepository,
	dbBuildFactory db.BuildFactory,
	signingKey *rsa.PrivateKey,
	engine engine.Engine,
	workerClient worker.Client,
	workerProvider worker.WorkerProvider,
	drain <-chan struct{},
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	radarScannerFactory radar.ScannerFactory,
	variablesFactory creds.VariablesFactory,
) (http.Handler, error) {

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(teamFactory)
	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)
	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)
	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewAPIAuthWrappa(
			auth.JWTValidator{PublicKey: &signingKey.PublicKey},
			auth.JWTReader{PublicKey: &signingKey.PublicKey},
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

		cmd.oauthBaseURL(),

		teamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		dbVolumeFactory,
		dbContainerRepository,
		dbBuildFactory,

		cmd.PeerURL.String(),
		buildserver.NewEventHandler,
		drain,

		engine,
		workerClient,
		workerProvider,
		radarSchedulerFactory,
		radarScannerFactory,

		reconfigurableSink,

		cmd.AuthDuration,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		Version,
		WorkerVersion,
		variablesFactory,
		containerserver.NewInterceptTimeoutFactory(cmd.InterceptIdleTimeout),
	)
}

type tlsRedirectHandler struct {
	externalHost string
	baseHandler  http.Handler
}

func (h tlsRedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" || r.Method == "HEAD" {
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

func (cmd *ATCCommand) constructPipelineSyncer(
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

func (cmd *ATCCommand) appendStaticWorker(
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
				cmd.Worker.GardenURL.URL().Host,
				cmd.Worker.BaggageclaimURL.String(),
				resourceTypes,
			),
		},
	)
}

func (cmd *ATCCommand) isTLSEnabled() bool {
	return cmd.TLSBindPort != 0
}

func init() {
}
