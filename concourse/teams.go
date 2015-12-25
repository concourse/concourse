package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

// SetTeam creates or updates team teamName with the settings provided in passedTeam.
// passedTeam should reflect the desired state of team's configuration.
func (client *client) SetTeam(teamName string, passedTeam atc.Team) (atc.Team, bool, bool, error) {
	params := rata.Params{"team_name": teamName}

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(passedTeam)
	if err != nil {
		return atc.Team{}, false, false, fmt.Errorf("Unable to marshal plan: %s", err)
	}

	var team atc.Team
	response := Response{
		Result: &team,
	}
	err = client.connection.Send(Request{
		RequestName: atc.SetTeam,
		Params:      params,
		Body:        buffer,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, &response)

	if err != nil {
		return team, false, false, err
	}

	var created, updated bool
	if response.Created {
		created = true
	} else {
		updated = true
	}

	return team, created, updated, nil
}
