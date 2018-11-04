package ccserver

import (
	"encoding/xml"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

type Project struct {
	Name			string `xml:"name,attr"`
	LastBuildStatus	string `xml:"lastBuildStatus,attr"`
}

type ProjectsContainer struct {
	XMLName   xml.Name `xml:"Projects"`
	Projects  []Project `xml:"Project"`
}

func (s *Server) GetCC(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-cc")
	teamName := rata.Param(r, "team_name")

	team, found, err := s.teamFactory.FindTeam(teamName)

	if err != nil {
		logger.Error("failed-to-find-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Debug("team-not-found", lager.Data{"team": teamName})
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pipelines, err := team.Pipelines()

	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var projects []Project

	for _, pipeline := range pipelines {
		jobs, err := pipeline.Jobs()
		if err != nil {
			logger.Error("failed-to-get-jobs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, job := range jobs {
			build, _, err := job.FinishedAndNextBuild()

			if err != nil {
				logger.Error("failed-to-get-finished-build", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if build != nil {
				projects = append(projects, buildProject(build, pipeline, job))
			}
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	err = xml.NewEncoder(w).Encode(ProjectsContainer{Projects: projects})

	if err != nil {
		logger.Error("failed-to-serialize-projects", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func buildProject(build db.Build, pipeline db.Pipeline, job db.Job) Project {
	var lastBuildStatus string
	switch {
	case build.Status() == db.BuildStatusSucceeded:
		lastBuildStatus = "Success"
	case build.Status() == db.BuildStatusFailed:
		lastBuildStatus = "Failure"
	case build.Status() == db.BuildStatusErrored:
		lastBuildStatus = "Exception"
	default:
		lastBuildStatus = "Unknown"
	}

	projectName := fmt.Sprintf("%s :: %s", pipeline.Name(), job.Config().Name)
	return Project{
		Name:            projectName,
		LastBuildStatus: lastBuildStatus,
	}
}
