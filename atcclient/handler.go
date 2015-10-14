package atcclient

import "github.com/concourse/atc"

//go:generate counterfeiter . Handler
type Handler interface {
	// 	AbortBuild()
	// 	BuildEvents()
	// 	CreateBuild()
	// 	CreatePipe()
	// 	DeletePipeline()
	// 	DownloadCLI()
	// 	GetConfig()
	// 	HijackContainer()
	// 	ListContainer()
	// 	ListJobInputs()
	// 	ReadPipe()
	// 	SaveConfig()
	// 	WritePipe()
	AllBuilds() ([]atc.Build, error)
	Build(buildID string) (atc.Build, error)
	Job(pipelineName, jobName string) (atc.Job, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, error)
}

type AtcHandler struct {
	client Client
}

func NewAtcHandler(c Client) AtcHandler {
	return AtcHandler{client: c}
}

// TODO should the handler have functions that reflect API calls directly (ie separate GetBuild and GetJobBuild),
// or should it have comprehensive functions (ie integrated GetBuild and GetJobBuild)

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

func (buildHandler AtcHandler) Job(pipelineName, jobName string) (atc.Job, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"pipeline_name": pipelineName, "job_name": jobName}
	var job atc.Job
	err := buildHandler.client.MakeRequest(&job, atc.GetJob, params, nil)
	return job, err
}
