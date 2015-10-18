package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) PipelineConfig(pipelineName string) (atc.Config, error) {
	// if pipelineName == "" {
	// 	pipelineName = atc.DefaultPipelineName
	// }
	params := map[string]string{"pipeline_name": pipelineName}
	var config atc.Config
	err := handler.client.Send(Request{
		RequestName: atc.GetConfig,
		Params:      params,
		Result:      &config,
	})
	return config, err
}
