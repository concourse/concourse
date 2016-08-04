package triggerbuild

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/web"
)

type Handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
	}
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	client := handler.clientFactory.Build(r)

	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")
	jobName := r.FormValue(":job")

	team := client.Team(teamName)
	build, err := team.CreateJobBuild(pipelineName, jobName)
	if err != nil {
		handler.logger.Error("failed-to-create-build", err)
		return err
	}

	redirectPath, err := web.Routes.CreatePathForRoute(web.GetBuild, rata.Params{
		"team_name":     teamName,
		"pipeline_name": pipelineName,
		"job":           jobName,
		"build":         build.Name,
	})
	if err != nil {
		handler.logger.Error("failed-to-construct-redirect-uri", err, lager.Data{
			"team":     teamName,
			"pipeline": pipelineName,
			"job":      jobName,
			"build":    build.Name,
		})
		return err
	}

	http.Redirect(w, r, redirectPath, http.StatusFound)

	return nil
}
