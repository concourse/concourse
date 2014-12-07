package web

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

type templateFuncs struct{}

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job": jobName(args[0]),
		})

	case routes.GetBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job":   jobName(args[0]),
			"build": args[1].(db.Build).Name,
		})

	case routes.AbortBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(db.Build).ID),
		})

	case routes.Public:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"filename": args[0].(string),
		})

	case atc.BuildEvents:
		return atc.Routes.CreatePathForRoute(handler, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(db.Build).ID),
		})

	case routes.LogIn:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{})

	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}

func jobName(x interface{}) string {
	switch v := x.(type) {
	case string:
		return v
	default:
		return x.(atc.JobConfig).Name
	}
}
