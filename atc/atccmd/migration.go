package atccmd

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/flag"
)

type Migration struct {
	Postgres           flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`
	EncryptionKey      flag.Cipher         `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	CurrentDBVersion   bool                `long:"current-db-version" description:"Print the current database version and exit"`
	SupportedDBVersion bool                `long:"supported-db-version" description:"Print the max supported database version and exit"`
	MigrateDBToVersion int                 `long:"migrate-db-to-version" description:"Migrate to the specified database version and exit"`
}

func (m *Migration) Execute(args []string) error {
	if m.CurrentDBVersion {
		return m.currentDBVersion()
	}
	if m.SupportedDBVersion {
		return m.supportedDBVersion()
	}
	if m.MigrateDBToVersion > 0 {
		return m.migrateDBToVersion()
	}
	return errors.New("must specify one of `--current-db-version`, `--supported-db-version`, or `--migrate-db-to-version`")

}

func (cmd *Migration) currentDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		encryption.NewNoEncryption(),
	)

	version, err := helper.CurrentVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *Migration) supportedDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		encryption.NewNoEncryption(),
	)

	version, err := helper.SupportedVersion()
	if err != nil {
		return err
	}

	fmt.Println(version)
	return nil
}

func (cmd *Migration) migrateDBToVersion() error {
	version := cmd.MigrateDBToVersion

	// XXX: Can this use runcommand.NewKey ????
	var newKey *encryption.Key
	if cmd.EncryptionKey != nil {
		AEAD, err := parseAEAD(cmd.EncryptionKey)
		if err != nil {
			return err
		}

		newKey = encryption.NewKey(AEAD)
	}

	var strategy encryption.Strategy
	if newKey != nil {
		strategy = newKey
	} else {
		strategy = encryption.NewNoEncryption()
	}

	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		nil,
		strategy,
	)

	err := helper.MigrateToVersion(version)
	if err != nil {
		return fmt.Errorf("Could not migrate to version: %d Reason: %s", version, err.Error())
	}

	fmt.Println("Successfully migrated to version:", version)
	return nil
}
