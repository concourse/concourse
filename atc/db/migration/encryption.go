package migration

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db/encryption"
)

var encryptedColumns = []encryptedColumn{
	{"teams", "legacy_auth", "id"},
	{"resources", "config", "id"},
	{"jobs", "config", "id"},
	{"resource_types", "config", "id"},
	{"builds", "private_plan", "id"},
	{"cert_cache", "cert", "domain"},
	{"checks", "plan", "id"},
	{"pipelines", "var_sources", "id"},
}

type encryptedColumn struct {
	Table      string
	Column     string
	PrimaryKey string
}

func (self migrator) encryptPlaintext(key *encryption.Key) error {
	logger := self.logger.Session("encrypt")
	for _, ec := range encryptedColumns {
		rows, err := self.db.Query(`
			SELECT ` + ec.PrimaryKey + `, ` + ec.Column + `
			FROM ` + ec.Table + `
			WHERE nonce IS NULL
			AND ` + ec.Column + ` IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": ec.Table,
		})

		encryptedRows := 0

		for rows.Next() {
			var (
				primaryKey interface{}
				val        sql.NullString
			)

			err := rows.Scan(&primaryKey, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			if !val.Valid {
				continue
			}

			rLog := tLog.Session("row", lager.Data{
				"primary-key": primaryKey,
			})

			encrypted, nonce, err := key.Encrypt([]byte(val.String))
			if err != nil {
				rLog.Error("failed-to-encrypt", err)
				return err
			}

			_, err = self.db.Exec(`
				UPDATE `+ec.Table+`
				SET `+ec.Column+` = $1, nonce = $2
				WHERE `+ec.PrimaryKey+` = $3
			`, encrypted, nonce, primaryKey)
			if err != nil {
				rLog.Error("failed-to-update", err)
				return err
			}

			encryptedRows++
		}

		if encryptedRows > 0 {
			tLog.Info("encrypted-existing-plaintext-data", lager.Data{
				"rows": encryptedRows,
			})
		}
	}

	return nil
}

func (self migrator) decryptToPlaintext(oldKey *encryption.Key) error {
	logger := self.logger.Session("decrypt")
	for _, ec := range encryptedColumns {
		rows, err := self.db.Query(`
			SELECT ` + ec.PrimaryKey + `, nonce, ` + ec.Column + `
			FROM ` + ec.Table + `
			WHERE nonce IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": ec.Table,
		})

		decryptedRows := 0

		for rows.Next() {
			var (
				primaryKey interface{}
				val, nonce string
			)

			err := rows.Scan(&primaryKey, &nonce, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			rLog := tLog.Session("row", lager.Data{
				"primary-key": primaryKey,
			})

			decrypted, err := oldKey.Decrypt(val, &nonce)
			if err != nil {
				rLog.Error("failed-to-decrypt", err)
				return err
			}

			_, err = self.db.Exec(`
				UPDATE `+ec.Table+`
				SET `+ec.Column+` = $1, nonce = NULL
				WHERE `+ec.PrimaryKey+` = $2
			`, decrypted, primaryKey)
			if err != nil {
				rLog.Error("failed-to-update", err)
				return err
			}

			decryptedRows++
		}

		if decryptedRows > 0 {
			tLog.Info("decrypted-existing-encrypted-data", lager.Data{
				"rows": decryptedRows,
			})
		}
	}

	return nil
}

var ErrEncryptedWithUnknownKey = errors.New("row encrypted with neither old nor new key")

func (self migrator) encryptWithNewKey(newKey *encryption.Key, oldKey *encryption.Key) error {
	logger := self.logger.Session("rotate")
	for _, ec := range encryptedColumns {
		rows, err := self.db.Query(`
			SELECT ` + ec.PrimaryKey + `, nonce, ` + ec.Column + `
			FROM ` + ec.Table + `
			WHERE nonce IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": ec.Table,
		})

		encryptedRows := 0

		for rows.Next() {
			var (
				primaryKey interface{}
				val, nonce string
			)

			err := rows.Scan(&primaryKey, &nonce, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			rLog := tLog.Session("row", lager.Data{
				"primary-key": primaryKey,
			})

			decrypted, err := oldKey.Decrypt(val, &nonce)
			if err != nil {
				_, err = newKey.Decrypt(val, &nonce)
				if err == nil {
					rLog.Debug("already-encrypted-with-new-key")
					continue
				}

				logger.Error("failed-to-decrypt-with-either-key", err)
				return ErrEncryptedWithUnknownKey
			}

			encrypted, newNonce, err := newKey.Encrypt(decrypted)
			if err != nil {
				rLog.Error("failed-to-encrypt", err)
				return err
			}

			_, err = self.db.Exec(`
				UPDATE `+ec.Table+`
				SET `+ec.Column+` = $1, nonce = $2
				WHERE `+ec.PrimaryKey+` = $3
			`, encrypted, newNonce, primaryKey)
			if err != nil {
				rLog.Error("failed-to-update", err)
				return err
			}

			encryptedRows++
		}

		if encryptedRows > 0 {
			tLog.Info("re-encrypted-existing-encrypted-data", lager.Data{
				"rows": encryptedRows,
			})
		}
	}

	return nil
}

