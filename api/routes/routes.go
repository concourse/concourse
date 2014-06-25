package routes

import "github.com/tedsuo/router"

const (
	UpdateBuild = "UpdateBuild"

	LogInput = "LogInput"
)

var Routes = router.Routes{
	{Path: "/builds/:job/:build", Method: "PUT", Handler: UpdateBuild},

	{Path: "/builds/:job/:build/log/input", Method: "GET", Handler: LogInput},
}
