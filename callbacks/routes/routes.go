package routes

import "github.com/tedsuo/rata"

const (
	UpdateBuild = "UpdateBuild"

	LogInput = "LogInput"
)

var Routes = rata.Routes{
	{Path: "/builds/:job/:build", Method: "PUT", Name: UpdateBuild},
	{Path: "/builds/:job/:build/processes", Method: "POST", Name: UpdateBuild},

	{Path: "/builds/:job/:build/log/input", Method: "GET", Name: LogInput},
}
