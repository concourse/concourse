package getbuilds

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
)

const layout = "2006-01-02 15:04:05 (MST)"

type PresentedBuild struct {
	ID           int
	JobName      string
	PipelineName string
	TeamName     string
	Status       string
	BuildName    string

	StartTime string
	EndTime   string

	CSSClass string
	Path     string
}

func formatTime(uts int64) string {
	if uts == 0 {
		return "n/a"
	}

	convertedTime := time.Unix(uts, 0).UTC()
	return convertedTime.Format(layout)
}

func PresentBuilds(builds []atc.Build) []PresentedBuild {
	presentedBuilds := []PresentedBuild{}

	for _, build := range builds {
		var cssClass string
		var jobName string
		var pipelineName string

		if build.JobName == "" {
			jobName = "[one off]"
			pipelineName = "[one off]"
			cssClass = "build-one-off"
		} else {
			jobName = build.JobName
			pipelineName = build.PipelineName
		}

		presentedBuilds = append(presentedBuilds, PresentedBuild{
			ID:           build.ID,
			JobName:      jobName,
			PipelineName: pipelineName,
			TeamName:     build.TeamName,
			StartTime:    formatTime(build.StartTime),
			EndTime:      formatTime(build.EndTime),
			CSSClass:     cssClass,
			Status:       string(build.Status),
			Path:         web.PathForBuild(build),
		})
	}

	return presentedBuilds
}
