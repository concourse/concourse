package db_test

import (
	"time"

	"github.com/concourse/concourse/atc/creds/idtoken"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SigningKey factory", func() {
	var (
		factory db.SigningKeyFactory
	)

	BeforeEach(func() {
		factory = db.NewSigningKeyFactory(dbConn)
	})

	It("can store and retrieve signing keys", func() {
		rsaKey1, err := idtoken.GenerateNewKey(db.SigningKeyTypeRSA)
		Expect(err).ToNot(HaveOccurred())

		rsaKey2, err := idtoken.GenerateNewKey(db.SigningKeyTypeRSA)
		Expect(err).ToNot(HaveOccurred())

		previousKeys, err := factory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(previousKeys).To(HaveLen(0))

		Expect(factory.CreateKey(*rsaKey1)).To(Succeed())
		// make sure there is a measurable difference in created_at of the two keys
		time.Sleep(1 * time.Second)
		Expect(factory.CreateKey(*rsaKey2)).To(Succeed())

		storedKeys, err := factory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(storedKeys).To(HaveLen(2))

		Expect(storedKeys[0].JWK().KeyID).To(Equal(rsaKey1.KeyID))
		Expect(storedKeys[1].JWK().KeyID).To(Equal(rsaKey2.KeyID))

		latest, err := factory.GetNewestKey(db.SigningKeyTypeRSA)
		Expect(err).ToNot(HaveOccurred())

		Expect(latest.JWK().KeyID).To(Equal(rsaKey2.KeyID))

		Expect(storedKeys[0].Delete()).To(Succeed())
		storedKeys, err = factory.GetAllKeys()
		Expect(err).ToNot(HaveOccurred())
		Expect(storedKeys).To(HaveLen(1))
		Expect(storedKeys[0].JWK().KeyID).To(Equal(rsaKey2.KeyID))

		_, err = factory.GetNewestKey(db.SigningKeyTypeEC)
		Expect(err).To(HaveOccurred())
	})
})
