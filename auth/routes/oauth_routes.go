package routes

import "github.com/tedsuo/rata"

const (
	OAuthBegin    = "OAuthBegin"
	OAuthCallback = "OAuthCallback"
	LogOut        = "LogOut"
	Token         = "Token"
)

var OAuthRoutes = rata.Routes{
	{Path: "/auth/logout", Method: "GET", Name: LogOut},
	{Path: "/auth/:provider/callback", Method: "GET", Name: OAuthCallback},
	{Path: "/auth/:provider", Method: "GET", Name: OAuthBegin},
	{Path: "/auth/:provider/token", Method: "POST", Name: Token},
}
