package db

import (
	"errors"

	"github.com/concourse/atc"
)

type StaticConfigDB struct {
	Config atc.Config
}

var ErrConfigIsStatic = errors.New("configuration is static")

func (db StaticConfigDB) GetConfig(string) (atc.Config, ConfigVersion, error) {
	return db.Config, 0, nil
}

func (db StaticConfigDB) SaveConfig(string, atc.Config, ConfigVersion) error {
	return ErrConfigIsStatic
}
