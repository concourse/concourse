package concourse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

var ErrDestroyRefused = errors.New("not-permitted-to-destroy-as-requested")

// CreateOrUpdate creates or updates team teamName with the settings provided in passedTeam.
// passedTeam should reflect the desired state of team's configuration.
func (team *team) CreateOrUpdate(passedTeam atc.Team) (atc.Team, bool, bool, error) {
	params := rata.Params{"team_name": team.name}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(passedTeam)
	if err != nil {
		return atc.Team{}, false, false, fmt.Errorf("Unable to marshal plan: %s", err)
	}

	var savedTeam atc.Team
	response := internal.Response{
		Result: &savedTeam,
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
		return savedTeam, false, false, err
	}

	var created, updated bool
	if response.Created {
		created = true
	} else {
		updated = true
	}

	return savedTeam, created, updated, nil
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

	if err == internal.ErrForbidden {
		return ErrDestroyRefused
	}

	return err
}

func (team *team) RenameTeam(teamName, name string) (bool, error) {
	params := rata.Params{
		"team_name": teamName,
	}

	jsonBytes, err := json.Marshal(atc.RenameRequest{NewName: name})
	if err != nil {
		return false, err
	}

	err = team.connection.Send(internal.Request{
		RequestName: atc.RenameTeam,
		Params:      params,
		Body:        bytes.NewBuffer(jsonBytes),
		Header:      http.Header{"Content-Type": []string{"application/json"}},
	}, nil)
	switch err.(type) {
	case nil:
		return true, nil
	case internal.ResourceNotFoundError:
		return false, nil
	default:
		return false, err
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
