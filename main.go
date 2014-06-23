package main

import (
	"errors"
	"flag"
	"os"
	"time"

	turbineroutes "github.com/concourse/turbine/routes"
	"github.com/fraenkel/candiedyaml"
	"github.com/garyburd/redigo/redis"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/drainer"
	apiroutes "github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/queue"
	"github.com/concourse/atc/server"
	"github.com/concourse/atc/server/auth"
	"github.com/concourse/atc/watcher"
	"github.com/concourse/atc/watchman"
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

	atcEndpoint := router.NewRequestGenerator("http://"+*peerAddr, apiroutes.Routes)
	turbineEndpoint := router.NewRequestGenerator(*turbineURL, turbineroutes.Routes)
	builder := builder.NewBuilder(redisDB, conf.Resources, turbineEndpoint, atcEndpoint)

	logger := lager.NewLogger("atc")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	queuer := queue.NewQueue(logger, 10*time.Second, builder)

	serverHandler, err := server.New(
		logger,
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

	if *httpUsername != "" && *httpHashedPassword != "" {
		serverHandler = auth.Handler{
			Handler:        serverHandler,
			Username:       *httpUsername,
			HashedPassword: *httpHashedPassword,
		}
	}

	drainer := drainer.NewDrainer()

	apiHandler, err := api.New(logger, redisDB, drainer)
	if err != nil {
		fatal(err)
	}

	watchman := watchman.NewWatchman(logger, redisDB, queuer)

	group := grouper.EnvokeGroup(grouper.RunGroup{
		"web":     http_server.New(*listenAddr, serverHandler),
		"api":     http_server.New(*apiListenAddr, apiHandler),
		"watcher": watcher.NewWatcher(conf.Jobs, conf.Resources, redisDB, turbineEndpoint, watchman),
		"drainer": ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			<-signals
			drainer.Drain()
			return nil
		}),
	})

	running := ifrit.Envoke(sigmon.New(group))

	logger.Info("main", "listening", "", lager.Data{
		"web": *listenAddr,
		"api": *apiListenAddr,
	})

	err = <-running.Wait()
	if err != nil {
		logger.Error("main", "exited", "failure", err)
		return
	}

	logger.Info("main", "exited", "")
}

func fatal(err error) {
	println(err.Error())
	os.Exit(1)
}
