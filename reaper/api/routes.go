package api

import "github.com/tedsuo/rata"

const (
	DestroyContainers = "DestroyContainers"
	List              = "List"
	Ping              = "Ping"
)

var Routes = rata.Routes{
	{Path: "/containers/destroy", Method: "DELETE", Name: DestroyContainers},
	{Path: "/containers/list", Method: "GET", Name: List},
	{Path: "/ping", Method: "GET", Name: Ping},
}
