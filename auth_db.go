package auth

import "github.com/concourse/atc/db"

//go:generate counterfeiter . AuthDB

type AuthDB interface {
	GetTeam() (db.SavedTeam, bool, error)
}
