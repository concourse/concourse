package routes

import "github.com/tedsuo/rata"

const (
	UpdateBuild  = "UpdateBuild"
	RecordEvents = "RecordEvents"
)

var Routes = rata.Routes{
	{Path: "/api/callbacks/builds/:build", Method: "PUT", Name: UpdateBuild},
	{Path: "/api/callbacks/builds/:build/events", Method: "GET", Name: RecordEvents},
}
