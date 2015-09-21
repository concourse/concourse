package getbuilds

import (
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
)

type PresentedBuild struct {
	ID           int
	JobName      string
	PipelineName string
	Status       string
	BuildName    string

	StartTime string
	EndTime   string

	CSSClass string
	Path     string
}

func formatTime(date time.Time) string {
	if date.IsZero() {
		return "n/a"
	}

	const layout = "2006-01-02 15:04:05 (MST)"
	return date.Format(layout)
}

func PresentBuilds(builds []db.Build) []PresentedBuild {
	presentedBuilds := []PresentedBuild{}

	for _, build := range builds {
		var cssClass string
		var jobName string
		var pipelineName string

		if build.OneOff() {
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
			StartTime:    formatTime(build.StartTime),
			EndTime:      formatTime(build.EndTime),
			CSSClass:     cssClass,
			Status:       string(build.Status),
			Path:         routes.PathForBuild(build),
		})
	}

	return presentedBuilds
}
