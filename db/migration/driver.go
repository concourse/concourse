package migration

import (
	"database/sql"
	"errors"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/concourse/atc/db/encryption"
	"github.com/concourse/atc/db/migration/migrations"
	"github.com/mattes/migrate/database"
)

//go:generate counterfeiter . Driver

type Driver interface {
	Open(url string) (database.Driver, error)
	Close() error
	Lock() error
	Unlock() error
	Run(migration io.Reader) error
	SetVersion(version int, dirty bool) error
	Version() (version int, dirty bool, err error)
	Drop() error
}

//go:generate counterfeiter . Migrations

type Migrations interface {
	Run(name string) error
}

func NewDriver(d Driver, db *sql.DB, es encryption.Strategy) Driver {
	return NewDriverForMigrations(d, migrations.NewMigrations(db, es))
}

func NewDriverForMigrations(d Driver, m Migrations) Driver {
	return &driver{d, m}
}

type driver struct {
	Driver
	Migrations
}

func (self *driver) Run(reader io.Reader) error {

	migr, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	contents := string(migr)

	if strings.HasPrefix(contents, "package") {

		re := regexp.MustCompile("(Up|Down)_[0-9]*")

		name := re.FindString(contents)
		if name == "" {
			return errors.New("No migration found. Must match (Up|Down)_[0-9]*")
		}

		return self.Migrations.Run(name)

	} else {
		return self.Driver.Run(strings.NewReader(contents))
	}
}
