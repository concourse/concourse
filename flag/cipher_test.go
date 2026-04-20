package flag_test

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/concourse/concourse/flag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cipher", func() {
	// 32-byte key for AES-256
	rawKey := "AES256Key-32Characters1234567890"

	Describe("Cipher (raw bytes)", func() {
		It("creates a valid AEAD from a 32-byte string", func() {
			var c flag.Cipher
			err := c.UnmarshalFlag(rawKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("creates a valid AEAD from a 16-byte string", func() {
			var c flag.Cipher
			err := c.UnmarshalFlag("AES128Key-16Char")
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("returns an error for an invalid key length", func() {
			var c flag.Cipher
			err := c.UnmarshalFlag("too-short")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to construct AES cipher"))
		})
	})

	Describe("CipherBase64", func() {
		It("creates a valid AEAD from a base64-encoded 32-byte key", func() {
			encoded := base64.StdEncoding.EncodeToString([]byte(rawKey))

			var c flag.CipherBase64
			err := c.UnmarshalFlag(encoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("creates a valid AEAD from a base64-encoded 16-byte key", func() {
			encoded := base64.StdEncoding.EncodeToString([]byte("AES128Key-16Char"))

			var c flag.CipherBase64
			err := c.UnmarshalFlag(encoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("supports keys with full byte range (0x00-0xFF)", func() {
			// Key with bytes that can't be typed as raw ASCII
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i)
			}
			encoded := base64.StdEncoding.EncodeToString(key)

			var c flag.CipherBase64
			err := c.UnmarshalFlag(encoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("returns an error for invalid base64", func() {
			var c flag.CipherBase64
			err := c.UnmarshalFlag("not-valid-base64!!!")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode base64 encryption key"))
		})

		It("returns an error for a base64 string that decodes to an invalid key length", func() {
			encoded := base64.StdEncoding.EncodeToString([]byte("short"))

			var c flag.CipherBase64
			err := c.UnmarshalFlag(encoded)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to construct AES cipher"))
		})
	})

	Describe("CipherHex", func() {
		It("creates a valid AEAD from a hex-encoded 32-byte key", func() {
			encoded := hex.EncodeToString([]byte(rawKey))

			var c flag.CipherHex
			err := c.UnmarshalFlag(encoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("creates a valid AEAD from a hex-encoded 16-byte key", func() {
			encoded := hex.EncodeToString([]byte("AES128Key-16Char"))

			var c flag.CipherHex
			err := c.UnmarshalFlag(encoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("supports keys with full byte range (0x00-0xFF)", func() {
			key := make([]byte, 32)
			for i := range key {
				key[i] = byte(i)
			}
			encoded := hex.EncodeToString(key)

			var c flag.CipherHex
			err := c.UnmarshalFlag(encoded)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.AEAD).ToNot(BeNil())
		})

		It("returns an error for invalid hex", func() {
			var c flag.CipherHex
			err := c.UnmarshalFlag("not-valid-hex-zzz")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode hex encryption key"))
		})

		It("returns an error for a hex string that decodes to an invalid key length", func() {
			encoded := hex.EncodeToString([]byte("short"))

			var c flag.CipherHex
			err := c.UnmarshalFlag(encoded)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to construct AES cipher"))
		})
	})

	Describe("Compatibility", func() {
		It("all three cipher types produce equivalent AEADs for the same key", func() {
			var raw flag.Cipher
			var b64 flag.CipherBase64
			var hx flag.CipherHex

			Expect(raw.UnmarshalFlag(rawKey)).To(Succeed())
			Expect(b64.UnmarshalFlag(base64.StdEncoding.EncodeToString([]byte(rawKey)))).To(Succeed())
			Expect(hx.UnmarshalFlag(hex.EncodeToString([]byte(rawKey)))).To(Succeed())

			// Encrypt with raw, decrypt with base64 and hex
			plaintext := []byte("hello concourse")
			nonce := make([]byte, raw.AEAD.NonceSize())

			ciphertext := raw.AEAD.Seal(nil, nonce, plaintext, nil)

			decrypted1, err := b64.AEAD.Open(nil, nonce, ciphertext, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(decrypted1).To(Equal(plaintext))

			decrypted2, err := hx.AEAD.Open(nil, nonce, ciphertext, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(decrypted2).To(Equal(plaintext))
		})
	})
})
