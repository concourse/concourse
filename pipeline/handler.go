package pipeline

import (
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"
)

type server struct {
	logger lager.Logger

	template *template.Template
}

func NewServer(logger lager.Logger, template *template.Template) *server {
	return &server{
		logger: logger,

		template: template,
	}
}

type TemplateData struct {
	GroupStates  []group.State
	Groups       map[string]bool
	PipelineName string
}

func (server *server) GetPipeline(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, _, err := pipelineDB.GetConfig()
		if err != nil {
			server.logger.Error("failed-to-load-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		groups := map[string]bool{}
		for _, group := range config.Groups {
			groups[group.Name] = false
		}

		enabledGroups, found := r.URL.Query()["groups"]
		if !found && len(config.Groups) > 0 {
			enabledGroups = []string{config.Groups[0].Name}
		}

		for _, name := range enabledGroups {
			groups[name] = true
		}

		data := TemplateData{
			Groups: groups,
			GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
				return groups[g.Name]
			}),
			PipelineName: pipelineDB.GetPipelineName(),
		}

		log := server.logger.Session("index")

		err = server.template.Execute(w, data)
		if err != nil {
			log.Fatal("failed-to-task-template", err, lager.Data{
				"template-data": data,
			})
		}
	})
}
