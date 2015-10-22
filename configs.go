package atcclient

import (
	"bytes"

	"github.com/concourse/atc"
)

func (handler AtcHandler) PipelineConfig(pipelineName string) (atc.Config, string, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}

	var config atc.Config
	var version string
	responseHeaders := map[string][]string{}

	err := handler.client.Send(Request{
		RequestName: atc.GetConfig,
		Params:      params,
	}, &Response{
		Result:  &config,
		Headers: &responseHeaders,
	})

	if header, ok := responseHeaders[atc.ConfigVersionHeader]; ok {
		version = header[0]
	}

	switch err.(type) {
	case nil:
		return config, version, true, nil
	case ResourceNotFoundError:
		return config, version, false, nil
	default:
		return config, version, false, err
	}
}

func (handler AtcHandler) CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, buffer *bytes.Buffer, contentType string) (bool, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	response := Response{}
	err := handler.client.Send(Request{
		RequestName: atc.SaveConfig,
		Params:      params,
		Body:        buffer,
		Headers: map[string][]string{
			"Content-Type":          {contentType},
			atc.ConfigVersionHeader: {configVersion},
		},
	},
		&response,
	)
	if err != nil {
		return false, false, err
	}

	if response.Created {
		return true, false, nil
	}
	return false, true, nil
}
