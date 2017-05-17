package atccmd

import (
	"crypto/aes"
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
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/gc/buildreaper"
	"github.com/concourse/atc/gcng"
	"github.com/concourse/atc/lockrunner"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/publichandler"
	"github.com/concourse/atc/web/robotstxt"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/image"
	"github.com/concourse/atc/wrappa"
	"github.com/concourse/retryhttp"
	"github.com/cppforlife/go-semi-semantic/version"
	jwt "github.com/dgrijalva/jwt-go"
	multierror "github.com/hashicorp/go-multierror"
	flags "github.com/jessevdk/go-flags"
	"github.com/lib/pq"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/xoebus/zest"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/routes"

	// dynamically registered auth providers
	_ "github.com/concourse/atc/auth/genericoauth"
	_ "github.com/concourse/atc/auth/github"
	_ "github.com/concourse/atc/auth/uaa"

	// dynamically registered metric emitters
	_ "github.com/concourse/atc/metric/emitter"
)

type ATCCommand struct {
	Logger LagerFlag

	Authentication atc.AuthFlags `group:"Authentication"`

	// populated by main.go as providers inject their flags dynamically
	ProviderAuth map[string]provider.AuthConfig

	BindIP   IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16 `long:"bind-port" default:"8080"    description:"Port on which to listen for HTTP traffic."`

	TLSBindPort uint16   `long:"tls-bind-port" description:"Port on which to listen for HTTPS traffic."`
	TLSCert     FileFlag `long:"tls-cert"      description:"File containing an SSL certificate."`
	TLSKey      FileFlag `long:"tls-key"       description:"File containing an RSA private key, used to encrypt HTTPS traffic."`

	ExternalURL URLFlag `long:"external-url" default:"http://127.0.0.1:8080" description:"URL used to reach any ATC from the outside world."`
	PeerURL     URLFlag `long:"peer-url"     default:"http://127.0.0.1:8080" description:"URL used to reach this ATC from other ATCs in the cluster."`

	OAuthBaseURL URLFlag `long:"oauth-base-url" description:"URL used as the base of OAuth redirect URIs. If not specified, the external URL is used."`

	AuthDuration time.Duration `long:"auth-duration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`

	EncryptionKey string `long:"encryption-key" description:"16 or 32 byte AES key used to encrypt pipeline config and team auth before storing it into the database."`

	Postgres PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`

	DebugBindIP   IPFlag `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16 `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	SessionSigningKey FileFlag `long:"session-signing-key" description:"File containing an RSA private key, used to sign session tokens."`

	ResourceCheckingInterval     time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	OldResourceGracePeriod       time.Duration `long:"old-resource-grace-period" default:"5m" description:"How long to cache the result of a get step after a newer version of the resource is found."`
	ResourceCacheCleanupInterval time.Duration `long:"resource-cache-cleanup-interval" default:"30s" description:"Interval on which to cleanup old caches of resources."`

	CLIArtifactsDir DirFlag `long:"cli-artifacts-dir" description:"Directory containing downloadable CLI binaries."`

	Developer struct {
		Noop bool `short:"n" long:"noop"              description:"Don't actually do any automatic scheduling or checking."`
	} `group:"Developer Options"`

	AllowSelfSignedCertificates bool `long:"allow-self-signed-certificates" description:"Allow self signed certificates."`

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

	GCInterval time.Duration `long:"gc-interval" default:"30s" description:"Interval on which to perform garbage collection."`

	BuildTrackerInterval time.Duration `long:"build-tracker-interval" default:"10s" description:"Interval on which to run build tracking."`
}

func (cmd *ATCCommand) WireDynamicFlags(commandFlags *flags.Command) {
	var authGroup *flags.Group
	var metricsGroup *flags.Group

	groups := commandFlags.Groups()
	for i := 0; i < len(groups); i++ {
		group := groups[i]

		if authGroup == nil && group.ShortDescription == "Authentication" {
			authGroup = group
		}

		if metricsGroup == nil && group.ShortDescription == "Metrics & Diagnostics" {
			metricsGroup = group
		}

		if metricsGroup != nil && authGroup != nil {
			break
		}

		groups = append(groups, group.Groups()...)
	}

	if authGroup == nil {
		panic("could not find Authentication group for registering providers")
	}

	if metricsGroup == nil {
		panic("could not find Metrics & Diagnostics group for registering providers")
	}

	authConfigs := make(provider.AuthConfigs)

	for name, p := range provider.GetProviders() {
		authConfigs[name] = p.AddAuthGroup(authGroup)
	}

	cmd.ProviderAuth = authConfigs

	metric.WireEmitters(metricsGroup)
}

func (cmd *ATCCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *ATCCommand) Runner(args []string) (ifrit.Runner, error) {
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

	logger, reconfigurableSink := cmd.constructLogger()

	go metric.PeriodicallyEmit(logger.Session("periodic-metrics"), 10*time.Second)

	cmd.configureMetrics(logger)

	connectionCountingDriverName := "connection-counting"
	metric.SetupConnectionCountingDriver("postgres", cmd.Postgres.ConnectionString(), connectionCountingDriverName)

	retryingDriverName := "too-many-connections-retrying"
	dbng.SetupConnectionRetryingDriver(connectionCountingDriverName, cmd.Postgres.ConnectionString(), retryingDriverName)

	dbConn, dbngConn, err := cmd.constructDBConn(retryingDriverName, logger)
	if err != nil {
		return nil, err
	}

	if cmd.LogDBQueries {
		dbConn = db.Log(logger.Session("log-conn"), dbConn)
	}

	lockConn, err := cmd.constructLockConn(retryingDriverName)
	if err != nil {
		return nil, err
	}

	lockFactory := lock.NewLockFactory(lockConn)

	listener := pq.NewListener(cmd.Postgres.ConnectionString(), time.Second, time.Minute, nil)
	bus := db.NewNotificationsBus(listener, dbConn)

	var strategy dbng.EncryptionStrategy
	if cmd.EncryptionKey != "" {
		block, err := aes.NewCipher([]byte(cmd.EncryptionKey))
		if err != nil {
			return nil, err
		}
		strategy = dbng.NewEncryptionKey(block)
	} else {
		strategy = dbng.NewNoEncryption()
	}
	dbTeamFactory := dbng.NewTeamFactory(dbngConn, lockFactory, strategy)
	sqlDB := db.NewSQL(dbConn, bus, lockFactory)
	resourceFactoryFactory := resource.NewResourceFactoryFactory()
	pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, lockFactory)
	dbBuildFactory := dbng.NewBuildFactory(dbngConn, lockFactory, strategy)
	dbVolumeFactory := dbng.NewVolumeFactory(dbngConn)
	dbContainerFactory := dbng.NewContainerFactory(dbngConn)
	dbPipelineFactory := dbng.NewPipelineFactory(dbngConn, lockFactory, strategy)
	dbWorkerFactory := dbng.NewWorkerFactory(dbngConn)
	dbWorkerLifecycle := dbng.NewWorkerLifecycle(dbngConn)
	dbResourceCacheFactory := dbng.NewResourceCacheFactory(dbngConn, lockFactory)
	dbResourceConfigFactory := dbng.NewResourceConfigFactory(dbngConn, lockFactory)
	dbWorkerBaseResourceTypeFactory := dbng.NewWorkerBaseResourceTypeFactory(dbngConn)
	resourceFetcherFactory := resource.NewFetcherFactory(sqlDB, clock.NewClock(), dbResourceCacheFactory)
	workerClient := cmd.constructWorkerPool(
		logger,
		sqlDB,
		resourceFetcherFactory,
		resourceFactoryFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		dbWorkerBaseResourceTypeFactory,
		dbVolumeFactory,
		dbWorkerFactory,
		dbTeamFactory,
		workerVersion,
	)

	resourceFetcher := resourceFetcherFactory.FetcherFor(workerClient)
	resourceFactory := resourceFactoryFactory.FactoryFor(workerClient)
	teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
	engine := cmd.constructEngine(workerClient, resourceFetcher, resourceFactory, dbResourceCacheFactory, teamDBFactory)

	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		resourceFactory,
		cmd.ResourceCheckingInterval,
		engine,
	)

	radarScannerFactory := radar.NewScannerFactory(
		resourceFactory,
		cmd.ResourceCheckingInterval,
		cmd.ExternalURL.String(),
	)

	signingKey, err := cmd.loadOrGenerateSigningKey()
	if err != nil {
		return nil, err
	}

	err = sqlDB.CreateDefaultTeamIfNotExists()
	if err != nil {
		return nil, err
	}

	err = cmd.configureAuthForDefaultTeam(dbTeamFactory)
	if err != nil {
		return nil, err
	}

	providerFactory := auth.NewOAuthFactory(
		logger.Session("oauth-provider-factory"),
		cmd.oauthBaseURL(),
		routes.OAuthRoutes,
		routes.OAuthCallback,
	)
	if err != nil {
		return nil, err
	}

	drain := make(chan struct{})

	apiHandler, err := cmd.constructAPIHandler(
		logger,
		reconfigurableSink,
		sqlDB,
		teamDBFactory,
		dbTeamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		dbVolumeFactory,
		dbContainerFactory,
		dbBuildFactory,
		providerFactory,
		signingKey,
		pipelineDBFactory,
		engine,
		workerClient,
		drain,
		radarSchedulerFactory,
		radarScannerFactory,
	)

	if err != nil {
		return nil, err
	}

	oauthHandler, err := auth.NewOAuthHandler(
		logger,
		providerFactory,
		dbTeamFactory,
		signingKey,
		cmd.AuthDuration,
		cmd.isTLSEnabled(),
	)
	if err != nil {
		return nil, err
	}

	webHandler, err := web.NewHandler(logger)
	if err != nil {
		return nil, err
	}
	webHandler = metric.WrapHandler(logger, "web", webHandler)

	publicHandler, err := publichandler.NewHandler()
	if err != nil {
		return nil, err
	}

	var httpHandler, httpsHandler http.Handler
	if cmd.isTLSEnabled() {
		httpHandler = cmd.constructHTTPHandler(
			logger,
			tlsRedirectHandler{
				externalHost: cmd.ExternalURL.URL().Host,
				baseHandler:  webHandler,
			},
			tlsRedirectHandler{
				externalHost: cmd.ExternalURL.URL().Host,
				baseHandler:  publicHandler,
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
				baseHandler:  oauthHandler,
			},
		)

		httpsHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			publicHandler,
			apiHandler,
			oauthHandler,
		)
	} else {
		httpHandler = cmd.constructHTTPHandler(
			logger,
			webHandler,
			publicHandler,
			apiHandler,
			oauthHandler,
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
				sqlDB,
				pipelineDBFactory,
				dbPipelineFactory,
				radarSchedulerFactory,
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
			gcng.NewCollector(
				logger.Session("ng-collector"),
				gcng.NewBuildCollector(
					logger.Session("build-collector"),
					dbBuildFactory,
				),
				gcng.NewWorkerCollector(
					logger.Session("worker-collector"),
					dbWorkerLifecycle,
				),
				gcng.NewResourceCacheUseCollector(
					logger.Session("resource-cache-use-collector"),
					dbResourceCacheFactory,
				),
				gcng.NewResourceConfigUseCollector(
					logger.Session("resource-config-use-collector"),
					dbResourceConfigFactory,
				),
				gcng.NewResourceConfigCollector(
					logger.Session("resource-config-collector"),
					dbResourceConfigFactory,
				),
				gcng.NewResourceCacheCollector(
					logger.Session("resource-cache-collector"),
					dbResourceCacheFactory,
				),
				gcng.NewVolumeCollector(
					logger.Session("volume-collector"),
					dbVolumeFactory,
					gcng.NewBaggageclaimClientFactory(dbWorkerFactory),
				),
				gcng.NewContainerCollector(
					logger.Session("container-collector"),
					dbContainerFactory,
					dbWorkerFactory,
					gcng.NewGardenClientFactory(),
				),
			),
			"collector",
			sqlDB,
			clock.NewClock(),
			cmd.GCInterval,
		)},

		{"build-reaper", lockrunner.NewRunner(
			logger.Session("build-reaper-runner"),
			buildreaper.NewBuildReaper(
				logger.Session("build-reaper"),
				sqlDB,
				pipelineDBFactory,
				500,
			),
			"build-reaper",
			sqlDB,
			clock.NewClock(),
			30*time.Second,
		)},
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

func (cmd *ATCCommand) internalURL() string {
	if cmd.isTLSEnabled() {
		if strings.Contains(cmd.ExternalURL.String(), ":") {
			return cmd.ExternalURL.String()
		} else {
			return fmt.Sprintf("%s:%d", cmd.ExternalURL, cmd.TLSBindPort)
		}
	} else {
		return fmt.Sprintf("http://127.0.0.1:%d", cmd.BindPort)
	}
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

func (cmd *ATCCommand) configureMetrics(logger lager.Logger) {
	host := cmd.Metrics.HostName
	if host == "" {
		host, _ = os.Hostname()
	}

	metric.Initialize(logger.Session("metrics"), host, cmd.Metrics.Attributes)
}

func (cmd *ATCCommand) constructDBConn(driverName string, logger lager.Logger) (db.Conn, dbng.Conn, error) {
	dbngConn, err := dbng.Open(logger.Session("db"), driverName, cmd.Postgres.ConnectionString())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	dbConn, err := migrations.LockDBAndMigrate(logger.Session("db.migrations"), driverName, cmd.Postgres.ConnectionString())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	dbConn.SetMaxOpenConns(64)
	dbngConn.SetMaxOpenConns(64)

	return metric.CountQueries(dbConn), dbngConn, nil
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
	sqlDB *db.SQLDB,
	resourceFetcherFactory resource.FetcherFactory,
	resourceFactoryFactory resource.ResourceFactoryFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	dbWorkerBaseResourceTypeFactory dbng.WorkerBaseResourceTypeFactory,
	dbVolumeFactory dbng.VolumeFactory,
	dbWorkerFactory dbng.WorkerFactory,
	dbTeamFactory dbng.TeamFactory,
	workerVersion *version.Version,
) worker.Client {
	imageResourceFetcherFactory := image.NewImageResourceFetcherFactory(
		resourceFetcherFactory,
		resourceFactoryFactory,
		dbResourceCacheFactory,
		dbResourceConfigFactory,
		clock.NewClock(),
	)
	return worker.NewPool(
		worker.NewDBWorkerProvider(
			sqlDB,
			retryhttp.NewExponentialBackOffFactory(5*time.Minute),
			image.NewImageFactory(imageResourceFetcherFactory),
			dbResourceCacheFactory,
			dbResourceConfigFactory,
			dbWorkerBaseResourceTypeFactory,
			dbVolumeFactory,
			dbTeamFactory,
			dbWorkerFactory,
			workerVersion,
		),
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

func (cmd *ATCCommand) configureAuthForDefaultTeam(teamFactory dbng.TeamFactory) error {
	team, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
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
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	teamDBFactory db.TeamDBFactory,
) engine.Engine {
	gardenFactory := exec.NewGardenFactory(
		workerClient,
		resourceFetcher,
		resourceFactory,
		dbResourceCacheFactory,
	)

	execV2Engine := engine.NewExecEngine(
		gardenFactory,
		engine.NewBuildDelegateFactory(),
		teamDBFactory,
		cmd.ExternalURL.String(),
	)

	execV1Engine := engine.NewExecV1DummyEngine()

	return engine.NewDBEngine(engine.Engines{execV2Engine, execV1Engine})
}

func (cmd *ATCCommand) constructHTTPHandler(
	logger lager.Logger,
	webHandler http.Handler,
	publicHandler http.Handler,
	apiHandler http.Handler,
	oauthHandler http.Handler,
) http.Handler {
	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", apiHandler)
	webMux.Handle("/auth/", oauthHandler)
	webMux.Handle("/public/", publicHandler)
	webMux.Handle("/robots.txt", robotstxt.Handler{})
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
	sqlDB *db.SQLDB,
	teamDBFactory db.TeamDBFactory,
	dbTeamFactory dbng.TeamFactory,
	dbPipelineFactory dbng.PipelineFactory,
	dbWorkerFactory dbng.WorkerFactory,
	dbVolumeFactory dbng.VolumeFactory,
	dbContainerFactory dbng.ContainerFactory,
	dbBuildFactory dbng.BuildFactory,
	providerFactory auth.OAuthFactory,
	signingKey *rsa.PrivateKey,
	pipelineDBFactory db.PipelineDBFactory,
	engine engine.Engine,
	workerClient worker.Client,
	drain <-chan struct{},
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	radarScannerFactory radar.ScannerFactory,
) (http.Handler, error) {
	authValidator := auth.JWTValidator{
		PublicKey: &signingKey.PublicKey,
	}

	getTokenValidator := auth.NewTeamAuthValidator(dbTeamFactory, authValidator)

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(
		dbTeamFactory,
	)

	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(dbBuildFactory)

	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(dbBuildFactory)

	checkWorkerTeamAccessHandlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(dbWorkerFactory)

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewAPIAuthWrappa(
			authValidator,
			getTokenValidator,
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

		auth.NewAuthTokenGenerator(signingKey),
		auth.NewCSRFTokenGenerator(),
		providerFactory,
		cmd.oauthBaseURL(),

		pipelineDBFactory,
		teamDBFactory,
		dbTeamFactory,
		dbPipelineFactory,
		dbWorkerFactory,
		dbVolumeFactory,
		dbContainerFactory,
		dbBuildFactory,

		sqlDB, // pipes.PipeDB

		cmd.PeerURL.String(),
		buildserver.NewEventHandler,
		drain,

		engine,
		workerClient,
		radarSchedulerFactory,
		radarScannerFactory,

		reconfigurableSink,

		cmd.AuthDuration,

		cmd.isTLSEnabled(),

		cmd.CLIArtifactsDir.Path(),
		Version,
		WorkerVersion,
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
	sqlDB *db.SQLDB,
	pipelineDBFactory db.PipelineDBFactory,
	dbPipelineFactory dbng.PipelineFactory,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
) *pipelines.Syncer {
	return pipelines.NewSyncer(
		logger,
		sqlDB,
		pipelineDBFactory,
		dbPipelineFactory,
		func(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline) ifrit.Runner {
			return grouper.NewParallel(os.Interrupt, grouper.Members{
				{
					pipelineDB.ScopedName("radar"),
					radar.NewRunner(
						logger.Session(pipelineDB.ScopedName("radar")),
						cmd.Developer.Noop,
						radarSchedulerFactory.BuildScanRunnerFactory(dbPipeline, cmd.ExternalURL.String()),
						dbPipeline,
						1*time.Minute,
					),
				},
				{
					pipelineDB.ScopedName("scheduler"),
					&scheduler.Runner{
						Logger: logger.Session(pipelineDB.ScopedName("scheduler")),

						DB:       pipelineDB,
						Pipeline: dbPipeline,

						Scheduler: radarSchedulerFactory.BuildScheduler(pipelineDB, dbPipeline, cmd.ExternalURL.String()),

						Noop: cmd.Developer.Noop,

						Interval: 10 * time.Second,
					},
				},
			})
		},
	)
}

func (cmd *ATCCommand) appendStaticWorker(
	logger lager.Logger,
	workerFactory dbng.WorkerFactory,
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
