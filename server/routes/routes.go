package routes

import "github.com/tedsuo/router"

const (
	Index        = "Index"
	TriggerBuild = "TriggerBuild"
	GetBuild     = "GetBuild"
	AbortBuild   = "AbortBuild"
	Public       = "Public"
	LogOutput    = "LogOutput"
)

var Routes = router.Routes{
	{Path: "/", Method: "GET", Handler: Index},
	{Path: "/jobs/:job/builds", Method: "POST", Handler: TriggerBuild},
	{Path: "/jobs/:job/builds/:build", Method: "GET", Handler: GetBuild},
	{Path: "/jobs/:job/builds/:build/abort", Method: "POST", Handler: AbortBuild},
	{Path: "/jobs/:job/builds/:build/log", Method: "GET", Handler: LogOutput},
	{Path: "/public/:filename", Method: "GET", Handler: Public},
}
