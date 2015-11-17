package concourse

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
)

func (client *client) PipelineConfig(pipelineName string) (atc.Config, string, bool, error) {
	params := map[string]string{"pipeline_name": pipelineName}

	var config atc.Config
	var version string
	responseHeaders := http.Header{}

	err := client.connection.Send(Request{
		RequestName: atc.GetConfig,
		Params:      params,
	}, &Response{
		Result:  &config,
		Headers: &responseHeaders,
	})

	version = responseHeaders.Get(atc.ConfigVersionHeader)

	switch err.(type) {
	case nil:
		return config, version, true, nil
	case ResourceNotFoundError:
		return config, version, false, nil
	default:
		return config, version, false, err
	}
}

func (client *client) CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig atc.Config) (bool, bool, error) {
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

	writer.Close()

	err = client.connection.Send(Request{
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
