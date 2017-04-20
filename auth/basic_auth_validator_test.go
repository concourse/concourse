package auth_test

import (
	"encoding/base64"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/dbng/dbngfakes"
)

var _ = Describe("BasicAuthValidator", func() {
	username := "username"
	password := "password"

	var (
		validator auth.Validator
		fakeTeam  *dbngfakes.FakeTeam
	)
	BeforeEach(func() {
		fakeTeam = new(dbngfakes.FakeTeam)
		encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 4)
		Expect(err).ToNot(HaveOccurred())

		fakeTeam.NameReturns(atc.DefaultTeamName)
		fakeTeam.BasicAuthReturns(&atc.BasicAuth{
			BasicAuthUsername: username,
			BasicAuthPassword: string(encryptedPassword),
		})

		validator = auth.NewBasicAuthValidator(fakeTeam)
	})

	Describe("IsAuthenticated", func() {
		var (
			request *http.Request

			isAuthenticated bool
		)

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", "http://example.com", nil)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			isAuthenticated = validator.IsAuthenticated(request)
		})

		Context("when the request's basic auth header has the correct credentials", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", "Basic "+b64(username+":"+password))
			})

			It("returns true", func() {
				Expect(isAuthenticated).To(BeTrue())
			})

			Context("with different casing", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "bAsIc "+b64(username+":"+password))
				})

				It("returns true", func() {
					Expect(isAuthenticated).To(BeTrue())
				})
			})
		})

		Context("when the request's basic auth header has incorrect correct credentials", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", "Basic "+b64(username+":bogus-"+password))
			})

			It("returns false", func() {
				Expect(isAuthenticated).To(BeFalse())
			})
		})

		Context("when the request's Authorization header isn't basic auth", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", "Bearer "+b64(username+":"+password))
			})

			It("returns false", func() {
				Expect(isAuthenticated).To(BeFalse())
			})
		})
	})
})

func b64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
