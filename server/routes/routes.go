package routes

import "github.com/tedsuo/router"

const (
	SetResult = "SetResult"
	LogInput  = "LogInput"

	TriggerBuild = "TriggerBuild"
)

var Routes = router.Routes{
	{Path: "/jobs/:job/builds", Method: "POST", Handler: TriggerBuild},
}
