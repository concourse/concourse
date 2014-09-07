package web

import (
	"fmt"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

type templateFuncs struct {
	peerAddr string
}

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job": jobName(args[0]),
		})

	case routes.GetBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job":   jobName(args[0]),
			"build": args[1].(builds.Build).Name,
		})

	case routes.AbortBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(builds.Build).ID),
		})

	case routes.Public:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"filename": args[0].(string),
		})

	case routes.LogOutput:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(builds.Build).ID),
		})

	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}

func jobName(x interface{}) string {
	switch v := x.(type) {
	case string:
		return v
	default:
		return x.(config.Job).Name
	}
}
