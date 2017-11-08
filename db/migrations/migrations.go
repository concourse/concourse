package migrations

import "github.com/concourse/atc/db/migration"

type EncryptionStrategy interface {
	Encrypt([]byte) (string, *string, error)
	Decrypt(string, *string) ([]byte, error)
}

func New(strategy EncryptionStrategy) []migration.Migrator {
	return []migration.Migrator{
		InitialSchema,
	}
}
