package accessor_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/concourse/atc/api/accessor"
	jwt "github.com/dgrijalva/jwt-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessorFactory", func() {
	var accessorFactory accessor.AccessFactory
	var access accessor.Access
	var key *rsa.PrivateKey
	var req *http.Request

	Describe("Create", func() {
		BeforeEach(func() {
			reader := rand.Reader
			bitSize := 2048
			var err error
			key, err = rsa.GenerateKey(reader, bitSize)
			Expect(err).NotTo(HaveOccurred())

			publicKey := &key.PublicKey
			//publicKey = rsa.GenerateKey(random, bits)
			accessorFactory = accessor.NewAccessFactory(publicKey)

			req, err = http.NewRequest("GET", "localhost:8080", nil)
			Expect(err).NotTo(HaveOccurred())
		})
		JustBeforeEach(func() {
			access = accessorFactory.Create(req)
		})

		Context("when request has jwt token set", func() {
			BeforeEach(func() {
				token := jwt.New(jwt.SigningMethodRS256)
				tokenString, err := token.SignedString(key)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			})

			It("creates valid access object", func() {
				Expect(access).ToNot(BeNil())
			})
		})

		Context("when request has jwt token with invalid signing key", func() {
			BeforeEach(func() {
				mySigningKey := []byte("AllYourBase")

				token := jwt.New(jwt.SigningMethodHS256)
				tokenString, err := token.SignedString(mySigningKey)

				Expect(err).NotTo(HaveOccurred())
				req.Header.Add("Authorization", fmt.Sprintf("BEARER %s", tokenString))
			})

			It("creates valid access object", func() {
				Expect(access).ToNot(BeNil())
			})

		})
		Context("when request does not have jwt token set", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", "")
			})
			It("creates valid access object", func() {
				Expect(access).ToNot(BeNil())
			})
		})

		Context("when request does not have valid jwt token set", func() {
			BeforeEach(func() {
				req.Header.Add("Authorization", "blah-token")
			})
			It("creates valid access object", func() {
				Expect(access).ToNot(BeNil())
			})
		})
	})
})
