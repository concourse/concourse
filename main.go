package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fraenkel/candiedyaml"
	"github.com/garyburd/redigo/redis"
	"github.com/tedsuo/router"
	proleroutes "github.com/winston-ci/prole/routes"

	"github.com/winston-ci/winston/api"
	apiroutes "github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/resources"
	"github.com/winston-ci/winston/server"
	"github.com/winston-ci/winston/watchman"
)

var configPath = flag.String(
	"config",
	"",
	"path to winston server config .yml",
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

var proleURL = flag.String(
	"proleURL",
	"http://127.0.0.1:4637",
	"address denoting the prole service",
)

var redisAddr = flag.String(
	"redisAddr",
	"127.0.0.1:6379",
	"redis server address",
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

	redisDB := db.NewRedis(redis.NewPool(func() (redis.Conn, error) {
		return redis.DialTimeout("tcp", *redisAddr, 5*time.Second, 0, 0)
	}, 20))

	winstonEndpoint := router.NewRequestGenerator("http://"+*peerAddr, apiroutes.Routes)
	proleEndpoint := router.NewRequestGenerator(*proleURL, proleroutes.Routes)
	builder := builder.NewBuilder(redisDB, proleEndpoint, winstonEndpoint)

	serverHandler, err := server.New(conf, redisDB, *templatesDir, *publicDir, *peerAddr, builder)
	if err != nil {
		fatal(err)
	}

	apiHandler, err := api.New(redisDB)
	if err != nil {
		fatal(err)
	}

	errs := make(chan error, 2)

	go func() {
		log.Println("serving web on", *listenAddr)
		errs <- http.ListenAndServe(*listenAddr, serverHandler)
	}()

	go func() {
		log.Println("serving api on", *apiListenAddr)
		errs <- http.ListenAndServe(*apiListenAddr, apiHandler)
	}()

	watcher := watchman.NewWatchman(builder)

	for _, job := range conf.Jobs {
		for resourceName, outputJobs := range job.Inputs {
			resource, found := conf.Resources.Lookup(resourceName)
			if !found {
				log.Fatalln("unknown resource:", resourceName)
			}

			current, err := redisDB.GetCurrentSource(job.Name, resourceName)
			if err == nil {
				resource.Source = config.Source(current)
			}

			var checker resources.Checker
			if len(outputJobs) == 0 {
				checker = resources.NewProleChecker(proleEndpoint)
			} else {
				checker = resources.NewWinstonChecker(redisDB, outputJobs)
			}

			watcher.Watch(job, resource, conf.Resources, checker, time.Minute)
		}
	}

	fatal(<-errs)
}

func fatal(err error) {
	println(err.Error())
	os.Exit(1)
}
