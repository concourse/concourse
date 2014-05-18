package routes

import "github.com/tedsuo/router"

const (
	UpdateBuild = "UpdateBuild"

	LogInput  = "LogInput"
	LogOutput = "LogOutput"
)

var Routes = router.Routes{
	{Path: "/builds/:job/:build", Method: "PUT", Handler: UpdateBuild},

	{Path: "/builds/:job/:build/log/input", Method: "GET", Handler: LogInput},
	{Path: "/builds/:job/:build/log/output", Method: "GET", Handler: LogOutput},
}
