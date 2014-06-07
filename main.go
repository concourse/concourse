package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"time"

	"github.com/fraenkel/candiedyaml"
	"github.com/garyburd/redigo/redis"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/router"
	proleroutes "github.com/winston-ci/prole/routes"

	"github.com/winston-ci/winston/api"
	"github.com/winston-ci/winston/api/drainer"
	apiroutes "github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/queue"
	"github.com/winston-ci/winston/server"
	"github.com/winston-ci/winston/watcher"
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
	builder := builder.NewBuilder(redisDB, conf.Resources, proleEndpoint, winstonEndpoint)
	queuer := queue.NewQueue(10*time.Second, builder)

	serverHandler, err := server.New(
		conf,
		redisDB,
		*templatesDir,
		*publicDir,
		*peerAddr,
		queuer,
	)
	if err != nil {
		fatal(err)
	}

	drainer := drainer.NewDrainer()

	apiHandler, err := api.New(redisDB, drainer)
	if err != nil {
		fatal(err)
	}

	watchman := watchman.NewWatchman(redisDB, queuer)

	group := grouper.EnvokeGroup(grouper.RunGroup{
		"web":     http_server.New(*listenAddr, serverHandler),
		"api":     http_server.New(*apiListenAddr, apiHandler),
		"watcher": watcher.NewWatcher(conf.Jobs, conf.Resources, redisDB, proleEndpoint, watchman),
		"drainer": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			<-signals
			drainer.Drain()
			return nil
		}),
	})

	running := ifrit.Envoke(sigmon.New(group))

	log.Println("serving web on", *listenAddr)
	log.Println("serving api on", *apiListenAddr)

	err = <-running.Wait()
	if err != nil {
		fatal(err)
	}

	log.Println("exited")
}

func fatal(err error) {
	println(err.Error())
	os.Exit(1)
}
