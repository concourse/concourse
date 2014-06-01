package routes

import "github.com/tedsuo/router"

const (
	Index        = "Index"
	TriggerBuild = "TriggerBuild"
	GetBuild     = "GetBuild"
	Public       = "Public"
)

var Routes = router.Routes{
	{Path: "/", Method: "GET", Handler: Index},
	{Path: "/jobs/:job/builds", Method: "POST", Handler: TriggerBuild},
	{Path: "/jobs/:job/builds/:build", Method: "GET", Handler: GetBuild},
	{Path: "/public/:filename", Method: "GET", Handler: Public},
}
