package present

import (
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
)

func Team(team db.Team) atc.Team {
	return atc.Team{
		ID:   team.ID(),
		Name: team.Name(),
		Auth: team.Auth(),
	}
}
