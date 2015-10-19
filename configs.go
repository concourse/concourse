package atcclient

import "github.com/concourse/atc"

func (handler AtcHandler) PipelineConfig(pipelineName string) (atc.Config, string, error) {
	params := map[string]string{"pipeline_name": pipelineName}

	var config atc.Config
	var version string
	responseHeaders := map[string][]string{}

	err := handler.client.Send(Request{
		RequestName: atc.GetConfig,
		Params:      params,
	}, Response{
		Result:  &config,
		Headers: &responseHeaders,
	})

	if err != nil {
		return config, version, err
	}

	if header, ok := responseHeaders[atc.ConfigVersionHeader]; ok {
		version = header[0]
	}

	return config, version, err
}
