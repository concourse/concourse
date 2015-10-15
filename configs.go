package atcclient

import "github.com/concourse/atc"

func (configHandler AtcHandler) PipelineConfig(pipelineName string) (atc.Config, error) {
	// if pipelineName == "" {
	// 	pipelineName = atc.DefaultPipelineName
	// }
	params := map[string]string{"pipeline_name": pipelineName}
	var config atc.Config
	err := configHandler.client.MakeRequest(&config, atc.GetConfig, params, nil)
	return config, err
}
