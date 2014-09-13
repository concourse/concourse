package routes

import "github.com/tedsuo/rata"

const (
	Index        = "Index"
	TriggerBuild = "TriggerBuild"
	GetBuild     = "GetBuild"
	AbortBuild   = "AbortBuild"
	Public       = "Public"
	LogOutput    = "LogOutput"
	GetResource  = "GetResource"
)

var Routes = rata.Routes{
	// public
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/resources/:resource", Method: "GET", Name: GetResource},
	{Path: "/public/:filename", Method: "GET", Name: Public},

	// public jobs only
	{Path: "/jobs/:job/builds/:build", Method: "GET", Name: GetBuild},
	{Path: "/builds/:build_id/log", Method: "GET", Name: LogOutput},

	// private
	{Path: "/jobs/:job/builds", Method: "POST", Name: TriggerBuild},
	{Path: "/builds/:build_id/abort", Method: "POST", Name: AbortBuild},
}
