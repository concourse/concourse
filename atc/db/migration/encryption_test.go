package migration_test

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	"github.com/concourse/concourse/atc/db/migration/migrationfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encryption", func() {
	var (
		err         error
		db          *sql.DB
		lockDB      *sql.DB
		lockFactory lock.LockFactory
		bindata     *migrationfakes.FakeBindata
		fakeLogFunc = func(logger lager.Logger, id lock.LockID) {}
	)

	BeforeEach(func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockDB, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockFactory = lock.NewLockFactory(lockDB, fakeLogFunc, fakeLogFunc)

		bindata = new(migrationfakes.FakeBindata)
		bindata.AssetStub = asset
	})

	AfterEach(func() {
		_ = db.Close()
		_ = lockDB.Close()
	})

	Context("starting with unencrypted DB", func() {
		var (
			key *encryption.Key
		)
		BeforeEach(func() {
			key = createKey("AES256Key-32Characters1234567890")
		})

		It("encrypts the database", func() {
			migrator := migration.NewMigrator(db, lockFactory)

			err := migrator.Up(nil, nil)
			Expect(err).ToNot(HaveOccurred())

			insertIntoEncryptedColumn(db, encryption.NewNoEncryption(), "test")

			err = migrator.Up(key, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(isEncryptedWith(db, key, "test")).To(BeTrue())
		})
	})

	Context("starting with encrypted DB", func() {
		var (
			key1 *encryption.Key
			key2 *encryption.Key
		)

		BeforeEach(func() {
			key1 = createKey("AES256Key-32Characters1234567890")
			key2 = createKey("AES256Key-32Characters0987654321")
		})

		Context("removing the encryption key", func() {
			It("decrypts the database", func() {
				migrator := migration.NewMigrator(db, lockFactory)

				err := migrator.Up(key1, nil)
				Expect(err).ToNot(HaveOccurred())

				insertIntoEncryptedColumn(db, key1, "test")

				err = migrator.Up(nil, key1)
				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, encryption.NewNoEncryption(), "test")).To(BeTrue())
			})
		})

		Context("rotating the encryption key", func() {
			It("re-encrypts the database with the new key", func() {
				migrator := migration.NewMigrator(db, lockFactory)

				err := migrator.Up(key2, nil)
				Expect(err).ToNot(HaveOccurred())

				insertIntoEncryptedColumn(db, key2, "test")

				err = migrator.Up(key1, key2)
				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, key1, "test")).To(BeTrue())
			})

			It("rotates the key while doing a migration", func() {
				migrator := migration.NewMigrator(db, lockFactory)

				// do all the necessary schema migrations to this particular version
				err = migrator.Migrate(nil, nil, 1513895878)
				Expect(err).ToNot(HaveOccurred())

				insertIntoEncryptedColumnLegacy(db, key1, "test")

				// the migration after this one, 1516643303 needs to re-encrypt the auth column
				err := migrator.Up(key2, key1)
				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, key2, "test")).To(BeTrue())
			})
		})
	})

	Context("starting with partially encrypted DB", func() {
		var (
			key1     *encryption.Key
			key2     *encryption.Key
			migrator migration.Migrator
		)

		BeforeEach(func() {
			key1 = createKey("AES256Key-32Characters1234567890")
			key2 = createKey("AES256Key-32Characters0987654321")
			migrator = migration.NewMigrator(db, lockFactory)

			err := migrator.Up(nil, nil)
			Expect(err).ToNot(HaveOccurred())

			insertIntoEncryptedColumn(db, encryption.NewNoEncryption(), "row1")
			insertIntoEncryptedColumn(db, key1, "row2")
		})

		Context("adding the encryption key", func() {
			It("encrypts the database", func() {
				err = migrator.Up(key1, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, key1, "row1")).To(BeTrue())
				Expect(isEncryptedWith(db, key1, "row2")).To(BeTrue())
			})
		})

		Context("removing the encryption key", func() {
			It("decrypts the database", func() {
				err = migrator.Up(nil, key1)
				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, encryption.NewNoEncryption(), "row1")).To(BeTrue())
				Expect(isEncryptedWith(db, encryption.NewNoEncryption(), "row2")).To(BeTrue())
			})
		})

		Context("rotating the encryption key", func() {
			It("re-encrypts the database with the new key", func() {
				err = migrator.Up(key2, key1)

				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, key2, "row1")).To(BeTrue())
				Expect(isEncryptedWith(db, key2, "row2")).To(BeTrue())
			})
		})

		Context("rotating to the same key", func() {
			It("doesn't break", func() {
				err = migrator.Up(key1, key1)

				Expect(err).NotTo(HaveOccurred())
				Expect(isEncryptedWith(db, key1, "row1")).To(BeTrue())
				Expect(isEncryptedWith(db, key1, "row2")).To(BeTrue())
			})
		})
	})
})

// used to test database versions before the column got renamed
func insertIntoEncryptedColumnLegacy(db *sql.DB, strategy encryption.Strategy, name string) {
	ciphertext, nonce, err := strategy.Encrypt([]byte("{}"))
	Expect(err).ToNot(HaveOccurred())
	_, err = db.Exec(`INSERT INTO teams(name, auth, nonce) VALUES($1, $2, $3)`, name, ciphertext, nonce)
	Expect(err).ToNot(HaveOccurred())
}

func insertIntoEncryptedColumn(db *sql.DB, strategy encryption.Strategy, name string) {
	ciphertext, nonce, err := strategy.Encrypt([]byte("{}"))
	Expect(err).ToNot(HaveOccurred())
	_, err = db.Exec(`INSERT INTO teams(name, legacy_auth, nonce) VALUES($1, $2, $3)`, name, ciphertext, nonce)
	Expect(err).ToNot(HaveOccurred())
}

func isEncryptedWith(db *sql.DB, strategy encryption.Strategy, name string) bool {
	var (
		ciphertext string
		nonce      *string
	)
	row := db.QueryRow(`SELECT legacy_auth, nonce FROM teams WHERE name = $1`, name)
	err := row.Scan(&ciphertext, &nonce)
	Expect(err).ToNot(HaveOccurred())

	_, err = strategy.Decrypt(ciphertext, nonce)
	return err == nil
}

// createKey generates an encryption.Key from a 32 characters key
func createKey(key string) *encryption.Key {
	k := []byte(key)

	block, err := aes.NewCipher(k)
	Expect(err).ToNot(HaveOccurred())

	aesgcm, err := cipher.NewGCM(block)
	Expect(err).ToNot(HaveOccurred())

	return encryption.NewKey(aesgcm)
}
