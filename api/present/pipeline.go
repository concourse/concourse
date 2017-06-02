package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Pipeline(savedPipeline db.Pipeline) atc.Pipeline {
	pathForRoute, err := web.Routes.CreatePathForRoute(web.Pipeline, rata.Params{
		"team_name": savedPipeline.TeamName(),
		"pipeline":  savedPipeline.Name(),
	})

	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	config, _, _, err := savedPipeline.Config()
	if err != nil {
		panic("failed to get config: " + err.Error())
	}

	return atc.Pipeline{
		ID:       savedPipeline.ID(),
		Name:     savedPipeline.Name(),
		TeamName: savedPipeline.TeamName(),
		URL:      pathForRoute,
		Paused:   savedPipeline.Paused(),
		Public:   savedPipeline.Public(),
		Groups:   config.Groups,
	}
}
