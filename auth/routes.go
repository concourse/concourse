package auth

import "github.com/tedsuo/rata"

const (
	OAuthBegin      = "OAuthBegin"
	OAuthCallback   = "OAuthCallback"
	OAuthV1Begin    = "OAuthV1Begin"
	OAuthV1Callback = "OAuthV1Callback"
	LogOut          = "LogOut"
)

var Routes = rata.Routes{
	{Path: "/auth/logout", Method: "GET", Name: LogOut},
	{Path: "/auth/:provider/callback", Method: "GET", Name: OAuthCallback},
	{Path: "/auth/:provider", Method: "GET", Name: OAuthBegin},
}

var V1Routes = rata.Routes{
	{Path: "/oauth/v1/:provider/callback", Method: "GET", Name: OAuthV1Callback},
	{Path: "/oauth/v1/:provider", Method: "GET", Name: OAuthV1Begin},
}
