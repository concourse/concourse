package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func Team(team dbng.Team) atc.Team {
	return atc.Team{
		ID:   team.ID(),
		Name: team.Name(),
	}
}
func SavedTeam(team db.SavedTeam) atc.Team {
	return atc.Team{
		ID:   team.ID,
		Name: team.Name,
	}
}
