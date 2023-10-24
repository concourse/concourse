package ccserver

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
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
	var pipelines []db.Pipeline
	acc := accessor.GetAccessor(r)

	if acc.IsAuthorized(teamName) {
		pipelines, err = team.Pipelines()
	} else {
		pipelines, err = team.PublicPipelines()
	}

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

func (s *Server) buildProject(j atc.JobSummary) Project {
	var lastBuildStatus string
	switch {
	case db.BuildStatus(j.FinishedBuild.Status) == db.BuildStatusSucceeded:
		lastBuildStatus = "Success"
	case db.BuildStatus(j.FinishedBuild.Status) == db.BuildStatusFailed:
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

	pipelineRef := atc.PipelineRef{
		Name:         j.PipelineName,
		InstanceVars: j.PipelineInstanceVars,
	}
	return Project{
		Activity:        activity,
		LastBuildLabel:  fmt.Sprint(j.FinishedBuild.Name),
		LastBuildStatus: lastBuildStatus,
		LastBuildTime:   time.Unix(j.FinishedBuild.EndTime, 0).UTC().Format(time.RFC3339),
		Name:            fmt.Sprintf("%s/%s", pipelineRef.String(), j.Name),
		WebUrl:          s.createWebUrl(j.TeamName, pipelineRef, j.Name),
	}
}

func (s *Server) createWebUrl(teamName string, pipelineRef atc.PipelineRef, jobName string) string {
	externalURL, err := url.Parse(s.externalURL)
	if err != nil {
		fmt.Println("Could not parse externalURL")
	}

	queryParams := pipelineRef.QueryParams().Encode()
	if queryParams != "" {
		queryParams = "?" + queryParams
	}
	pipelineURL, err := url.Parse("/teams/" + teamName + "/pipelines/" + pipelineRef.Name + "/jobs/" + jobName + queryParams)
	if err != nil {
		fmt.Println("Could not parse pipelineURL")
	}

	return externalURL.ResolveReference(pipelineURL).String()
}
