package routes

import "github.com/tedsuo/rata"

const (
	UpdateBuild  = "UpdateBuild"
	RecordEvents = "RecordEvents"
)

var Routes = rata.Routes{
	{Path: "/builds/:build", Method: "PUT", Name: UpdateBuild},
	{Path: "/builds/:build/events", Method: "GET", Name: RecordEvents},
}
