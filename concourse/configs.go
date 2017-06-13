package concourse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) PipelineConfig(pipelineName string) (atc.Config, atc.RawConfig, string, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	var configResponse atc.ConfigResponse

	responseHeaders := http.Header{}
	response := internal.Response{
		Headers: &responseHeaders,
		Result:  &configResponse,
	}
	err := team.connection.Send(internal.Request{
		RequestName: atc.GetConfig,
		Params:      params,
	}, &response)

	switch err.(type) {
	case nil:
		version := responseHeaders.Get(atc.ConfigVersionHeader)

		if len(configResponse.Errors) > 0 {
			return atc.Config{}, configResponse.RawConfig, version, false, PipelineConfigError{configResponse.Errors}
		}

		return *configResponse.Config, configResponse.RawConfig, version, true, nil
	case internal.ResourceNotFoundError:
		return atc.Config{}, atc.RawConfig(""), "", false, nil
	default:
		return atc.Config{}, atc.RawConfig(""), "", false, err
	}
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

func (team *team) CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig []byte) (bool, bool, []ConfigWarning, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	response := internal.Response{}

	err := team.connection.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.SaveConfig,
		Params:             params,
		Body:               bytes.NewBuffer(passedConfig),
		Header: http.Header{
			"Content-Type":          {"application/x-yaml"},
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
