package encryption_test

import (
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/encryption/encryptionfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encryption Key with Fallback", func() {
	var (
		key       *encryption.FallbackStrategy
		strategy1 *encryptionfakes.FakeStrategy
		strategy2 *encryptionfakes.FakeStrategy
	)

	BeforeEach(func() {
		strategy1 = &encryptionfakes.FakeStrategy{}
		strategy2 = &encryptionfakes.FakeStrategy{}

		key = encryption.NewFallbackStrategy(strategy1, strategy2)
	})

	Context("when the main key is valid", func() {
		It("decrypts plaintext using the main key", func() {
			strategy1.DecryptReturns([]byte("plaintext"), nil)

			decryptedText, err := key.Decrypt("ciphertext", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(decryptedText).To(Equal([]byte("plaintext")))
		})
	})

	Context("when the main key is invalid", func() {
		It("decrypts plaintext using the fallback key", func() {
			strategy1.DecryptReturns(nil, encryption.ErrDataIsEncrypted)
			strategy2.DecryptReturns([]byte("plaintext"), nil)

			decryptedText, err := key.Decrypt("ciphertext", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(decryptedText).To(Equal([]byte("plaintext")))
		})
	})

	Context("when both keys to decrypt are invalid", func() {
		It("return with an error", func() {
			strategy1.DecryptReturns(nil, encryption.ErrDataIsEncrypted)
			strategy2.DecryptReturns(nil, encryption.ErrDataIsEncrypted)

			_, err := key.Decrypt("ciphertext", nil)
			Expect(err).To(HaveOccurred())
		})
	})
})
