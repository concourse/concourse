package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/concourse/skymarshal/auth"
	"github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuthTokenGenerator", func() {
	var tokenGenerator auth.AuthTokenGenerator
	var priv *rsa.PrivateKey

	BeforeEach(func() {
		var err error
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).NotTo(HaveOccurred())

		tokenGenerator = auth.NewAuthTokenGenerator(priv)
	})

	decodeFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return priv.Public(), nil
	}

	Describe("GenerateToken", func() {
		It("sets team name, admin, csrf", func() {
			csrfToken := "some-csrf-token"
			tokenType, tokenValue, err := tokenGenerator.GenerateToken(time.Now().Add(1*time.Hour), "some-team", false, csrfToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(tokenType)).To(Equal("Bearer"))

			token, err := jwt.Parse(string(tokenValue), decodeFunc)
			Expect(err).NotTo(HaveOccurred())
			claims := token.Claims.(jwt.MapClaims)
			Expect(claims["teamName"]).To(Equal("some-team"))
			Expect(claims["isAdmin"]).To(Equal(false))
			Expect(claims["csrf"]).To(Equal(csrfToken))
		})
	})
})
