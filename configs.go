package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) PipelineConfig(pipelineName string) (atc.Config, error) {
	// if pipelineName == "" {
	// 	pipelineName = atc.DefaultPipelineName
	// }
	params := map[string]string{"pipeline_name": pipelineName}
	var config atc.Config
	err := handler.client.MakeRequest(&config, atc.GetConfig, params, nil, nil)
	return config, err
}
