package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/migration"
	"github.com/fraenkel/candiedyaml"
	_ "github.com/lib/pq"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	troutes "github.com/concourse/turbine/routes"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/callbacks"
	croutes "github.com/concourse/atc/callbacks/routes"
	"github.com/concourse/atc/config"
	Db "github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resources"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/scheduler/factory"
	"github.com/concourse/atc/web"
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

var externalAddress = flag.String(
	"externalAddress",
	"127.0.0.1",
	"external IP of the ATC",
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

var callbacksListenPort = flag.Int(
	"callbacksListenPort",
	8081,
	"port for the internal callbacks server to listen on",
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

var publicViewable = flag.Bool(
	"publicViewable",
	false,
	"allow viewability without authentication (destructive operations still require auth)",
)

var noop = flag.Bool(
	"noop",
	false,
	"don't trigger any builds automatically",
)

func main() {
	flag.Parse()

	if *pipelinePath == "" {
		fatal(errors.New("must specify -pipeline"))
	}

	if _, err := os.Stat(*templatesDir); err != nil {
		fatal(errors.New("directory specified via -templates does not exist"))
	}

	if _, err := os.Stat(*publicDir); err != nil {
		fatal(errors.New("directory specified via -public does not exist"))
	}

	pipelineFile, err := os.Open(*pipelinePath)
	if err != nil {
		fatal(err)
	}

	var conf config.Config
	err = candiedyaml.NewDecoder(pipelineFile).Decode(&conf)
	if err != nil {
		fatal(err)
	}

	pipelineFile.Close()

	logger := lager.NewLogger("atc")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

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

	for _, job := range conf.Jobs {
		err := db.RegisterJob(job.Name)
		if err != nil {
			fatal(err)
		}
	}

	for _, resource := range conf.Resources {
		err := db.RegisterResource(resource.Name)
		if err != nil {
			fatal(err)
		}
	}

	callbacksAddr := fmt.Sprintf("%s:%d", *externalAddress, *callbacksListenPort)
	webAddr := fmt.Sprintf("%s:%d", *externalAddress, *webListenPort)

	callbackEndpoint := rata.NewRequestGenerator("http://"+callbacksAddr, croutes.Routes)
	turbineEndpoint := rata.NewRequestGenerator(*turbineURL, troutes.Routes)
	builder := builder.NewBuilder(db, turbineEndpoint, callbackEndpoint)

	scheduler := &scheduler.Scheduler{
		DB:      db,
		Factory: &factory.BuildFactory{Resources: conf.Resources},
		Builder: builder,
		Logger:  logger.Session("scheduler"),
	}

	tracker := logfanout.NewTracker(db)

	radar := radar.NewRadar(logger, db, *checkInterval)

	var validator auth.Validator

	if *httpUsername != "" && *httpHashedPassword != "" {
		validator = auth.BasicAuthValidator{
			Username:       *httpUsername,
			HashedPassword: *httpHashedPassword,
		}
	} else {
		validator = auth.NoopValidator{}
	}

	apiHandler, err := api.NewHandler(
		logger,
		db,
		db,
		builder,
		tracker,
		5*time.Second,
		webAddr,
	)
	if err != nil {
		fatal(err)
	}

	webHandler, err := web.NewHandler(
		logger,
		validator,
		conf,
		scheduler,
		radar,
		db,
		*templatesDir,
		*publicDir,
		tracker,
	)
	if err != nil {
		fatal(err)
	}

	callbacksHandler, err := callbacks.NewHandler(logger, db, tracker)
	if err != nil {
		fatal(err)
	}

	webMux := http.NewServeMux()
	webMux.Handle("/api/v1/", auth.Handler{Handler: apiHandler, Validator: validator})
	webMux.Handle("/", webHandler)

	var publicHandler http.Handler
	if *publicViewable {
		publicHandler = webMux
	} else {
		publicHandler = auth.Handler{
			Handler:   webMux,
			Validator: validator,
		}
	}

	// copy Authorization header as ATC-Authorization cookie for websocket auth
	publicHandler = auth.CookieSetHandler{
		Handler: publicHandler,
	}

	webListenAddr := fmt.Sprintf("%s:%d", *webListenAddress, *webListenPort)
	callbacksListenAddr := fmt.Sprintf("%s:%d", *externalAddress, *callbacksListenPort)
	debugListenAddr := fmt.Sprintf("%s:%d", *debugListenAddress, *debugListenPort)

	group := grouper.RunGroup{
		"web":       http_server.New(webListenAddr, publicHandler),
		"callbacks": http_server.New(callbacksListenAddr, callbacksHandler),
		"debug":     http_server.New(debugListenAddr, http.DefaultServeMux),

		"radar": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			if *noop {
				close(ready)
				<-signals
				return nil
			}

			for _, resource := range conf.Resources {
				checker := resources.NewTurbineChecker(turbineEndpoint)
				radar.Scan(checker, resource)
			}

			close(ready)

			<-signals

			return nil
		}),

		"scheduler": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

			if *noop {
				<-signals
				return nil
			}

			for {
				select {
				case <-time.After(10 * time.Second):
					for _, job := range conf.Jobs {
						scheduler.TryNextPendingBuild(job)
						scheduler.BuildLatestInputs(job)
					}

				case <-signals:
					return nil
				}
			}

			return nil
		}),

		"drainer": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			<-signals
			tracker.Drain()
			return nil
		}),
	}

	running := ifrit.Envoke(sigmon.New(group))

	logger.Info("listening", lager.Data{
		"web":       webListenAddr,
		"callbacks": callbacksListenAddr,
		"debug":     debugListenAddr,
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
