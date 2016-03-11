package concourse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"
)

func (client *client) PipelineConfig(pipelineName string) (atc.Config, string, bool, error) {
	params := rata.Params{"pipeline_name": pipelineName}

	var configResponse atc.ConfigResponse

	responseHeaders := http.Header{}
	response := internal.Response{
		Headers: &responseHeaders,
		Result:  &configResponse,
	}
	err := client.connection.Send(internal.Request{
		RequestName: atc.GetConfig,
		Params:      params,
	}, &response)

	switch err.(type) {
	case nil:
		version := responseHeaders.Get(atc.ConfigVersionHeader)

		if len(configResponse.Errors) > 0 {
			return atc.Config{}, version, false, PipelineConfigError{configResponse.Errors}
		}

		return *configResponse.Config, version, true, nil
	case internal.ResourceNotFoundError:
		return atc.Config{}, "", false, nil
	default:
		return atc.Config{}, "", false, err
	}
}

type pipelineConfigResponse struct {
	Config *atc.Config
	Errors []string `json:"errors"`
}

type configValidationError struct {
	ErrorMessages []string `json:"errors"`
}

func (c configValidationError) Error() string {
	return fmt.Sprintf("invalid configuration:\n%s", strings.Join(c.ErrorMessages, "\n"))
}

type ConfigWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type setConfigResponse struct {
	Errors   []string        `json:"errors"`
	Warnings []ConfigWarning `json:"warnings"`
}

func (client *client) CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig atc.Config) (bool, bool, []ConfigWarning, error) {
	params := rata.Params{"pipeline_name": pipelineName}
	response := internal.Response{}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	yamlWriter, err := writer.CreatePart(
		textproto.MIMEHeader{
			"Content-type": {"application/x-yaml"},
		},
	)
	if err != nil {
		return false, false, []ConfigWarning{}, err
	}

	rawConfig, err := yaml.Marshal(passedConfig)
	if err != nil {
		return false, false, []ConfigWarning{}, err
	}

	_, err = yamlWriter.Write(rawConfig)
	if err != nil {
		return false, false, []ConfigWarning{}, err
	}

	writer.Close()

	err = client.connection.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.SaveConfig,
		Params:             params,
		Body:               body,
		Header: http.Header{
			"Content-Type":          {writer.FormDataContentType()},
			atc.ConfigVersionHeader: {configVersion},
		},
	},
		&response,
	)
	if err != nil {
		if unexpectedResponseError, ok := err.(internal.UnexpectedResponseError); ok {
			if unexpectedResponseError.StatusCode == http.StatusBadRequest {
				var validationErr configValidationError

				err = json.Unmarshal([]byte(unexpectedResponseError.Body), &validationErr)
				if err != nil {
					return false, false, []ConfigWarning{}, err
				}

				return false, false, []ConfigWarning{}, validationErr
			}
		}

		return false, false, []ConfigWarning{}, err
	}

	configResponse := setConfigResponse{}
	readCloser, ok := response.Result.(io.ReadCloser)
	if !ok {
		return false, false, []ConfigWarning{}, errors.New("Failed to assert type of response result")
	}
	defer readCloser.Close()

	contents, err := ioutil.ReadAll(readCloser)
	if err != nil {
		return false, false, []ConfigWarning{}, err
	}

	err = json.Unmarshal(contents, &configResponse)
	if err != nil {
		return false, false, []ConfigWarning{}, err
	}

	return response.Created, !response.Created, configResponse.Warnings, nil
}
