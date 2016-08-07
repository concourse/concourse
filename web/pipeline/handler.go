package pipeline

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
)

type Handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	GroupStates  []group.State
	Groups       map[string]bool
	PipelineName string
	TeamName     string
	Elm          bool
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	client := handler.clientFactory.Build(r)

	pipelineName := r.FormValue(":pipeline")
	teamName := r.FormValue(":team_name")

	team := client.Team(teamName)
	pipeline, found, err := team.Pipeline(pipelineName)
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
		return err
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	groups := map[string]bool{}
	for _, group := range pipeline.Groups {
		groups[group.Name] = false
	}

	_, isElm := r.URL.Query()["elm"]

	enabledGroups, found := r.URL.Query()["groups"]
	if !found && len(pipeline.Groups) > 0 {
		enabledGroups = []string{pipeline.Groups[0].Name}
	}

	for _, name := range enabledGroups {
		groups[name] = true
	}

	data := TemplateData{
		Groups: groups,
		GroupStates: group.States(pipeline.Groups, func(g atc.GroupConfig) bool {
			return groups[g.Name]
		}),
		PipelineName: pipelineName,
		TeamName:     teamName,
		Elm:          isElm,
	}

	log := handler.logger.Session("index")

	err = handler.template.Execute(w, data)
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": data,
		})
		return err
	}

	return nil
}
