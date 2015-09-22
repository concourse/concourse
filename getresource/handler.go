package getresource

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"
	"github.com/concourse/atc/web/pagination"
	"github.com/pivotal-golang/lager"
)

type server struct {
	logger lager.Logger

	validator auth.Validator

	template *template.Template
}

func NewServer(logger lager.Logger, template *template.Template, validator auth.Validator) *server {
	return &server{
		logger: logger,

		validator: validator,

		template: template,
	}
}

type TemplateData struct {
	Resource atc.Resource
	History  []*db.VersionHistory

	FailingToCheck bool
	CheckError     error

	GroupStates  []group.State
	PipelineName string

	PaginationData pagination.PaginationData
}

//go:generate counterfeiter . ResourcesDB

type ResourcesDB interface {
	GetPipelineName() string
	GetConfig() (atc.Config, db.ConfigVersion, bool, error)
	GetResource(string) (db.SavedResource, error)
	GetResourceHistoryCursor(string, int, bool, int) ([]*db.VersionHistory, bool, error)
	GetResourceHistoryMaxID(int) (int, error)
}

var ErrResourceConfigNotFound = errors.New("could not find resource")

func FetchTemplateData(resourceDB ResourcesDB, authenticated bool, resourceName string, startingID int, resultsGreaterThanStartingID bool) (TemplateData, error) {
	config, _, found, err := resourceDB.GetConfig()
	if err != nil {
		return TemplateData{}, err
	}

	if !found {
		return TemplateData{}, ErrResourceConfigNotFound
	}

	configResource, found := config.Resources.Lookup(resourceName)
	if !found {
		return TemplateData{}, ErrResourceConfigNotFound
	}

	dbResource, err := resourceDB.GetResource(configResource.Name)
	if err != nil {
		return TemplateData{}, err
	}

	maxID, err := resourceDB.GetResourceHistoryMaxID(dbResource.ID)
	if err != nil {
		return TemplateData{}, err
	}

	if startingID == 0 && !resultsGreaterThanStartingID {
		startingID = maxID
	}

	history, moreResultsInGivenDirection, err := resourceDB.GetResourceHistoryCursor(configResource.Name, startingID, resultsGreaterThanStartingID, 100)
	if err != nil {
		return TemplateData{}, err
	}

	var paginationData pagination.PaginationData

	if len(history) > 0 {
		paginationData = pagination.NewPaginationData(
			resultsGreaterThanStartingID,
			moreResultsInGivenDirection,
			maxID,
			history[0].VersionedResource.ID,
			history[len(history)-1].VersionedResource.ID,
		)
	} else {
		paginationData = pagination.PaginationData{}
	}

	resource := present.Resource(configResource, config.Groups, dbResource, authenticated)

	templateData := TemplateData{
		Resource:       resource,
		History:        history,
		PaginationData: paginationData,
		PipelineName:   resourceDB.GetPipelineName(),
		GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
			for _, groupResource := range g.Resources {
				if groupResource == configResource.Name {
					return true
				}
			}

			return false
		}),
	}

	return templateData, nil
}

func (server *server) GetResource(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := server.logger.Session("get-resource")

		resourceName := r.FormValue(":resource")

		startingID, parseErr := strconv.Atoi(r.FormValue("id"))
		if parseErr != nil {
			session.Info("cannot-parse-id-to-int", lager.Data{"id": r.FormValue("id")})
			startingID = 0
		}

		resultsGreaterThanStartingID, parseErr := strconv.ParseBool(r.FormValue("newer"))
		if parseErr != nil {
			resultsGreaterThanStartingID = false
			session.Info("cannot-parse-newer-to-bool", lager.Data{"newer": r.FormValue("newer")})
		}

		authenticated := server.validator.IsAuthenticated(r)
		templateData, err := FetchTemplateData(pipelineDB, authenticated, resourceName, startingID, resultsGreaterThanStartingID)

		switch err {
		case ErrResourceConfigNotFound:
			session.Error("could-not-find-resource-in-config", ErrResourceConfigNotFound, lager.Data{
				"resource": resourceName,
			})
			w.WriteHeader(http.StatusNotFound)
			return
		case nil:
			break
		default:
			session.Error("failed-to-build-template-data", err, lager.Data{
				"resource": resourceName,
			})
			http.Error(w, "failed to fetch resources", http.StatusInternalServerError)
			return
		}

		err = server.template.Execute(w, templateData)
		if err != nil {
			session.Fatal("failed-to-task-template", err, lager.Data{
				"template-data": templateData,
			})
		}
	})
}
