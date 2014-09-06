package routes

import "github.com/tedsuo/rata"

const (
	UpdateBuild  = "UpdateBuild"
	RecordEvents = "RecordEvents"
)

var Routes = rata.Routes{
	{Path: "/builds/:job/:build", Method: "PUT", Name: UpdateBuild},
	{Path: "/builds/:job/:build/events", Method: "GET", Name: RecordEvents},
}
