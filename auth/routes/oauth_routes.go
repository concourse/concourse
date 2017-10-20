package routes

import "github.com/tedsuo/rata"

const (
	OAuthBegin     = "OAuthBegin"
	OAuthCallback  = "OAuthCallback"
	OAuth1Begin    = "OAuth1Begin"
	OAuth1Callback = "OAuth1Callback"
	LogOut         = "LogOut"
)

var OAuthRoutes = rata.Routes{
	{Path: "/auth/logout", Method: "GET", Name: LogOut},
	{Path: "/auth/:provider/callback", Method: "GET", Name: OAuthCallback},
	{Path: "/auth/:provider", Method: "GET", Name: OAuthBegin},
}

var OAuth1Routes = rata.Routes{
	{Path: "/oauth/v1/:provider/callback", Method: "GET", Name: OAuth1Callback},
	{Path: "/oauth/v1/:provider", Method: "GET", Name: OAuth1Begin},
}
