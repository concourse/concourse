package atccmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
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
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/gc/buildreaper"
	"github.com/concourse/atc/gc/containerkeepaliver"
	"github.com/concourse/atc/gc/dbgc"
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
	jwt "github.com/dgrijalva/jwt-go"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx"
	"github.com/lib/pq"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/xoebus/zest"
)

type ATCCommand struct {
	BindIP   IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16 `long:"bind-port" default:"8080"    description:"Port on which to listen for HTTP traffic."`

	TLSBindPort uint16   `long:"tls-bind-port" description:"Port on which to listen for HTTPS traffic."`
	TLSCert     FileFlag `long:"tls-cert"      description:"File containing an SSL certificate."`
	TLSKey      FileFlag `long:"tls-key"       description:"File containing an RSA private key, used to encrypt HTTPS traffic."`

	ExternalURL URLFlag `long:"external-url" default:"http://127.0.0.1:8080" description:"URL used to reach any ATC from the outside world."`
	PeerURL     URLFlag `long:"peer-url"     default:"http://127.0.0.1:8080" description:"URL used to reach this ATC from other ATCs in the cluster."`

	OAuthBaseURL URLFlag `long:"oauth-base-url" description:"URL used as the base of OAuth redirect URIs. If not specified, the external URL is used."`

	AuthDuration time.Duration `long:"auth-duration" default:"24h" description:"Length of time for which tokens are valid. Afterwards, users will have to log back in."`

	PostgresDataSource string `long:"postgres-data-source" default:"postgres://127.0.0.1:5432/atc?sslmode=disable" description:"PostgreSQL connection string."`

	DebugBindIP   IPFlag `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16 `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	SessionSigningKey FileFlag `long:"session-signing-key" description:"File containing an RSA private key, used to sign session tokens."`

	ResourceCheckingInterval     time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`
	OldResourceGracePeriod       time.Duration `long:"old-resource-grace-period" default:"5m" description:"How long to cache the result of a get step after a newer version of the resource is found."`
	ResourceCacheCleanupInterval time.Duration `long:"resource-cache-cleanup-interval" default:"30s" description:"Interval on which to cleanup old caches of resources."`

	CLIArtifactsDir DirFlag `long:"cli-artifacts-dir" description:"Directory containing downloadable CLI binaries."`

	Developer struct {
		DevelopmentMode bool `short:"d" long:"development-mode"  description:"Lax security rules to make local development easier."`
		Noop            bool `short:"n" long:"noop"              description:"Don't actually do any automatic scheduling or checking."`
	} `group:"Developer Options"`

	AllowSelfSignedCertificates bool `long:"allow-self-signed-certificates" description:"Allow self signed certificates."`

	Worker struct {
		GardenURL       URLFlag           `long:"garden-url"       description:"A Garden API endpoint to register as a worker."`
		BaggageclaimURL URLFlag           `long:"baggageclaim-url" description:"A Baggageclaim API endpoint to register with the worker."`
		ResourceTypes   map[string]string `long:"resource"         description:"A resource type to advertise for the worker. Can be specified multiple times." value-name:"TYPE:IMAGE"`
	} `group:"Static Worker (optional)" namespace:"worker"`

	BasicAuth atc.BasicAuthFlag `group:"Basic Authentication" namespace:"basic-auth"`

	GitHubAuth atc.GitHubAuthFlag `group:"GitHub Authentication" namespace:"github-auth"`

	UAAAuth atc.UAAAuthFlag `group:"UAA Authentication" namespace:"uaa-auth"`

	GenericOAuth atc.GenericOAuthFlag `group:"Generic OAuth Authentication (Allows access to ALL authenticated users)" namespace:"generic-oauth"`

	Metrics struct {
		HostName   string            `long:"metrics-host-name"   description:"Host string to attach to emitted metrics."`
		Tags       []string          `long:"metrics-tag"         description:"Tag to attach to emitted metrics. Can be specified multiple times." value-name:"TAG"`
		Attributes map[string]string `long:"metrics-attribute"   description:"A key-value attribute to attach to emitted metrics. Can be specified multiple times." value-name:"NAME:VALUE"`

		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`

		RiemannHost          string `long:"riemann-host"                description:"Riemann server address to emit metrics to."`
		RiemannPort          uint16 `long:"riemann-port" default:"5555" description:"Port of the Riemann server to emit metrics to."`
		RiemannServicePrefix string `long:"riemann-service-prefix" default:"" description:"An optional prefix for emitted Riemann services"`
	} `group:"Metrics & Diagnostics"`

	LogDBQueries bool `long:"log-db-queries" description:"Log database queries."`
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

	logger, reconfigurableSink := cmd.constructLogger()

	go metric.PeriodicallyEmit(logger.Session("periodic-metrics"), 10*time.Second)

	if cmd.Metrics.RiemannHost != "" {
		cmd.configureMetrics(logger)
	}

	dbConn, dbngConn, err := cmd.constructDBConn(logger)
	if err != nil {
		return nil, err
	}
	if cmd.LogDBQueries {
		dbConn = db.Log(logger.Session("log-conn"), dbConn)
	}

	lockConn, err := cmd.constructLockConn()
	if err != nil {
		return nil, err
	}
	lockFactory := db.NewLockFactory(lockConn)

	listener := pq.NewListener(cmd.PostgresDataSource, time.Second, time.Minute, nil)
	bus := db.NewNotificationsBus(listener, dbConn)

	sqlDB := db.NewSQL(dbConn, bus, lockFactory)
	resourceFetcherFactory := resource.NewFetcherFactory(sqlDB, clock.NewClock())
	resourceFactoryFactory := resource.NewResourceFactoryFactory()
	pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, lockFactory)
	dbVolumeFactory := dbng.NewVolumeFactory(dbngConn)
	dbContainerFactory := dbng.NewContainerFactory(dbngConn)
	dbTeamFactory := dbng.NewTeamFactory(dbngConn)
	dbWorkerFactory := dbng.NewWorkerFactory(dbngConn)
	dbResourceCacheFactory := dbng.NewResourceCacheFactory(dbngConn)
	dbResourceTypeFactory := dbng.NewResourceTypeFactory(dbngConn)
	dbResourceConfigFactory := dbng.NewResourceConfigFactory(dbngConn)
	dbBaseResourceTypeFactory := dbng.NewBaseResourceTypeFactory(dbngConn)
	workerClient := cmd.constructWorkerPool(
		logger,
		sqlDB,
		resourceFetcherFactory,
		resourceFactoryFactory,
		pipelineDBFactory,
		dbContainerFactory,
		dbResourceCacheFactory,
		dbResourceTypeFactory,
		dbResourceConfigFactory,
		dbBaseResourceTypeFactory,
		dbVolumeFactory,
	)

	resourceFetcher := resourceFetcherFactory.FetcherFor(workerClient)
	resourceFactory := resourceFactoryFactory.FactoryFor(workerClient)
	teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
	engine := cmd.constructEngine(workerClient, resourceFetcher, resourceFactory, teamDBFactory)

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

	err = cmd.configureAuthForDefaultTeam(teamDBFactory)
	if err != nil {
		return nil, err
	}

	providerFactory := provider.NewOAuthFactory(
		logger.Session("oauth-provider-factory"),
		cmd.oauthBaseURL(),
		auth.OAuthRoutes,
		auth.OAuthCallback,
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
		dbWorkerFactory,
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
		teamDBFactory,
		signingKey,
		cmd.AuthDuration,
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
	if cmd.TLSBindPort != 0 {
		httpHandler = cmd.constructHTTPHandler(
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
			webHandler,
			publicHandler,
			apiHandler,
			oauthHandler,
		)
	} else {
		httpHandler = cmd.constructHTTPHandler(
			webHandler,
			publicHandler,
			apiHandler,
			oauthHandler,
		)
	}

	members := []grouper.Member{
		{"drainer", drainer(drain)},

		{"debug", http_server.New(
			cmd.debugBindAddr(),
			http.DefaultServeMux,
		)},

		{"pipelines", pipelines.SyncRunner{
			Syncer: cmd.constructPipelineSyncer(
				logger.Session("syncer"),
				sqlDB,
				pipelineDBFactory,
				radarSchedulerFactory,
			),
			Interval: 10 * time.Second,
			Clock:    clock.NewClock(),
		}},

		{"builds", builds.TrackerRunner{
			Tracker: builds.NewTracker(
				logger.Session("build-tracker"),
				sqlDB,
				engine,
			),
			Interval: 10 * time.Second,
			Clock:    clock.NewClock(),
		}},

		{"volumecollector", lockrunner.NewRunner(
			logger.Session("volume-collector"),
			gcng.NewVolumeCollector(
				logger.Session("volume-collector"),
				dbVolumeFactory,
				workerClient,
			),
			"volume-collector",
			sqlDB,
			clock.NewClock(),
			30*time.Second,
		)},

		{"workercollector", lockrunner.NewRunner(
			logger.Session("worker-collector"),
			gcng.NewWorkerCollector(
				logger.Session("volume-collector"),
				dbWorkerFactory,
			),
			"worker-collector",
			sqlDB,
			clock.NewClock(),
			30*time.Second,
		)},

		{"containerkeepaliver", lockrunner.NewRunner(
			logger.Session("container-keepaliver"),
			containerkeepaliver.NewContainerKeepAliver(
				logger.Session("container-keepaliver"),
				workerClient,
				sqlDB,
			),
			"container-keepaliver",
			sqlDB,
			clock.NewClock(),
			30*time.Second,
		)},

		{"buildreaper", lockrunner.NewRunner(
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

		{"dbgc", lockrunner.NewRunner(
			logger.Session("dbgc"),
			dbgc.NewDBGarbageCollector(
				logger.Session("dbgc"),
				sqlDB,
			),
			"dbgc",
			sqlDB,
			clock.NewClock(),
			60*time.Second,
		)},
	}

	if cmd.Worker.GardenURL.URL() != nil {
		members = cmd.appendStaticWorker(logger, sqlDB, members)
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

		if cmd.TLSBindPort != 0 {
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

func (cmd *ATCCommand) authConfigured() bool {
	return cmd.BasicAuth.IsConfigured() || cmd.GitHubAuth.IsConfigured() || cmd.UAAAuth.IsConfigured() || cmd.GenericOAuth.IsConfigured()
}

func (cmd *ATCCommand) validate() error {
	var errs *multierror.Error

	if !cmd.authConfigured() && !cmd.Developer.DevelopmentMode {
		errs = multierror.Append(
			errs,
			errors.New("must configure basic auth, OAuth, UAAAuth, or turn on development mode"),
		)
	}

	if cmd.GitHubAuth.IsConfigured() {
		if cmd.ExternalURL.URL() == nil {
			errs = multierror.Append(
				errs,
				errors.New("must specify --external-url to use OAuth"),
			)
		}

		err := cmd.GitHubAuth.Validate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if cmd.GenericOAuth.IsConfigured() {
		err := cmd.GenericOAuth.Validate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if cmd.BasicAuth.IsConfigured() {
		err := cmd.BasicAuth.Validate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if cmd.UAAAuth.IsConfigured() {
		err := cmd.UAAAuth.Validate()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
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
	if cmd.TLSBindPort != 0 {
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
	logger := lager.NewLogger("atc")

	logLevel := lager.INFO
	if cmd.Developer.DevelopmentMode {
		logLevel = lager.DEBUG
	}

	reconfigurableSink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, lager.DEBUG), logLevel)
	logger.RegisterSink(reconfigurableSink)

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

	metric.Initialize(
		logger.Session("metrics"),
		fmt.Sprintf("%s:%d", cmd.Metrics.RiemannHost, cmd.Metrics.RiemannPort),
		host,
		cmd.Metrics.Tags,
		cmd.Metrics.Attributes,
		cmd.Metrics.RiemannServicePrefix,
	)
}

func (cmd *ATCCommand) constructDBConn(logger lager.Logger) (db.Conn, dbng.Conn, error) {
	driverName := "connection-counting"
	metric.SetupConnectionCountingDriver("postgres", cmd.PostgresDataSource, driverName)

	dbConn, err := migrations.LockDBAndMigrate(logger.Session("db.migrations"), driverName, cmd.PostgresDataSource)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	dbngConn, err := migrations.DBNGConn(logger.Session("db.migrations"), driverName, cmd.PostgresDataSource)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	dbConn.SetMaxOpenConns(64)
	dbngConn.SetMaxOpenConns(64)

	return metric.CountQueries(dbConn), dbngConn, nil
}

func (cmd *ATCCommand) constructLockConn() (*db.RetryableConn, error) {
	var pgxConfig pgx.ConnConfig
	var err error

	if strings.HasPrefix(cmd.PostgresDataSource, "postgres://") ||
		strings.HasPrefix(cmd.PostgresDataSource, "postgresql://") {
		pgxConfig, err = pgx.ParseURI(cmd.PostgresDataSource)
	} else {
		pgxConfig, err = pgx.ParseDSN(cmd.PostgresDataSource)
	}

	if err != nil {
		return nil, err
	}

	connector := &db.PgxConnector{PgxConfig: pgxConfig}

	pgxConn, err := connector.Connect()
	if err != nil {
		return nil, err
	}

	return &db.RetryableConn{Connector: connector, Conn: pgxConn}, nil
}

func (cmd *ATCCommand) constructWorkerPool(
	logger lager.Logger,
	sqlDB *db.SQLDB,
	resourceFetcherFactory resource.FetcherFactory,
	resourceFactoryFactory resource.ResourceFactoryFactory,
	pipelineDBFactory db.PipelineDBFactory,
	dbContainerFactory *dbng.ContainerFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
	dbResourceTypeFactory dbng.ResourceTypeFactory,
	dbResourceConfigFactory dbng.ResourceConfigFactory,
	dbBaseResourceTypeFactory dbng.BaseResourceTypeFactory,
	dbVolumeFactory dbng.VolumeFactory,
) worker.Client {
	return worker.NewPool(
		worker.NewDBWorkerProvider(
			logger,
			sqlDB,
			keepaliveDialer,
			retryhttp.NewExponentialBackOffFactory(5*time.Minute),
			image.NewFactory(resourceFetcherFactory, resourceFactoryFactory),
			dbContainerFactory,
			dbResourceCacheFactory,
			dbResourceTypeFactory,
			dbResourceConfigFactory,
			dbBaseResourceTypeFactory,
			dbVolumeFactory,
			pipelineDBFactory,
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

func (cmd *ATCCommand) configureAuthForDefaultTeam(teamDBFactory db.TeamDBFactory) error {
	teamDB := teamDBFactory.GetTeamDB(atc.DefaultTeamName)

	var basicAuth *db.BasicAuth
	if cmd.BasicAuth.IsConfigured() {
		basicAuth = &db.BasicAuth{
			BasicAuthUsername: cmd.BasicAuth.Username,
			BasicAuthPassword: cmd.BasicAuth.Password,
		}
	}
	_, err := teamDB.UpdateBasicAuth(basicAuth)
	if err != nil {
		return err
	}

	var gitHubAuth *db.GitHubAuth
	if cmd.GitHubAuth.IsConfigured() {
		gitHubTeams := []db.GitHubTeam{}
		for _, gitHubTeam := range cmd.GitHubAuth.Teams {
			gitHubTeams = append(gitHubTeams, db.GitHubTeam{
				TeamName:         gitHubTeam.TeamName,
				OrganizationName: gitHubTeam.OrganizationName,
			})
		}

		gitHubAuth = &db.GitHubAuth{
			ClientID:      cmd.GitHubAuth.ClientID,
			ClientSecret:  cmd.GitHubAuth.ClientSecret,
			Organizations: cmd.GitHubAuth.Organizations,
			Teams:         gitHubTeams,
			Users:         cmd.GitHubAuth.Users,
			AuthURL:       cmd.GitHubAuth.AuthURL,
			TokenURL:      cmd.GitHubAuth.TokenURL,
			APIURL:        cmd.GitHubAuth.APIURL,
		}
	}

	_, err = teamDB.UpdateGitHubAuth(gitHubAuth)
	if err != nil {
		return err
	}

	var uaaAuth *db.UAAAuth
	if cmd.UAAAuth.IsConfigured() {
		cfCACert := ""
		if cmd.UAAAuth.CFCACert != "" {
			cfCACertFileContents, err := ioutil.ReadFile(string(cmd.UAAAuth.CFCACert))
			if err != nil {
				return err
			}
			cfCACert = string(cfCACertFileContents)
		}

		uaaAuth = &db.UAAAuth{
			ClientID:     cmd.UAAAuth.ClientID,
			ClientSecret: cmd.UAAAuth.ClientSecret,
			CFSpaces:     cmd.UAAAuth.CFSpaces,
			AuthURL:      cmd.UAAAuth.AuthURL,
			TokenURL:     cmd.UAAAuth.TokenURL,
			CFURL:        cmd.UAAAuth.CFURL,
			CFCACert:     cfCACert,
		}
	}

	_, err = teamDB.UpdateUAAAuth(uaaAuth)
	if err != nil {
		return err
	}

	var genericOAuth *db.GenericOAuth
	if cmd.GenericOAuth.IsConfigured() {
		genericOAuth = &db.GenericOAuth{
			AuthURL:       cmd.GenericOAuth.AuthURL,
			AuthURLParams: cmd.GenericOAuth.AuthURLParams,
			Scope:         cmd.GenericOAuth.Scope,
			TokenURL:      cmd.GenericOAuth.TokenURL,
			ClientID:      cmd.GenericOAuth.ClientID,
			ClientSecret:  cmd.GenericOAuth.ClientSecret,
			DisplayName:   cmd.GenericOAuth.DisplayName,
		}
	}

	_, err = teamDB.UpdateGenericOAuth(genericOAuth)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *ATCCommand) constructEngine(
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	teamDBFactory db.TeamDBFactory,
) engine.Engine {
	gardenFactory := exec.NewGardenFactory(
		workerClient,
		resourceFetcher,
		resourceFactory,
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

	// proxy Authorization header to/from auth cookie,
	// to support auth from JS (EventSource) and custom JWT auth
	httpHandler := auth.CookieSetHandler{
		Handler: webMux,
	}

	return httpHandler
}

func (cmd *ATCCommand) constructAPIHandler(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	sqlDB *db.SQLDB,
	teamDBFactory db.TeamDBFactory,
	dbTeamFactory dbng.TeamFactory,
	dbWorkerFactory dbng.WorkerFactory,
	providerFactory provider.OAuthFactory,
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

	getTokenValidator := auth.NewTeamAuthValidator(teamDBFactory, authValidator)

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(
		pipelineDBFactory,
		teamDBFactory,
	)

	checkBuildReadAccessHandlerFactory := auth.NewCheckBuildReadAccessHandlerFactory(sqlDB)

	checkBuildWriteAccessHandlerFactory := auth.NewCheckBuildWriteAccessHandlerFactory(sqlDB)

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIMetricsWrappa(logger),
		wrappa.NewAPIAuthWrappa(
			authValidator,
			getTokenValidator,
			auth.JWTReader{PublicKey: &signingKey.PublicKey},
			checkPipelineAccessHandlerFactory,
			checkBuildReadAccessHandlerFactory,
			checkBuildWriteAccessHandlerFactory,
		),
		wrappa.NewConcourseVersionWrappa(Version),
	}

	return api.NewHandler(
		logger,
		cmd.ExternalURL.String(),
		apiWrapper,

		auth.NewTokenGenerator(signingKey),
		providerFactory,
		cmd.oauthBaseURL(),

		pipelineDBFactory,
		teamDBFactory,
		dbTeamFactory,
		dbWorkerFactory,

		sqlDB, // teamserver.TeamDB
		sqlDB, // workerserver.WorkerDB
		sqlDB, // buildserver.BuildsDB
		sqlDB, // containerserver.ContainerDB
		sqlDB, // volumeserver.VolumesDB
		sqlDB, // pipes.PipeDB
		sqlDB, // db.PipelinesDB

		config.ValidateConfig,
		cmd.PeerURL.String(),
		buildserver.NewEventHandler,
		drain,

		engine,
		workerClient,
		radarSchedulerFactory,
		radarScannerFactory,

		reconfigurableSink,

		cmd.AuthDuration,

		cmd.CLIArtifactsDir.Path(),
		Version,
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
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
) *pipelines.Syncer {
	return pipelines.NewSyncer(
		logger,
		sqlDB,
		pipelineDBFactory,
		func(pipelineDB db.PipelineDB) ifrit.Runner {
			return grouper.NewParallel(os.Interrupt, grouper.Members{
				{
					pipelineDB.ScopedName("radar"),
					radar.NewRunner(
						logger.Session(pipelineDB.ScopedName("radar")),
						cmd.Developer.Noop,
						radarSchedulerFactory.BuildScanRunnerFactory(pipelineDB, cmd.ExternalURL.String()),
						pipelineDB,
						1*time.Minute,
					),
				},
				{
					pipelineDB.ScopedName("scheduler"),
					&scheduler.Runner{
						Logger: logger.Session(pipelineDB.ScopedName("scheduler")),

						DB: pipelineDB,

						Scheduler: radarSchedulerFactory.BuildScheduler(pipelineDB, cmd.ExternalURL.String()),

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
	sqlDB *db.SQLDB,
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
				sqlDB,
				clock.NewClock(),
				cmd.Worker.GardenURL.URL().Host,
				cmd.Worker.BaggageclaimURL.String(),
				resourceTypes,
			),
		},
	)
}
