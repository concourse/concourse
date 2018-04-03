package reaper

import "github.com/tedsuo/rata"

const (
	DestroyContainers = "DestroyContainers"
)

var Routes = rata.Routes{
	{Path: "/containers/destroy", Method: "DELETE", Name: DestroyContainers},
}
