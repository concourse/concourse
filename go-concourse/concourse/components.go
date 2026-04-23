package concourse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (client *client) ListComponents() ([]atc.Component, error) {
	var components []atc.Component
	resp, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.GetComponents,
	})
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&components)
		if err != nil {
			return nil, err
		}
		return components, nil

	case http.StatusForbidden:
		return nil, errors.New("must be an owner of the 'main' team to interact with components")

	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}

func (client *client) PauseComponent(componentName string) error {
	return client.manageComponent(componentName, atc.PauseComponent)
}

func (client *client) UnpauseComponent(componentName string) error {
	return client.manageComponent(componentName, atc.UnpauseComponent)
}

func (client *client) manageComponent(componentName string, endpoint string) error {
	params := rata.Params{
		"component_name": componentName,
	}
	resp, err := client.httpAgent.Send(internal.Request{
		RequestName: endpoint,
		Params:      params,
	})
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return nil

	case http.StatusNotFound:
		return fmt.Errorf("component '%s' not found", componentName)

	case http.StatusForbidden:
		return errors.New("must be an owner of the 'main' team to interact with components")

	default:
		body, _ := io.ReadAll(resp.Body)
		return internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}

func (client *client) PauseAllComponents() error {
	_, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.PauseAllComponents,
	})
	return err
}

func (client *client) UnpauseAllComponents() error {
	_, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.UnpauseAllComponents,
	})
	return err
}
