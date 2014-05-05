package routes

import "github.com/tedsuo/router"

const (
	TriggerBuild = "TriggerBuild"
)

var Routes = router.Routes{
	{Path: "/jobs/:job/builds", Method: "POST", Handler: TriggerBuild},
}
