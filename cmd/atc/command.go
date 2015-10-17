package main

import (
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

	logger := lager.NewLogger("atc")

	logLevel := lager.INFO
	if cmd.Developer.DevelopmentMode {
		logLevel = lager.DEBUG
	}

	sink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, lager.DEBUG), logLevel)
	logger.RegisterSink(sink)

	if cmd.Metrics.YellerAPIKey != "" {
		yellerSink := zest.NewYellerSink(cmd.Metrics.YellerAPIKey, cmd.Metrics.YellerEnvironment)
		logger.RegisterSink(yellerSink)
	}

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

	dbConn, err := migrations.LockDBAndMigrate(logger.Session("db.migrations"), "postgres", cmd.PostgresDataSource)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %s", err)
	}

	listener := pq.NewListener(cmd.PostgresDataSource, time.Second, time.Minute, nil)
	bus := db.NewNotificationsBus(listener, dbConn)

	explainDBConn := db.Explain(logger, dbConn, clock.NewClock(), 500*time.Millisecond)
	sqlDB := db.NewSQL(logger.Session("db"), explainDBConn, bus)
	pipelineDBFactory := db.NewPipelineDBFactory(logger.Session("db"), explainDBConn, bus, sqlDB)

	configDB := db.PlanConvertingConfigDB{NestedDB: sqlDB}

	workerClient := worker.NewPool(
		worker.NewDBWorkerProvider(
			logger,
			sqlDB,
			keepaliveDialer,
			worker.ExponentialRetryPolicy{
				Timeout: 5 * time.Minute,
			},
		),
	)

	tracker := resource.NewTracker(workerClient)

	gardenFactory := exec.NewGardenFactory(workerClient, tracker, func() string {
		guid, err := uuid.NewV4()
		if err != nil {
			panic("not enough entropy to generate guid: " + err.Error())
		}

		return guid.String()
	})

	execEngine := engine.NewExecEngine(gardenFactory, engine.NewBuildDelegateFactory(sqlDB), sqlDB)

	engine := engine.NewDBEngine(engine.Engines{execEngine}, sqlDB)

	var signingKey *rsa.PrivateKey

	if cmd.SessionSigningKey != "" {
		rsaKeyBlob, err := ioutil.ReadFile(string(cmd.SessionSigningKey))
		if err != nil {
			return fmt.Errorf("failed to read session signing key file: %s", err)
		}

		signingKey, err = jwt.ParseRSAPrivateKeyFromPEM(rsaKeyBlob)
		if err != nil {
			return fmt.Errorf("failed to parse session signing key as RSA: %s", err)
		}
	}

	validator, basicAuthEnabled := cmd.constructValidator(signingKey)

	oauthProviders := auth.Providers{}

	if cmd.GitHubAuth.Organization != "" {
		path, err := auth.OAuthRoutes.CreatePathForRoute(auth.OAuthCallback, rata.Params{
			"provider": github.ProviderName,
		})
		if err != nil {
			return err
		}

		oauthProviders[github.ProviderName] = github.NewProvider(
			cmd.GitHubAuth.Organization,
			cmd.GitHubAuth.ClientID,
			cmd.GitHubAuth.ClientSecret,
			cmd.ExternalURL.String()+path,
		)
	}

	drain := make(chan struct{})

	apiWrapper := wrappa.MultiWrappa{
		wrappa.NewAPIAuthWrappa(validator),
		wrappa.NewAPIMetricsWrappa(logger),
	}

	apiHandler, err := api.NewHandler(
		logger,
		apiWrapper,
		pipelineDBFactory,

		configDB,

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

		sink,

		cmd.CLIArtifactsDir.Path(),
	)
	if err != nil {
		return fmt.Errorf("failed to construct API handler: %s", err)
	}

	oauthHandler, err := auth.NewOAuthHandler(
		logger,
		oauthProviders,
		signingKey,
	)
	if err != nil {
		return fmt.Errorf("failed to construct OAuth handler: %s", err)
	}

	radarSchedulerFactory := pipelines.NewRadarSchedulerFactory(
		tracker,
		cmd.ResourceCheckingInterval,
		engine,
		sqlDB,
	)

	webWrapper := wrappa.MultiWrappa{
		wrappa.NewWebAuthWrappa(cmd.PubliclyViewable, validator),
		wrappa.NewWebMetricsWrappa(logger),
	}

	webHandler, err := webhandler.NewHandler(
		logger,
		webWrapper,
		oauthProviders,
		basicAuthEnabled,
		radarSchedulerFactory,
		sqlDB,
		pipelineDBFactory,
		configDB,
		cmd.TemplatesDir.Path(),
		cmd.PublicDir.Path(),
		engine,
	)
	if err != nil {
		return fmt.Errorf("failed to construct web handler: %s", err)
	}

	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", apiHandler)
	webMux.Handle("/auth/", oauthHandler)
	webMux.Handle("/", webHandler)

	var httpHandler http.Handler

	httpHandler = webMux

	httpHandler = auth.CookieSetHandler{
		Handler: httpHandler,
	}

	httpHandler = context.ClearHandler(httpHandler)

	webListenAddr := fmt.Sprintf("%s:%d", cmd.BindIP, cmd.BindPort)
	debugListenAddr := fmt.Sprintf("%s:%d", cmd.DebugBindIP, cmd.DebugBindPort)

	syncer := pipelines.NewSyncer(
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

	buildTracker := builds.NewTracker(
		logger.Session("build-tracker"),
		sqlDB,
		engine,
	)

	memberGrouper := []grouper.Member{
		{"web", http_server.New(webListenAddr, httpHandler)},

		{"debug", http_server.New(debugListenAddr, http.DefaultServeMux)},

		{"drainer", ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals

			close(drain)

			return nil
		})},

		{"pipelines", pipelines.SyncRunner{
			Syncer:   syncer,
			Interval: 10 * time.Second,
			Clock:    clock.NewClock(),
		}},

		{"builds", builds.TrackerRunner{
			Tracker:  buildTracker,
			Interval: 10 * time.Second,
			Clock:    clock.NewClock(),
		}},
	}

	// register a hardcoded worker
	if cmd.Worker.GardenURL.Host != "" {
		var resourceTypes []atc.WorkerResourceType
		for t, u := range cmd.Worker.ResourceTypes {
			resourceTypes = append(resourceTypes, atc.WorkerResourceType{
				Type:  t,
				Image: u.String(),
			})
		}

		memberGrouper = append(memberGrouper,
			grouper.Member{
				Name: "hardcoded-worker",
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

	group := grouper.NewParallel(os.Interrupt, memberGrouper)

	running := ifrit.Invoke(sigmon.New(group))

	logger.Info("listening", lager.Data{
		"web":   webListenAddr,
		"debug": debugListenAddr,
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

	if cmd.GitHubAuth.Organization != "" && cmd.SessionSigningKey == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --session-signing-key if OAuth is configured"),
		)
	}

	return errs.ErrorOrNil()
}

func (cmd *ATCCommand) constructValidator(signingKey *rsa.PrivateKey) (auth.Validator, bool) {
	if cmd.Developer.DevelopmentMode {
		return auth.NoopValidator{}, false
	}

	var basicAuthValidator auth.Validator

	if cmd.BasicAuth.Username != "" && cmd.BasicAuth.Password != "" {
		basicAuthValidator = auth.BasicAuthValidator{
			Username: cmd.BasicAuth.Username,
			Password: cmd.BasicAuth.Password,
		}
	}

	var jwtValidator auth.Validator

	if signingKey != nil {
		jwtValidator = auth.JWTValidator{
			PublicKey: &signingKey.PublicKey,
		}
	}

	var validator auth.Validator

	if basicAuthValidator != nil && jwtValidator != nil {
		validator = auth.ValidatorBasket{basicAuthValidator, jwtValidator}
	} else if basicAuthValidator != nil {
		validator = basicAuthValidator
	} else if jwtValidator != nil {
		validator = jwtValidator
	} else {
		validator = auth.NoopValidator{}
	}

	return validator, basicAuthValidator != nil
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
