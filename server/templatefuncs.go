package server

import (
	"fmt"
	"net"

	"github.com/tedsuo/router"
	apiroutes "github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/server/routes"
)

type templateFuncs struct {
	peerAddr string
}

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild, routes.GetJob:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job": args[0].(config.Job).Name,
		})

	case routes.GetBuild:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job":   args[0].(config.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})

	case routes.Public:
		return routes.Routes.PathForHandler(handler, router.Params{
			"filename": args[0].(string),
		})

	case apiroutes.LogOutput:
		path, err := apiroutes.Routes.PathForHandler(handler, router.Params{
			"job":   args[0].(config.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})
		if err != nil {
			return "", err
		}

		_, port, err := net.SplitHostPort(funcs.peerAddr)
		if err != nil {
			port = "80"
		}

		return ":" + port + path, nil

	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}
