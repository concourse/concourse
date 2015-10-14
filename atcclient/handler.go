package atcclient

import "github.com/concourse/atc"

type BuildHandler struct {
	client Client
}

func NewBuildHandler(c Client) BuildHandler {
	return BuildHandler{client: c}
}

// TODO should the handler have functions that reflect API calls directly (ie separate GetBuild and GetJobBuild),
// or should it have comprehensive functions (ie integrated GetBuild and GetJobBuild)

func (buildHandler *BuildHandler) GetJobBuild(jobName string, buildName string, pipelineName string) (atc.Build, error) {
	if pipelineName == "" {
		pipelineName = atc.DefaultPipelineName
	}
	params := map[string]string{"job_name": jobName, "build_name": buildName, "pipeline_name": pipelineName}
	var build atc.Build
	err := buildHandler.client.MakeRequest(&build, "GetJobBuild", params, nil)
	return build, err
}

// type Handler interface {
// 	AbortBuild()
// 	BuildEvents()
// 	CreateBuild()
// 	CreatePipe()
// 	DeletePipeline()
// 	DownloadCLI()
// 	GetBuild()
// 	GetConfig()
// 	GetJob()
// 	GetJobBuild()
// 	HijackContainer()
// 	ListBuilds()
// 	ListContainer()
// 	ListJobInputs()
// 	ReadPipe()
// 	SaveConfig()
// 	WritePipe()
// }
