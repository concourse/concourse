package api

import "github.com/tedsuo/rata"

const (
	DestroyContainers = "DestroyContainers"
	Ping              = "Ping"
)

var Routes = rata.Routes{
	{Path: "/containers/destroy", Method: "DELETE", Name: DestroyContainers},
	{Path: "/ping", Method: "GET", Name: Ping},
}
