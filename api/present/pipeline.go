package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Pipeline(savedPipeline db.SavedPipeline) atc.Pipeline {
	pathForRoute, err := routes.Routes.CreatePathForRoute(routes.Pipeline, rata.Params{
		"pipeline_name": savedPipeline.Name,
	})

	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	return atc.Pipeline{
		Name: savedPipeline.Name,
		URL:  pathForRoute,
	}
}
