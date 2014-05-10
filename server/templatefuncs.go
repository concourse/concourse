package server

import (
	"fmt"

	"github.com/tedsuo/router"
	apiroutes "github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/jobs"
	"github.com/winston-ci/winston/server/routes"
)

type templateFuncs struct {
	peerAddr string
}

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild, routes.GetJob:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job": args[0].(jobs.Job).Name,
		})

	case routes.GetBuild:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job":   args[0].(jobs.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})

	case routes.Public:
		return routes.Routes.PathForHandler(handler, router.Params{
			"filename": args[0].(string),
		})

	case apiroutes.LogOutput:
		path, err := apiroutes.Routes.PathForHandler(handler, router.Params{
			"job":   args[0].(jobs.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})
		if err != nil {
			return "", err
		}

		return "ws://" + funcs.peerAddr + path, nil

	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}
