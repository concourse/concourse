package index

import (
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"
)

type handler struct {
	logger lager.Logger

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewHandler(logger lager.Logger, db db.DB, configDB db.ConfigDB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		db:       db,
		configDB: configDB,

		template: template,
	}
}

type TemplateData struct {
	GroupStates []group.State
	Groups      map[string]bool
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config, _, err := handler.configDB.GetConfig(atc.DefaultPipelineName)
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
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
	}

	log := handler.logger.Session("index")

	err = handler.template.Execute(w, data)
	if err != nil {
		log.Fatal("failed-to-task-template", err, lager.Data{
			"template-data": data,
		})
	}
}
