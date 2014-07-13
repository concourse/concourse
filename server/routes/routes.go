package routes

import "github.com/tedsuo/rata"

const (
	Index        = "Index"
	TriggerBuild = "TriggerBuild"
	GetBuild     = "GetBuild"
	AbortBuild   = "AbortBuild"
	Public       = "Public"
	LogOutput    = "LogOutput"
)

var Routes = rata.Routes{
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/jobs/:job/builds", Method: "POST", Name: TriggerBuild},
	{Path: "/jobs/:job/builds/:build", Method: "GET", Name: GetBuild},
	{Path: "/jobs/:job/builds/:build/abort", Method: "POST", Name: AbortBuild},
	{Path: "/jobs/:job/builds/:build/log", Method: "GET", Name: LogOutput},
	{Path: "/public/:filename", Method: "GET", Name: Public},
}
