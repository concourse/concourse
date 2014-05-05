package routes

import "github.com/tedsuo/router"

const (
	SetResult = "SetResult"
	LogInput  = "LogInput"

	TriggerBuild = "TriggerBuild"
)

var Routes = router.Routes{
	{Path: "/builds/:job/:build/result", Method: "PUT", Handler: SetResult},
	{Path: "/builds/:job/:build/log/input", Method: "GET", Handler: LogInput},
}
