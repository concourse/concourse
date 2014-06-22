package server

import (
	"fmt"
	"net"

	apiroutes "github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/server/routes"
	"github.com/tedsuo/router"
)

type templateFuncs struct {
	peerAddr string
}

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job": args[0].(config.Job).Name,
		})

	case routes.GetBuild, routes.AbortBuild:
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
