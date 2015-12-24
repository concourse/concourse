package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Team(savedTeam db.SavedTeam) atc.Team {
	return atc.Team{
		ID:   savedTeam.ID,
		Name: savedTeam.Name,
	}
}
