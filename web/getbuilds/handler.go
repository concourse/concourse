package getbuilds

import (
	"html/template"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
)

type Handler struct {
	logger lager.Logger

	clientFactory web.ClientFactory

	template *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *Handler {
	return &Handler{
		logger: logger,

		clientFactory: clientFactory,

		template: template,
	}
}

type TemplateData struct {
	Builds []PresentedBuild

	Pagination concourse.Pagination
}

func FetchTemplateData(client concourse.Client, page concourse.Page) (TemplateData, error) {
	builds, pagination, err := client.Builds(page)
	if err != nil {
		return TemplateData{}, err
	}

	return TemplateData{
		Builds:     PresentBuilds(builds),
		Pagination: pagination,
	}, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	client := handler.clientFactory.Build(r)

	log := handler.logger.Session("builds")

	since, parseErr := strconv.Atoi(r.FormValue("since"))
	if parseErr != nil {
		since = 0
	}

	until, parseErr := strconv.Atoi(r.FormValue("until"))
	if parseErr != nil {
		until = 0
	}

	page := concourse.Page{
		Since: since,
		Until: until,
		Limit: atc.PaginationWebLimit,
	}

	templateData, err := FetchTemplateData(client, page)
	if err != nil {
		log.Error("failed-to-build-template-data", err)
		return err
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-build-template", err)
		return err
	}

	return nil
}
