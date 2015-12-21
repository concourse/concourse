package auth

import "github.com/concourse/atc/db"

//go:generate counterfeiter . AuthDB

type AuthDB interface {
	GetTeamByName(teamName string) (db.SavedTeam, error)
}
