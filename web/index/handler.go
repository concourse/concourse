package index

import (
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
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
	GroupStates []GroupState
	Groups      map[string]bool
}

type GroupState struct {
	Name    string
	Enabled bool
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config, err := handler.configDB.GetConfig()
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

	groupStates := make([]GroupState, len(config.Groups))
	for i, group := range config.Groups {
		groupStates[i] = GroupState{
			Name:    group.Name,
			Enabled: groups[group.Name],
		}
	}

	data := TemplateData{
		Groups:      groups,
		GroupStates: groupStates,
	}

	log := handler.logger.Session("index")

	err = handler.template.Execute(w, data)
	if err != nil {
		log.Fatal("failed-to-execute-template", err, lager.Data{
			"template-data": data,
		})
	}
}
