package auth

import "github.com/tedsuo/rata"

const (
	OAuthBegin    = "OAuthBegin"
	OAuthCallback = "OAuthCallback"
)

var OAuthRoutes = rata.Routes{
	{Path: "/auth/:provider", Method: "GET", Name: OAuthBegin},
	{Path: "/auth/:provider/callback", Method: "GET", Name: OAuthCallback},
}
