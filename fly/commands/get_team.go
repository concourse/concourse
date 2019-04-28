package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
	yaml "gopkg.in/yaml.v2"
)

type GetTeamCommand struct {
	Team string `short:"n" long:"team" required:"true" description:"Get configuration of this team"`
	JSON bool   `short:"j" long:"json" description:"Print config as json instead of yaml"`
}

func (command *GetTeamCommand) Execute(args []string) error {
	asJSON := command.JSON
	teamName := command.Team

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	if err := target.Validate(); err != nil {
		return err
	}

	config, found, err := target.Team().Config(teamName)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("team not found")
	}

	return dumpTeam(config, asJSON)
}

func dumpTeam(config atc.Team, asJSON bool) error {
	var payload []byte
	var err error
	if asJSON {
		payload, err = json.Marshal(config)
	} else {
		payload, err = yaml.Marshal(config)
	}
	if err != nil {
		return err
	}

	_, err = fmt.Printf("%s", payload)

	return err
}
