package routes

import "github.com/tedsuo/rata"

const (
	Index        = "Index"
	TriggerBuild = "TriggerBuild"
	GetBuild     = "GetBuild"
	AbortBuild   = "AbortBuild"
	Public       = "Public"
	GetResource  = "GetResource"
	GetJob       = "GetJob"
	LogIn        = "LogIn"
)

var Routes = rata.Routes{
	// public
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/jobs/:job", Method: "GET", Name: GetJob},
	{Path: "/resources/:resource", Method: "GET", Name: GetResource},
	{Path: "/public/:filename", Method: "GET", Name: Public},
	{Path: "/public/fonts/:filename", Method: "GET", Name: Public},

	// public jobs only
	{Path: "/jobs/:job/builds/:build", Method: "GET", Name: GetBuild},

	// private
	{Path: "/login", Method: "GET", Name: LogIn},
	{Path: "/jobs/:job/builds", Method: "POST", Name: TriggerBuild},
	{Path: "/builds/:build_id/abort", Method: "POST", Name: AbortBuild},
}
