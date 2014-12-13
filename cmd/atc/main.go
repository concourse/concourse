package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/migration"
	"github.com/cloudfoundry-incubator/candiedyaml"
	_ "github.com/lib/pq"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/config"
	Db "github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/event"
	rdr "github.com/concourse/atc/radar"
	sched "github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/web"
	"github.com/concourse/turbine"
)

var pipelinePath = flag.String(
	"pipeline",
	"",
	"path to atc pipeline config .yml",
)

var templatesDir = flag.String(
	"templates",
	"./web/templates",
	"path to directory containing the html templates",
)

var publicDir = flag.String(
	"public",
	"./web/public",
	"path to directory containing public resources (javascript, css, etc.)",
)

var turbineURL = flag.String(
	"turbineURL",
	"http://127.0.0.1:4637",
	"address denoting the turbine service",
)

var sqlDriver = flag.String(
	"sqlDriver",
	"postgres",
	"database/sql driver name",
)

var sqlDataSource = flag.String(
	"sqlDataSource",
	"",
	"database/sql data source configuration string",
)

var webListenAddress = flag.String(
	"webListenAddress",
	"0.0.0.0",
	"address to listen on",
)

var webListenPort = flag.Int(
	"webListenPort",
	8080,
	"port for the web server to listen on",
)

var callbacksURLString = flag.String(
	"callbacksURL",
	"http://127.0.0.1:8080",
	"URL used for callbacks to reach the ATC (excluding basic auth)",
)

var debugListenAddress = flag.String(
	"debugListenAddress",
	"127.0.0.1",
	"address for the pprof debugger listen on",
)

var debugListenPort = flag.Int(
	"debugListenPort",
	8079,
	"port for the pprof debugger to listen on",
)

var httpUsername = flag.String(
	"httpUsername",
	"",
	"basic auth username for the server",
)

var httpHashedPassword = flag.String(
	"httpHashedPassword",
	"",
	"basic auth password for the server",
)

var checkInterval = flag.Duration(
	"checkInterval",
	1*time.Minute,
	"interval on which to poll for new versions of resources",
)

var publiclyViewable = flag.Bool(
	"publiclyViewable",
	false,
	"allow viewability without authentication (destructive operations still require auth)",
)

var dev = flag.Bool(
	"dev",
	false,
	"dev mode; lax security",
)

var noop = flag.Bool(
	"noop",
	false,
	"don't trigger any builds automatically",
)

func main() {
	flag.Parse()

	if !*dev && (*httpUsername == "" || *httpHashedPassword == "") {
		fatal(errors.New("must specify -httpUsername and -httpHashedPassword or turn on dev mode"))
	}

	if _, err := os.Stat(*templatesDir); err != nil {
		fatal(errors.New("directory specified via -templates does not exist"))
	}

	if _, err := os.Stat(*publicDir); err != nil {
		fatal(errors.New("directory specified via -public does not exist"))
	}

	logger := lager.NewLogger("atc")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	var err error

	var dbConn *sql.DB

	for {
		dbConn, err = migration.Open(*sqlDriver, *sqlDataSource, migrations.Migrations)
		if err != nil {
			if strings.Contains(err.Error(), " dial ") {
				logger.Error("failed-to-open-db", err)
				time.Sleep(5 * time.Second)
				continue
			}

			fatal(err)
		}

		break
	}

	db := Db.NewSQL(logger.Session("db"), dbConn)

	var configDB Db.ConfigDB

	if *pipelinePath != "" {
		pipelineFile, err := os.Open(*pipelinePath)
		if err != nil {
			fatal(err)
		}

		var conf atc.Config
		err = candiedyaml.NewDecoder(pipelineFile).Decode(&conf)
		if err != nil {
			fatal(err)
		}

		pipelineFile.Close()

		if err := config.ValidateConfig(conf); err != nil {
			fatal(err)
		}

		configDB = Db.StaticConfigDB{
			Config: conf,
		}
	} else {
		configDB = db
	}

	turbineEndpoint := rata.NewRequestGenerator(*turbineURL, turbine.Routes)
	engine := engine.NewTurbine(turbineEndpoint, db)

	tracker := sched.NewTracker(logger.Session("tracker"), engine, db)

	builder := builder.NewBuilder(db, turbineEndpoint)

	scheduler := &sched.Scheduler{
		DB:      db,
		Factory: &factory.BuildFactory{ConfigDB: configDB},
		Builder: builder,
		Tracker: tracker,
		Locker:  db,
		Logger:  logger.Session("scheduler"),
	}

	radar := rdr.NewRadar(logger, db, *checkInterval, db, configDB)

	var webValidator auth.Validator

	if *httpUsername != "" && *httpHashedPassword != "" {
		webValidator = auth.BasicAuthHashedValidator{
			Username:       *httpUsername,
			HashedPassword: *httpHashedPassword,
		}
	} else {
		webValidator = auth.NoopValidator{}
	}

	callbacksURL, err := url.Parse(*callbacksURLString)
	if err != nil {
		fatal(err)
	}

	drain := make(chan struct{})

	apiHandler, err := api.NewHandler(
		logger,
		webValidator,

		db,
		configDB,

		configDB,

		db,
		configDB,

		configDB,

		config.ValidateConfig,
		builder,
		5*time.Second,
		callbacksURL.Host,
		event.NewHandler,
		drain,
	)
	if err != nil {
		fatal(err)
	}

	webHandler, err := web.NewHandler(
		logger,
		webValidator,
		scheduler,
		db,
		configDB,
		*templatesDir,
		*publicDir,
		drain,
	)
	if err != nil {
		fatal(err)
	}

	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", apiHandler)
	webMux.Handle("/", webHandler)

	var httpHandler http.Handler

	httpHandler = webMux

	if !*publiclyViewable {
		httpHandler = auth.Handler{
			Handler:   httpHandler,
			Validator: webValidator,
		}
	}

	// copy Authorization header as ATC-Authorization cookie for websocket auth
	httpHandler = auth.CookieSetHandler{
		Handler: httpHandler,
	}

	webListenAddr := fmt.Sprintf("%s:%d", *webListenAddress, *webListenPort)
	debugListenAddr := fmt.Sprintf("%s:%d", *debugListenAddress, *debugListenPort)

	group := grouper.NewParallel(os.Interrupt, []grouper.Member{
		{"web", http_server.New(webListenAddr, httpHandler)},

		{"debug", http_server.New(debugListenAddr, http.DefaultServeMux)},

		{"drainer", ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			<-signals

			close(drain)

			return nil
		})},

		{"radar", rdr.NewRunner(
			logger.Session("radar"),
			*noop,
			db,
			radar,
			configDB,
			1*time.Minute,
			turbineEndpoint,
		)},

		{"scheduler", &sched.Runner{
			Logger: logger.Session("scheduler"),

			Locker:   db,
			ConfigDB: configDB,

			Scheduler: scheduler,

			Noop: *noop,

			Interval: 10 * time.Second,
		}},
	})

	running := ifrit.Envoke(sigmon.New(group))

	logger.Info("listening", lager.Data{
		"web":   webListenAddr,
		"debug": debugListenAddr,
	})

	err = <-running.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}
}

func fatal(err error) {
	println(err.Error())
	os.Exit(1)
}
