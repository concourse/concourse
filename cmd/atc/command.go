package main

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/web/webhandler"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/wrappa"
	"github.com/dgrijalva/jwt-go"
	"github.com/felixge/tcpkeepalive"
	"github.com/gorilla/context"
	"github.com/hashicorp/go-multierror"
	"github.com/lib/pq"
	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"
	"github.com/xoebus/zest"
)

type ATCCommand struct {
	BindIP   IPFlag `long:"bind-ip"   default:"0.0.0.0" description:"IP address on which to listen for web traffic."`
	BindPort uint16 `long:"bind-port" default:"8080"    description:"Port on which to listen for web traffic."`

	PeerURL     URLFlag `long:"peer-url"     default:"http://127.0.0.1:8080" description:"URL used to reach this ATC from other ATCs in the cluster."`
	ExternalURL URLFlag `long:"external-url"                                 description:"URL used to reach any ATC from the outside world. Used for OAuth callbacks."`

	PostgresDataSource string `long:"postgres-data-source" default:"postgres://127.0.0.1:5432/atc?sslmode=disable" description:"PostgreSQL connection string."`

	TemplatesDir DirFlag `long:"templates" default:"./web/templates" description:"Directory containing the web templates."`
	PublicDir    DirFlag `long:"public"    default:"./web/public"    description:"Directory containing the web assets (js, css, etc.)."`

	DebugBindIP   IPFlag `long:"debug-bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the pprof debugger endpoints."`
	DebugBindPort uint16 `long:"debug-bind-port" default:"8079"      description:"Port on which to listen for the pprof debugger endpoints."`

	PubliclyViewable bool `short:"p" long:"publicly-viewable" description:"If true, anonymous users can view pipelines and public jobs."`

	SessionSigningKey FileFlag `long:"session-signing-key" description:"File containing an RSA private key, used to sign session tokens."`

	ResourceCheckingInterval time.Duration `long:"resource-checking-interval" default:"1m" description:"Interval on which to check for new versions of resources."`

	CLIArtifactsDir DirFlag `long:"cli-artifacts-dir" description:"Directory containing downloadable CLI binaries."`

	Developer struct {
		DevelopmentMode bool `short:"d" long:"development-mode"  description:"Lax security rules to make local development easier."`
		Noop            bool `short:"n" long:"noop"              description:"Don't actually do any automatic scheduling or checking."`
	} `group:"Developer Options"`

	Worker struct {
		GardenURL       URLFlag            `long:"garden-url"       description:"A Garden API endpoint to register as a worker."`
		BaggageclaimURL URLFlag            `long:"baggageclaim-url" description:"A Baggageclaim API endpoint to register with the worker."`
		ResourceTypes   map[string]URLFlag `long:"resource"         description:"A resource type to advertise for the worker. Can be specified multiple times." value-name:"TYPE:IMAGE"`
	} `group:"Static Worker (optional)" namespace:"worker"`

	BasicAuth struct {
		Username string `long:"username" description:"Username to use for basic auth."`
		Password string `long:"password" description:"Password to use for basic auth."`
	} `group:"Basic Authentication" namespace:"basic-auth"`

	GitHubAuth struct {
		ClientID     string `long:"client-id"     description:"Application client ID for enabling GitHub OAuth."`
		ClientSecret string `long:"client-secret" description:"Application client secret for enabling GitHub OAuth."`
		Organization string `long:"organization"  description:"GitHub organization whose members will have access."`
	} `group:"GitHub Authentication" namespace:"github-auth"`

	Metrics struct {
		HostName   string            `long:"metrics-host-name"   description:"Host string to attach to emitted metrics."`
		Tags       []string          `long:"metrics-tag"         description:"Tag to attach to emitted metrics. Can be specified multiple times." value-name:"TAG"`
		Attributes map[string]string `long:"metrics-attribute"   description:"A key-value attribute to attach to emitted metrics. Can be specified multiple times." value-name:"NAME:VALUE"`

		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`

		RiemannHost string `long:"riemann-host"                description:"Riemann server address to emit metrics to."`
		RiemannPort uint16 `long:"riemann-port" default:"5555" description:"Port of the Riemann server to emit metrics to."`
	} `group:"Metrics & Diagnostics"`
}

func (cmd *ATCCommand) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := cmd.validate()
	if err != nil {
		return err
	}

	logger, reconfigurableSink := cmd.constructLogger()

	cmd.configureMetrics(logger)

	sqlDB, pipelineDBFactory, err := cmd.constructDB(logger)
	if err != nil {
		return err
	}

	workerClient := cmd.constructWorkerPool(logger, sqlDB)

	tracker := resource.NewTracker(workerClient, sqlDB)

	engine := cmd.constructEngine(sqlDB, workerClient, tracker)

	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		tracker,
		cmd.ResourceCheckingInterval,
		engine,
		sqlDB,
	)

	signingKey, err := cmd.loadOrGenerateSigningKey()
	if err != nil {
		return err
	}

	authValidator, basicAuthEnabled := cmd.constructValidator(signingKey)

	oauthProviders, err := cmd.configureOAuthProviders()
	if err != nil {
		return err
	}

	drain := make(chan struct{})

	apiHandler, err := cmd.constructAPIHandler(
		logger,
		reconfigurableSink,
		sqlDB,
		authValidator,
		pipelineDBFactory,
		engine,
		workerClient,
		drain,
	)
	if err != nil {
		return err
	}

	oauthHandler, err := auth.NewOAuthHandler(
		logger,
		oauthProviders,
		signingKey,
	)
	if err != nil {
		return err
	}

	webHandler, err := cmd.constructWebHandler(
		logger,
		sqlDB,
		authValidator,
		oauthProviders,
		basicAuthEnabled,
		pipelineDBFactory,
		radarSchedulerFactory,
		engine,
	)
	if err != nil {
		return err
	}

	members := []grouper.Member{
		{"drainer", drainer(drain)},

		{"web", http_server.New(
			cmd.bindAddr(),
			cmd.constructHTTPHandler(
				webHandler,
				apiHandler,
				oauthHandler,
			),
		)},

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

		{"lostandfound", lostandfound.NewRunner(
			logger.Session("lost-and-found"),
			lostandfound.NewBaggageCollector(
				logger.Session("baggage-collector"),
				workerClient,
				sqlDB,
				pipelineDBFactory,
			),
			sqlDB,
			clock.NewClock(),
			10*time.Second,
			10*time.Second,
		)},
	}

	members = cmd.appendStaticWorker(logger, sqlDB, members)

	group := grouper.NewParallel(os.Interrupt, members)

	running := ifrit.Invoke(sigmon.New(group))

	logger.Info("listening", lager.Data{
		"web":   cmd.bindAddr(),
		"debug": cmd.debugBindAddr(),
	})

	close(ready)

	for {
		select {
		case s := <-signals:
			running.Signal(s)
		case err := <-running.Wait():
			if err != nil {
				logger.Error("exited-with-failure", err)
			}

			return err
		}
	}
}

func (cmd *ATCCommand) validate() error {
	var errs *multierror.Error

	if !cmd.Developer.DevelopmentMode && cmd.BasicAuth.Username == "" && cmd.GitHubAuth.Organization == "" {
		errs = multierror.Append(
			errs,
			errors.New("must configure basic auth, OAuth, or turn on development mode"),
		)
	}

	if cmd.GitHubAuth.Organization != "" && cmd.ExternalURL.String() == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --external-url to use OAuth"),
		)
	}

	if cmd.GitHubAuth.Organization != "" && (cmd.GitHubAuth.ClientID == "" || cmd.GitHubAuth.ClientSecret == "") {
		errs = multierror.Append(
			errs,
			errors.New("must specify --github-auth-client-id and --github-auth-client-secret to use GitHub OAuth"),
		)
	}

	return errs.ErrorOrNil()
}

func (cmd *ATCCommand) bindAddr() string {
	return fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
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
	if cmd.Metrics.RiemannHost != "" {
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
		)
	}
}

func (cmd *ATCCommand) constructDB(logger lager.Logger) (*db.SQLDB, db.PipelineDBFactory, error) {
	dbConn, err := migrations.LockDBAndMigrate(logger.Session("db.migrations"), "postgres", cmd.PostgresDataSource)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to migrate database: %s", err)
	}

	listener := pq.NewListener(cmd.PostgresDataSource, time.Second, time.Minute, nil)
	bus := db.NewNotificationsBus(listener, dbConn)

	explainDBConn := db.Explain(logger, dbConn, clock.NewClock(), 500*time.Millisecond)
	sqlDB := db.NewSQL(logger.Session("db"), explainDBConn, bus)

	pipelineDBFactory := db.NewPipelineDBFactory(logger.Session("db"), explainDBConn, bus, sqlDB)

	return sqlDB, pipelineDBFactory, err
}

func (cmd *ATCCommand) constructWorkerPool(logger lager.Logger, sqlDB *db.SQLDB) worker.Client {
	return worker.NewPool(
		worker.NewDBWorkerProvider(
			logger,
			sqlDB,
			keepaliveDialer,
			worker.ExponentialRetryPolicy{
				Timeout: 5 * time.Minute,
			},
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

func (cmd *ATCCommand) configureOAuthProviders() (auth.Providers, error) {
	oauthProviders := auth.Providers{}

	if cmd.GitHubAuth.Organization != "" {
		path, err := auth.OAuthRoutes.CreatePathForRoute(auth.OAuthCallback, rata.Params{
			"provider": github.ProviderName,
		})
		if err != nil {
			return nil, err
		}

		oauthProviders[github.ProviderName] = github.NewProvider(
			cmd.GitHubAuth.Organization,
			cmd.GitHubAuth.ClientID,
			cmd.GitHubAuth.ClientSecret,
			cmd.ExternalURL.String()+path,
		)
	}

	return oauthProviders, nil
}

func (cmd *ATCCommand) constructValidator(signingKey *rsa.PrivateKey) (auth.Validator, bool) {
	if cmd.Developer.DevelopmentMode {
		return auth.NoopValidator{}, false
	}

	jwtValidator := auth.JWTValidator{
		PublicKey: &signingKey.PublicKey,
	}

	var basicAuthValidator auth.Validator

	if cmd.BasicAuth.Username != "" && cmd.BasicAuth.Password != "" {
		basicAuthValidator = auth.BasicAuthValidator{
			Username: cmd.BasicAuth.Username,
			Password: cmd.BasicAuth.Password,
		}
	}

	var validator auth.Validator

	if basicAuthValidator != nil {
		validator = auth.ValidatorBasket{basicAuthValidator, jwtValidator}
	} else {
		validator = jwtValidator
	}

	return validator, basicAuthValidator != nil
}

func (cmd *ATCCommand) constructEngine(
	sqlDB *db.SQLDB,
	workerClient worker.Client,
	tracker resource.Tracker,
) engine.Engine {
	gardenFactory := exec.NewGardenFactory(workerClient, tracker, uuidGen)

	execEngine := engine.NewExecEngine(
		gardenFactory,
		engine.NewBuildDelegateFactory(sqlDB),
		sqlDB,
	)

	return engine.NewDBEngine(engine.Engines{execEngine}, sqlDB)
}

func (cmd *ATCCommand) constructHTTPHandler(
	webHandler http.Handler,
	apiHandler http.Handler,
	oauthHandler http.Handler,
) http.Handler {
	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", apiHandler)
	webMux.Handle("/auth/", oauthHandler)
	webMux.Handle("/", webHandler)

	var httpHandler http.Handler

	httpHandler = webMux

	// proxy Authorization header to/from auth cookie,
	// to support auth from JS (EventSource) and custom JWT auth
	httpHandler = auth.CookieSetHandler{
		Handler: httpHandler,
	}

	// don't leak gorilla context per-request
	httpHandler = context.ClearHandler(httpHandler)

	return httpHandler
}

func (cmd *ATCCommand) constructAPIHandler(
	logger lager.Logger,
	reconfigurableSink *lager.ReconfigurableSink,
	sqlDB *db.SQLDB,
	authValidator auth.Validator,
	pipelineDBFactory db.PipelineDBFactory,
	engine engine.Engine,
	workerClient worker.Client,
	drain <-chan struct{},
) (http.Handler, error) {
	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIAuthWrappa(authValidator),
		wrappa.NewAPIMetricsWrappa(logger),
	}

	return api.NewHandler(
		logger,
		apiWrapper,
		pipelineDBFactory,

		sqlDB, // db.ConfigDB
		sqlDB, // buildserver.BuildsDB
		sqlDB, // workerserver.WorkerDB
		sqlDB, // containerServer.ContainerDB
		sqlDB, // pipes.PipeDB
		sqlDB, // db.PipelinesDB

		config.ValidateConfig,
		cmd.PeerURL.String(),
		buildserver.NewEventHandler,
		drain,

		engine,
		workerClient,

		reconfigurableSink,

		cmd.CLIArtifactsDir.Path(),
	)
}

func (cmd *ATCCommand) constructWebHandler(
	logger lager.Logger,
	sqlDB *db.SQLDB,
	authValidator auth.Validator,
	oauthProviders auth.Providers,
	basicAuthEnabled bool,
	pipelineDBFactory db.PipelineDBFactory,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	engine engine.Engine,
) (http.Handler, error) {
	webWrapper := wrappa.MultiWrappa{
		wrappa.NewWebAuthWrappa(cmd.PubliclyViewable, authValidator),
		wrappa.NewWebMetricsWrappa(logger),
	}

	return webhandler.NewHandler(
		logger,
		webWrapper,
		oauthProviders,
		basicAuthEnabled,
		radarSchedulerFactory,
		sqlDB,
		pipelineDBFactory,
		sqlDB,
		cmd.TemplatesDir.Path(),
		cmd.PublicDir.Path(),
		engine,
	)
}

func (cmd *ATCCommand) constructPipelineSyncer(
	logger lager.Logger,
	sqlDB *db.SQLDB,
	pipelineDBFactory db.PipelineDBFactory,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
) *pipelines.Syncer {
	return pipelines.NewSyncer(
		logger.Session("syncer"),
		sqlDB,
		pipelineDBFactory,
		func(pipelineDB db.PipelineDB) ifrit.Runner {
			return grouper.NewParallel(os.Interrupt, grouper.Members{
				{
					pipelineDB.ScopedName("radar"),
					radar.NewRunner(
						logger.Session(pipelineDB.ScopedName("radar")),
						cmd.Developer.Noop,
						radarSchedulerFactory.BuildRadar(pipelineDB),
						pipelineDB,
						1*time.Minute,
					),
				},
				{
					pipelineDB.ScopedName("scheduler"),
					&scheduler.Runner{
						Logger: logger.Session(pipelineDB.ScopedName("scheduler")),

						DB: pipelineDB,

						Scheduler: radarSchedulerFactory.BuildScheduler(pipelineDB),

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
	if cmd.Worker.GardenURL.Host == "" {
		return members
	}

	var resourceTypes []atc.WorkerResourceType
	for t, u := range cmd.Worker.ResourceTypes {
		resourceTypes = append(resourceTypes, atc.WorkerResourceType{
			Type:  t,
			Image: u.String(),
		})
	}

	return append(members,
		grouper.Member{
			Name: "static-worker",
			Runner: worker.NewHardcoded(
				logger,
				sqlDB,
				clock.NewClock(),
				cmd.Worker.GardenURL.Host,
				cmd.Worker.BaggageclaimURL.String(),
				resourceTypes,
			),
		},
	)
}

func keepaliveDialer(network string, address string) (net.Conn, error) {
	conn, err := net.DialTimeout(network, address, 5*time.Second)
	if err != nil {
		return nil, err
	}

	err = tcpkeepalive.SetKeepAlive(conn, 10*time.Second, 3, 5*time.Second)
	if err != nil {
		println("failed to enable connection keepalive: " + err.Error())
	}

	return conn, nil
}

func uuidGen() string {
	guid, err := uuid.NewV4()
	if err != nil {
		panic("not enough entropy to generate guid: " + err.Error())
	}

	return guid.String()
}
