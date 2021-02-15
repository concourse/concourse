package atccmd

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/flag"
)

type Migration struct {
	lockFactory lock.LockFactory

	Postgres               flag.PostgresConfig `group:"PostgreSQL Configuration" namespace:"postgres"`
	EncryptionKey          flag.Cipher         `long:"encryption-key"     description:"A 16 or 32 length key used to encrypt sensitive information before storing it in the database."`
	OldEncryptionKey       flag.Cipher         `long:"old-encryption-key" description:"Encryption key previously used for encrypting sensitive information. If provided without a new key, data is decrypted. If provided with a new key, data is re-encrypted."`
	CurrentDBVersion       bool                `long:"current-db-version" description:"Print the current database version and exit"`
	SupportedDBVersion     bool                `long:"supported-db-version" description:"Print the max supported database version and exit"`
	MigrateDBToVersion     int                 `long:"migrate-db-to-version" description:"Migrate to the specified database version and exit"`
	MigrateToLatestVersion bool                `long:"migrate-to-latest-version" description:"Migrate to the latest migration version and exit"`
}

func (m *Migration) Execute(args []string) error {
	lockConn, err := constructLockConn(defaultDriverName, m.Postgres.ConnectionString())
	if err != nil {
		return err
	}
	defer lockConn.Close()

	m.lockFactory = lock.NewLockFactory(lockConn, metric.LogLockAcquired, metric.LogLockReleased)

	if m.MigrateToLatestVersion {
		return m.migrateToLatestVersion()
	}
	if m.CurrentDBVersion {
		return m.currentDBVersion()
	}
	if m.SupportedDBVersion {
		return m.supportedDBVersion()
	}
	if m.MigrateDBToVersion > 0 {
		return m.migrateDBToVersion()
	}
	if m.OldEncryptionKey.AEAD != nil {
		return m.rotateEncryptionKey()
	}
	return errors.New("must specify one of `--migrate-to-latest-version`, `--current-db-version`, `--supported-db-version`, `--migrate-db-to-version`, or `--old-encryption-key`")
}

func (cmd *Migration) currentDBVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		cmd.lockFactory,
		nil,
		nil,
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
		cmd.lockFactory,
		nil,
		nil,
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

	var newKey *encryption.Key
	var oldKey *encryption.Key

	if cmd.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.EncryptionKey.AEAD)
	}
	if cmd.OldEncryptionKey.AEAD != nil {
		oldKey = encryption.NewKey(cmd.OldEncryptionKey.AEAD)
	}

	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		cmd.lockFactory,
		newKey,
		oldKey,
	)
	err := helper.MigrateToVersion(version)
	if err != nil {
		return fmt.Errorf("Could not migrate to version: %d Reason: %s", version, err.Error())
	}

	fmt.Println("Successfully migrated to version:", version)
	return nil
}

func (cmd *Migration) rotateEncryptionKey() error {
	var newKey *encryption.Key
	var oldKey *encryption.Key

	if cmd.EncryptionKey.AEAD != nil {
		newKey = encryption.NewKey(cmd.EncryptionKey.AEAD)
	}
	if cmd.OldEncryptionKey.AEAD != nil {
		oldKey = encryption.NewKey(cmd.OldEncryptionKey.AEAD)
	}

	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		cmd.lockFactory,
		newKey,
		oldKey,
	)

	version, err := helper.CurrentVersion()
	if err != nil {
		return err
	}

	return helper.MigrateToVersion(version)
}

func (cmd *Migration) migrateToLatestVersion() error {
	helper := migration.NewOpenHelper(
		defaultDriverName,
		cmd.Postgres.ConnectionString(),
		cmd.lockFactory,
		nil,
		nil,
	)

	version, err := helper.SupportedVersion()
	if err != nil {
		return err
	}

	return helper.MigrateToVersion(version)
}
