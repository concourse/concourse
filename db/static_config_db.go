package db

import (
	"errors"

	"github.com/concourse/atc"
)

type StaticConfigDB struct {
	Config atc.Config
}

var ErrConfigIsStatic = errors.New("configuration is static")

func (db StaticConfigDB) GetConfig() (atc.Config, error) {
	return db.Config, nil
}

func (db StaticConfigDB) SaveConfig(atc.Config) error {
	return ErrConfigIsStatic
}
