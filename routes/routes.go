package routes

import (
	"fmt"

	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

const (
	Index           = "Index"
	TriggerBuild    = "TriggerBuild"
	GetBuild        = "GetBuild"
	GetBuilds       = "GetBuilds"
	GetJoblessBuild = "GetJoblessBuild"
	AbortBuild      = "AbortBuild"
	Public          = "Public"
	GetResource     = "GetResource"
	GetJob          = "GetJob"
	LogIn           = "LogIn"
)

var Routes = rata.Routes{
	// public
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/jobs/:job", Method: "GET", Name: GetJob},
	{Path: "/resources/:resource", Method: "GET", Name: GetResource},
	{Path: "/public/:filename", Method: "GET", Name: Public},
	{Path: "/public/fonts/:filename", Method: "GET", Name: Public},

	// public jobs only
	{Path: "/jobs/:job/builds/:build", Method: "GET", Name: GetBuild},

	// private
	{Path: "/login", Method: "GET", Name: LogIn},
	{Path: "/jobs/:job/builds", Method: "POST", Name: TriggerBuild},
	{Path: "/builds/:build_id/abort", Method: "POST", Name: AbortBuild},
	{Path: "/builds", Method: "GET", Name: GetBuilds},
	{Path: "/builds/:build_id", Method: "GET", Name: GetJoblessBuild},
}

func PathForBuild(build db.Build) string {
	var path string
	if build.OneOff() {
		path, _ = Routes.CreatePathForRoute(GetJoblessBuild, rata.Params{
			"build_id": fmt.Sprintf("%d", build.ID),
		})
	} else {
		path, _ = Routes.CreatePathForRoute(GetBuild, rata.Params{
			"job":   build.JobName,
			"build": build.Name,
		})
	}

	return path
}
