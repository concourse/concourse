package ccserver

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

type Project struct {
	Activity        string `xml:"activity,attr"`
	LastBuildLabel  string `xml:"lastBuildLabel,attr"`
	LastBuildStatus	string `xml:"lastBuildStatus,attr"`
	LastBuildTime   string `xml:"lastBuildTime,attr"`
	Name			string `xml:"name,attr"`
	WebUrl			string `xml:"webUrl,attr"`
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
			build, nextBuild, err := job.FinishedAndNextBuild()

			if err != nil {
				logger.Error("failed-to-get-finished-build", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if build != nil {
				projects = append(projects, s.buildProject(build, nextBuild, pipeline, job))
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

func (s *Server) buildProject(build db.Build, nextBuild db.Build, pipeline db.Pipeline, job db.Job) Project {
	var lastBuildStatus string
	switch {
	case build.Status() == db.BuildStatusSucceeded:
		lastBuildStatus = "Success"
	case build.Status() == db.BuildStatusFailed:
		lastBuildStatus = "Failure"
	default:
		lastBuildStatus = "Exception"
	}

	var activity string
	if nextBuild != nil {
		activity = "Building"
	} else {
		activity = "Sleeping"
	}

	webUrl := s.createWebUrl([]string{
		"teams",
		pipeline.TeamName(),
		"pipelines",
		pipeline.Name(),
		"jobs",
		job.Name(),
	})

	projectName := fmt.Sprintf("%s :: %s", pipeline.Name(), job.Name())
	return Project{
		Activity:		 activity,
		LastBuildLabel:  fmt.Sprint(build.ID()),
		LastBuildStatus: lastBuildStatus,
		LastBuildTime:   build.EndTime().Format(time.RFC3339),
		Name:            projectName,
		WebUrl:			 webUrl,
	}
}

func (s *Server) createWebUrl(pathComponents []string) string {
	for i, c := range pathComponents {
		pathComponents[i] = url.PathEscape(c)
	}

	return s.externalURL + "/" + strings.Join(pathComponents, "/")
}
