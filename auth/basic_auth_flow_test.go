package auth_test

import (
	"encoding/base64"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
)

func basicAuthFlow(validatorFunc func(username string, password string) auth.Validator) {
	username := "username"
	password := "password"

	var validator auth.Validator

	BeforeEach(func() {
		validator = validatorFunc(username, password)
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
}

func b64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(str))
}
