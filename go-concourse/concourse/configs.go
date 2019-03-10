package concourse

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) PipelineConfig(pipelineName string) (atc.Config, string, bool, error) {
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
		return configResponse.Config,
			responseHeaders.Get(atc.ConfigVersionHeader),
			true,
			nil
	case internal.ResourceNotFoundError:
		return atc.Config{}, "", false, nil
	default:
		return atc.Config{}, "", false, err
	}
}

type ConfigWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type setConfigResponse struct {
	Errors   []string        `json:"errors"`
	Warnings []ConfigWarning `json:"warnings"`
}

func (team *team) CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig []byte, checkCredentials bool) (bool, bool, []ConfigWarning, error) {
	params := rata.Params{
		"pipeline_name": pipelineName,
		"team_name":     team.name,
	}

	queryParams := url.Values{}
	if checkCredentials {
		queryParams.Add(atc.SaveConfigCheckCreds, "")
	}

	response := internal.Response{}

	err := team.connection.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.SaveConfig,
		Params:             params,
		Query:              queryParams,
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
				var validationErr atc.SaveConfigResponse
				err = json.Unmarshal([]byte(unexpectedResponseError.Body), &validationErr)
				if err != nil {
					return false, false, []ConfigWarning{}, err
				}

				return false, false, []ConfigWarning{}, InvalidConfigError{
					Errors: validationErr.Errors,
				}
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
