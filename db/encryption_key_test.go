package db_test

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encryption Key", func() {
	var (
		key       *db.EncryptionKey
		plaintext []byte
	)

	BeforeEach(func() {
		k := []byte("AES256Key-32Characters1234567890")

		block, err := aes.NewCipher(k)
		Expect(err).ToNot(HaveOccurred())

		aesgcm, err := cipher.NewGCM(block)
		Expect(err).ToNot(HaveOccurred())

		key = db.NewEncryptionKey(aesgcm)
	})

	Context("when the key is valid", func() {
		It("encrypts and decrypts plaintext", func() {
			plaintext = []byte("exampleplaintext")

			By("encrypting the plaintext")
			encryptedText, nonce, err := key.Encrypt(plaintext)
			Expect(err).ToNot(HaveOccurred())
			Expect(encryptedText).ToNot(BeEmpty())
			Expect(encryptedText).ToNot(Equal(plaintext))

			By("decrypting the encrypted text")
			decryptedText, err := key.Decrypt(encryptedText, nonce)
			Expect(err).ToNot(HaveOccurred())
			Expect(decryptedText).To(Equal(plaintext))
		})

		Context("when encrypting empty text", func() {
			It("does not error", func() {
				By("encrypting the plaintext")
				encryptedText, nonce, err := key.Encrypt(nil)
				Expect(err).ToNot(HaveOccurred())

				By("decrypting the encrypted text")
				decryptedText, err := key.Decrypt(encryptedText, nonce)
				Expect(err).ToNot(HaveOccurred())
				Expect(decryptedText).To(BeNil())
			})
		})

		Context("when the key to decrypt is invalid", func() {
			It("throws an error", func() {
				plaintext = []byte("exampleplaintext")

				By("encrypting the plaintext")
				encryptedText, nonce, err := key.Encrypt(plaintext)
				Expect(err).ToNot(HaveOccurred())
				Expect(encryptedText).ToNot(BeEmpty())
				Expect(encryptedText).ToNot(Equal(plaintext))

				By("decrypting the encrypted text with the wrong key")
				k := []byte("AES256Key-32Characters9564567123")

				block, err := aes.NewCipher(k)
				Expect(err).ToNot(HaveOccurred())

				aesgcm, err := cipher.NewGCM(block)
				Expect(err).ToNot(HaveOccurred())

				wrongKey := db.NewEncryptionKey(aesgcm)

				_, err = wrongKey.Decrypt(encryptedText, nonce)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
