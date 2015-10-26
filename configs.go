package atcclient

import (
	"bytes"
	"mime/multipart"
	"net/textproto"

	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
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

func (handler AtcHandler) CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig atc.Config, paused *bool) (bool, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}
	response := Response{}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	yamlWriter, err := writer.CreatePart(
		textproto.MIMEHeader{
			"Content-type": {"application/x-yaml"},
		},
	)
	if err != nil {
		return false, false, err
	}

	rawConfig, err := yaml.Marshal(passedConfig)
	if err != nil {
		return false, false, err
	}

	_, err = yamlWriter.Write(rawConfig)

	if paused != nil {
		if *paused {
			err = writer.WriteField("paused", "true")
		} else {
			err = writer.WriteField("paused", "false")
		}
		if err != nil {
			return false, false, err
		}
	}

	writer.Close()

	err = handler.client.Send(Request{
		RequestName: atc.SaveConfig,
		Params:      params,
		Body:        body,
		Headers: map[string][]string{
			"Content-Type":          {writer.FormDataContentType()},
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
