package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

func GetBuild(client concourse.Client, team concourse.Team, jobName string, buildNameOrID string, pipelineRef atc.PipelineRef) (atc.Build, error) {
	if buildNameOrID != "" {
		var build atc.Build
		var err error
		var found bool

		if team != nil {
			build, found, err = team.JobBuild(pipelineRef, jobName, buildNameOrID)
		} else {
			build, found, err = client.Build(buildNameOrID)
		}

		if err != nil {
			return atc.Build{}, fmt.Errorf("failed to get build %s", err)
		}

		if !found {
			return atc.Build{}, errors.New("build not found")
		}

		return build, nil
	} else if jobName != "" {
		job, found, err := team.Job(pipelineRef, jobName)

		if err != nil {
			return atc.Build{}, fmt.Errorf("failed to get job %s", err)
		}

		if !found {
			return atc.Build{}, errors.New("job not found")
		}

		if job.NextBuild != nil {
			return *job.NextBuild, nil
		} else if job.FinishedBuild != nil {
			return *job.FinishedBuild, nil
		} else {
			return atc.Build{}, errors.New("job has no builds")
		}
	} else {
		page := &concourse.Page{Limit: 100}

		for page != nil {
			allBuilds, pagination, err := client.Builds(*page)
			if err != nil {
				return atc.Build{}, fmt.Errorf("failed to get builds %s", err)
			}

			for _, build := range allBuilds {
				if build.JobName == "" {
					return build, nil
				}
			}

			page = pagination.Next
		}

		return atc.Build{}, errors.New("no builds match job")
	}
}

func GetLatestResourceVersion(team concourse.Team, resource flaghelpers.ResourceFlag, version atc.Version) (atc.ResourceVersion, error) {
	versions, _, found, err := team.ResourceVersions(resource.PipelineRef, resource.ResourceName, concourse.Page{}, version)

	if err != nil {
		return atc.ResourceVersion{}, err
	}

	if !found || len(versions) <= 0 {
		versionBytes, err := json.Marshal(version)
		if err != nil {
			return atc.ResourceVersion{}, err
		}

		return atc.ResourceVersion{}, errors.New(fmt.Sprintf("could not find version matching %s", string(versionBytes)))
	}

	return versions[0], nil
}

func GetTeam(target rc.Target, teamToFind string) concourse.Team {
	if teamToFind != "" {
		return target.Client().Team(teamToFind)
	}
	return target.Team()
}
