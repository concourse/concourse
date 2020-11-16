package concourse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

var ErrDestroyRefused = errors.New("not-permitted-to-destroy-as-requested")

type setTeamResponse struct {
	Errors   []string        `json:"errors,omitempty"`
	Warnings []ConfigWarning `json:"warnings,omitempty"`
	Team     atc.Team        `json:"team"`
}

// CreateOrUpdate creates or updates team teamName with the settings provided in passedTeam.
// passedTeam should reflect the desired state of team's configuration.
func (team *team) CreateOrUpdate(passedTeam atc.Team) (atc.Team, bool, bool, []ConfigWarning, error) {
	params := rata.Params{"team_name": team.Name()}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(passedTeam)
	if err != nil {
		return atc.Team{}, false, false, []ConfigWarning{}, fmt.Errorf("Unable to marshal plan: %s", err)
	}

	var result setTeamResponse
	response := internal.Response{
		Result: &result,
	}
	err = team.connection.Send(internal.Request{
		RequestName: atc.SetTeam,
		Params:      params,
		Body:        buffer,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &response)

	if err != nil {
		return result.Team, false, false, []ConfigWarning{}, err
	}

	var created, updated bool
	if response.Created {
		created = true
	} else {
		updated = true
	}

	return result.Team, created, updated, result.Warnings, nil
}

// DestroyTeam destroys the team with the name given as argument.
func (team *team) DestroyTeam(teamName string) error {
	params := rata.Params{"team_name": teamName}
	err := team.connection.Send(internal.Request{
		RequestName: atc.DestroyTeam,
		Params:      params,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, nil)

	if err == ErrForbidden {
		return ErrDestroyRefused
	}

	return err
}

func (team *team) RenameTeam(teamName, name string) (bool, []ConfigWarning, error) {
	params := rata.Params{
		"team_name": teamName,
	}

	jsonBytes, err := json.Marshal(atc.RenameRequest{NewName: name})
	if err != nil {
		return false, []ConfigWarning{}, err
	}

	var response setConfigResponse
	err = team.connection.Send(internal.Request{
		RequestName: atc.RenameTeam,
		Params:      params,
		Body:        bytes.NewBuffer(jsonBytes),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}, &internal.Response{
		Result: &response,
	})

	switch err.(type) {
	case nil:
		return true, response.Warnings, nil
	case internal.ResourceNotFoundError:
		return false, []ConfigWarning{}, nil
	default:
		return false, []ConfigWarning{}, err
	}
}

func (client *client) ListTeams() ([]atc.Team, error) {
	var teams []atc.Team
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListTeams,
	}, &internal.Response{
		Result: &teams,
	})

	return teams, err
}

func (client *client) FindTeam(teamName string) (Team, error) {
	var atcTeam atc.Team
	resp, err := client.httpAgent.Send(internal.Request{
		RequestName: atc.GetTeam,
		Params:      rata.Params{"team_name": teamName},
	})

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		err = json.NewDecoder(resp.Body).Decode(&atcTeam)
		if err != nil {
			return nil, err
		}
		return &team{
			atcTeam:    atcTeam,
			connection: client.connection,
			httpAgent:  client.httpAgent,
		}, nil
	case http.StatusForbidden:
		return nil, fmt.Errorf("you do not have a role on team '%s'", teamName)
	case http.StatusNotFound:
		return nil, fmt.Errorf("team '%s' does not exist", teamName)
	default:
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, internal.UnexpectedResponseError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}
}
