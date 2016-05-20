package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

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
