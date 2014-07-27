package main

import (
	"errors"
	"flag"
	"os"
	"time"

	"github.com/BurntSushi/migration"
	turbineroutes "github.com/concourse/turbine/routes"
	"github.com/fraenkel/candiedyaml"
	_ "github.com/lib/pq"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api"
	apiroutes "github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/config"
	Db "github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/logfanout"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/resources"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/server"
	"github.com/concourse/atc/server/auth"
)

var configPath = flag.String(
	"config",
	"",
	"path to atc server config .yml",
)

var templatesDir = flag.String(
	"templates",
	"",
	"path to directory containing the html templates",
)

var publicDir = flag.String(
	"public",
	"",
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

var peerAddr = flag.String(
	"peerAddr",
	"127.0.0.1:8081",
	"external address of the api server, used for callbacks",
)

var listenAddr = flag.String(
	"listenAddr",
	":8080",
	"port for the web server to listen on",
)

var apiListenAddr = flag.String(
	"apiListenAddr",
	":8081",
	"port for the api to listen on",
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

func main() {
	flag.Parse()

	if *configPath == "" {
		fatal(errors.New("must specify -config"))
	}

	if *templatesDir == "" {
		fatal(errors.New("must specify -templates"))
	}

	if *publicDir == "" {
		fatal(errors.New("must specify -public"))
	}

	configFile, err := os.Open(*configPath)
	if err != nil {
		fatal(err)
	}

	var conf config.Config
	err = candiedyaml.NewDecoder(configFile).Decode(&conf)
	if err != nil {
		fatal(err)
	}

	configFile.Close()

	dbConn, err := migration.Open(*sqlDriver, *sqlDataSource, migrations.Migrations)
	if err != nil {
		fatal(err)
	}

	db := Db.NewSQL(dbConn)

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

	atcEndpoint := rata.NewRequestGenerator("http://"+*peerAddr, apiroutes.Routes)
	turbineEndpoint := rata.NewRequestGenerator(*turbineURL, turbineroutes.Routes)
	builder := builder.NewBuilder(db, conf.Resources, turbineEndpoint, atcEndpoint)

	logger := lager.NewLogger("atc")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	tracker := logfanout.NewTracker(db)

	serverHandler, err := server.New(
		logger,
		conf,
		builder,
		db,
		*templatesDir,
		*publicDir,
		*peerAddr,
		tracker,
	)
	if err != nil {
		fatal(err)
	}

	if *httpUsername != "" && *httpHashedPassword != "" {
		serverHandler = auth.Handler{
			Handler:        serverHandler,
			Username:       *httpUsername,
			HashedPassword: *httpHashedPassword,
		}
	}

	apiHandler, err := api.New(logger, db, tracker)
	if err != nil {
		fatal(err)
	}

	turbineChecker := resources.NewTurbineChecker(turbineEndpoint)

	radar := radar.NewRadar(logger, turbineChecker, db, *checkInterval)

	scheduler := &scheduler.Scheduler{
		DB:      db,
		Builder: builder,
		Logger:  logger.Session("scheduler"),
	}

	group := grouper.EnvokeGroup(grouper.RunGroup{
		"web": http_server.New(*listenAddr, serverHandler),

		"api": http_server.New(*apiListenAddr, apiHandler),

		"radar": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			for _, resource := range conf.Resources {
				radar.Scan(resource)
			}

			close(ready)

			<-signals

			radar.Stop()

			return nil
		}),

		"scheduler": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)

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
	})

	running := ifrit.Envoke(sigmon.New(group))

	logger.Info("listening", lager.Data{
		"web": *listenAddr,
		"api": *apiListenAddr,
	})

	err = <-running.Wait()
	if err == nil {
		logger.Info("exited")
	} else {
		logger.Error("failed", err)
		os.Exit(1)
	}
}

func fatal(err error) {
	println(err.Error())
	os.Exit(1)
}
