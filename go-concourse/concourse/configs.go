package concourse

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) PipelineConfig(pipelineRef atc.PipelineRef) (atc.Config, string, bool, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
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
		Query:       pipelineRef.QueryParams(),
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

func (team *team) CreateOrUpdatePipelineConfig(pipelineRef atc.PipelineRef, configVersion string, passedConfig []byte, checkCredentials bool) (bool, bool, []ConfigWarning, error) {
	params := rata.Params{
		"pipeline_name": pipelineRef.Name,
		"team_name":     team.Name(),
	}

	queryParams := url.Values{}
	if checkCredentials {
		queryParams.Add(atc.SaveConfigCheckCreds, "")
	}

	response, err := team.httpAgent.Send(internal.Request{
		ReturnResponseBody: true,
		RequestName:        atc.SaveConfig,
		Params:             params,
		Query:              merge(queryParams, pipelineRef.QueryParams()),
		Body:               bytes.NewBuffer(passedConfig),
		Header: http.Header{
			"Content-Type":          {"application/x-yaml"},
			atc.ConfigVersionHeader: {configVersion},
		},
	})
	if err != nil {
		return false, false, []ConfigWarning{}, err
	}

	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)

	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
		configResponse := setConfigResponse{}
		err = json.Unmarshal(body, &configResponse)
		if err != nil {
			return false, false, []ConfigWarning{}, err
		}
		created := response.StatusCode == http.StatusCreated
		return created, !created, configResponse.Warnings, nil
	case http.StatusBadRequest:
		var validationErr atc.SaveConfigResponse
		err = json.Unmarshal(body, &validationErr)
		if err != nil {
			return false, false, []ConfigWarning{}, err
		}
		return false, false, []ConfigWarning{}, InvalidConfigError{Errors: validationErr.Errors}
	case http.StatusForbidden:
		return false, false, []ConfigWarning{}, internal.ForbiddenError{
			Reason: string(body),
		}
	default:
		return false, false, []ConfigWarning{}, internal.UnexpectedResponseError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       string(body),
		}
	}
}

func merge(base, extra url.Values) url.Values {
	if extra != nil {
		for key, values := range extra {
			for _, value := range values {
				base.Add(key, value)
			}
		}
	}
	return base
}
