package web

import "github.com/tedsuo/rata"

const (
	Index                 = "Index"
	Pipeline              = "Pipeline"
	TriggerBuild          = "TriggerBuild"
	GetBuild              = "GetBuild"
	GetBuilds             = "GetBuilds"
	GetJoblessBuild       = "GetJoblessBuild"
	Public                = "Public"
	GetResource           = "GetResource"
	GetJob                = "GetJob"
	LogIn                 = "LogIn"
	TeamLogIn             = "TeamLogIn"
	ProcessBasicAuthLogIn = "ProcessBasicAuthLogIn"
)

var Routes = rata.Routes{
	// public
	{Path: "/", Method: "GET", Name: Index},
	{Path: "/teams/:team_name/pipelines/:pipeline", Method: "GET", Name: Pipeline},
	{Path: "/teams/:team_name/pipelines/:pipeline_name/jobs/:job", Method: "GET", Name: GetJob},
	{Path: "/teams/:team_name/pipelines/:pipeline_name/resources/:resource", Method: "GET", Name: GetResource},
	{Path: "/public/:filename", Method: "GET", Name: Public},
	{Path: "/public/fonts/:filename", Method: "GET", Name: Public},
	{Path: "/public/favicons/:filename", Method: "GET", Name: Public},
	{Path: "/public/images/:filename", Method: "GET", Name: Public},

	// public jobs only
	{Path: "/teams/:team_name/pipelines/:pipeline_name/jobs/:job/builds/:build", Method: "GET", Name: GetBuild},

	// private
	{Path: "/teams/:team_name/pipelines/:pipeline_name/jobs/:job/builds", Method: "POST", Name: TriggerBuild},
	{Path: "/builds", Method: "GET", Name: GetBuilds},
	{Path: "/builds/:build_id", Method: "GET", Name: GetJoblessBuild},

	// auth
	{Path: "/login", Method: "GET", Name: LogIn},
	{Path: "/teams/:team_name/login", Method: "GET", Name: TeamLogIn},
	{Path: "/teams/:team_name/login", Method: "POST", Name: ProcessBasicAuthLogIn},
}
