package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

func (team *team) SetWebhook(webhook atc.Webhook) (bool, error) {
	params := rata.Params{
		"team_name": team.Name(),
	}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(webhook)
	if err != nil {
		return false, fmt.Errorf("Unable to marshal webhook: %w", err)
	}
	resp, err := team.httpAgent.Send(internal.Request{
		RequestName: atc.SetTeamWebhook,
		Params:      params,
		Body:        buffer,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		return true, nil
	case http.StatusNoContent:
		return false, nil
	case http.StatusForbidden:
		return false, fmt.Errorf("you do not have a role on team '%s'", team.Name())
	case http.StatusBadRequest:
		body, _ := ioutil.ReadAll(resp.Body)
		return false, fmt.Errorf(string(body))
	case http.StatusNotFound:
		return false, internal.ResourceNotFoundError{}
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		return false, internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}

func (team *team) DestroyWebhook(name string) error {
	params := rata.Params{
		"team_name":    team.Name(),
		"webhook_name": name,
	}

	resp, err := team.httpAgent.Send(internal.Request{
		RequestName: atc.DestroyTeamWebhook,
		Params:      params,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusForbidden:
		return fmt.Errorf("you do not have a role on team '%s'", team.Name())
	case http.StatusNotFound:
		return internal.ResourceNotFoundError{}
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		return internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}
