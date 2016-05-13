package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Pipeline(teamName string, savedPipeline db.SavedPipeline, config atc.Config) atc.Pipeline {
	pathForRoute, err := web.Routes.CreatePathForRoute(web.Pipeline, rata.Params{
		"team_name": teamName,
		"pipeline":  savedPipeline.Name,
	})

	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	return atc.Pipeline{
		TeamName: teamName,
		Name:     savedPipeline.Name,
		URL:      pathForRoute,
		Paused:   savedPipeline.Paused,
		Groups:   config.Groups,
	}
}
