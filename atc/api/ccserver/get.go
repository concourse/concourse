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
	LastBuildStatus string `xml:"lastBuildStatus,attr"`
	LastBuildTime   string `xml:"lastBuildTime,attr"`
	Name            string `xml:"name,attr"`
	WebUrl          string `xml:"webUrl,attr"`
}

type ProjectsContainer struct {
	XMLName  xml.Name  `xml:"Projects"`
	Projects []Project `xml:"Project"`
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
		dashboards, err := pipeline.Dashboard()

		if err != nil {
			logger.Error("failed-to-get-dashboards", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, dashboardJob := range dashboards {
			if dashboardJob.FinishedBuild != nil {
				projects = append(projects, s.buildProject(dashboardJob))
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

func (s *Server) buildProject(j db.DashboardJob) Project {
	var lastBuildStatus string
	switch {
	case j.FinishedBuild.Status() == db.BuildStatusSucceeded:
		lastBuildStatus = "Success"
	case j.FinishedBuild.Status() == db.BuildStatusFailed:
		lastBuildStatus = "Failure"
	default:
		lastBuildStatus = "Exception"
	}

	var activity string
	if j.NextBuild != nil {
		activity = "Building"
	} else {
		activity = "Sleeping"
	}

	webUrl := s.createWebUrl([]string{
		"teams",
		j.Job.TeamName(),
		"pipelines",
		j.Job.PipelineName(),
		"jobs",
		j.Job.Name(),
	})

	projectName := fmt.Sprintf("%s/%s", j.Job.PipelineName(), j.Job.Name())
	return Project{
		Activity:        activity,
		LastBuildLabel:  fmt.Sprint(j.FinishedBuild.Name()),
		LastBuildStatus: lastBuildStatus,
		LastBuildTime:   j.FinishedBuild.EndTime().Format(time.RFC3339),
		Name:            projectName,
		WebUrl:          webUrl,
	}
}

func (s *Server) createWebUrl(pathComponents []string) string {
	for i, c := range pathComponents {
		pathComponents[i] = url.PathEscape(c)
	}

	return s.externalURL + "/" + strings.Join(pathComponents, "/")
}
