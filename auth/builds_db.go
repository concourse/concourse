package auth

import "github.com/concourse/atc/db"

const BuildKey = "build"

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	GetBuildByID(buildID int) (db.Build, bool, error)
}
