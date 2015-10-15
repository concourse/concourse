package atcclient

import "github.com/concourse/atc"

func (buildHandler AtcHandler) JobBuild(pipelineName, jobName, buildName string) (atc.Build, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"job_name": jobName, "build_name": buildName, "pipeline_name": pipelineName}
	var build atc.Build
	err := buildHandler.client.MakeRequest(&build, atc.GetJobBuild, params, nil)
	return build, err
}

func (buildHandler AtcHandler) Build(buildID string) (atc.Build, error) {
	params := map[string]string{"build_id": buildID}
	var build atc.Build
	err := buildHandler.client.MakeRequest(&build, atc.GetBuild, params, nil)
	return build, err
}

func (buildHandler AtcHandler) AllBuilds() ([]atc.Build, error) {
	var builds []atc.Build
	err := buildHandler.client.MakeRequest(&builds, atc.ListBuilds, nil, nil)
	return builds, err
}
