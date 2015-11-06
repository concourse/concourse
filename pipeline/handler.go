package pipeline

import (
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
)

type handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) http.Handler {
	return &handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	GroupStates  []group.State
	Groups       map[string]bool
	PipelineName string
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client := handler.clientFactory.Build(r)

	pipelineName := r.FormValue(":pipeline")

	pipeline, found, err := client.Pipeline(pipelineName)
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	groups := map[string]bool{}
	for _, group := range pipeline.Groups {
		groups[group.Name] = false
	}

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
	}

	log := handler.logger.Session("index")

	err = handler.template.Execute(w, data)
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": data,
		})
	}
}
