package triggerbuild

import (
	"log"
	"net/http"

	"github.com/pivotal-golang/lager"
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

	pipelineName := r.FormValue(":pipeline_name")
	jobName := r.FormValue(":job")

	build, err := client.CreateJobBuild(pipelineName, jobName)
	if err != nil {
		handler.logger.Error("failed-to-create-build", err)
		return err
	}

	redirectPath, err := web.Routes.CreatePathForRoute(web.GetBuild, rata.Params{
		"pipeline_name": pipelineName,
		"job":           jobName,
		"build":         build.Name,
	})
	if err != nil {
		log.Fatal("failed-to-construct-redirect-uri", err, lager.Data{
			"pipeline": pipelineName,
			"job":      jobName,
			"build":    build.Name,
		})
	}

	http.Redirect(w, r, redirectPath, http.StatusFound)

	return nil
}
