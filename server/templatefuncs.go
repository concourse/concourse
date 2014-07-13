package server

import (
	"fmt"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/server/routes"
	"github.com/tedsuo/rata"
)

type templateFuncs struct {
	peerAddr string
}

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job": args[0].(config.Job).Name,
		})

	case routes.GetBuild, routes.AbortBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job":   args[0].(config.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})

	case routes.Public:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"filename": args[0].(string),
		})

	case routes.LogOutput:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job":   args[0].(config.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})

	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}
