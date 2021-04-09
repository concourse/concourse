package migrations

import (
	"database/sql"
	"reflect"

	"github.com/concourse/concourse/atc/db/encryption"
)

func NewMigrations(tx *sql.Tx, es encryption.Strategy) *migrations {
	return &migrations{tx, es}
}

type migrations struct {
	*sql.Tx
	encryption.Strategy
}

func (m *migrations) Run(name string) error {

	res := reflect.ValueOf(m).MethodByName(name).Call(nil)

	ret := res[0].Interface()

	if ret != nil {
		return ret.(error)
	}

	return nil
}
