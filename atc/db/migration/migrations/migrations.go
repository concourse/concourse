package migrations

import (
	"database/sql"
	"reflect"

	"github.com/concourse/atc/db/encryption"
)

func NewMigrations(db *sql.DB, es encryption.Strategy) *migrations {
	return &migrations{db, es}
}

type migrations struct {
	*sql.DB
	encryption.Strategy
}

func (self *migrations) Run(name string) error {

	res := reflect.ValueOf(self).MethodByName(name).Call(nil)

	ret := res[0].Interface()

	if ret != nil {
		return ret.(error)
	}

	return nil
}
