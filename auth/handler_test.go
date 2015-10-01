package auth_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"code.google.com/p/go.crypto/bcrypt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
)

var _ = Describe("BasicAuthHandler", func() {
	username := "username"
	password := "password"

	var validator auth.Validator

	var server *httptest.Server
	var client *http.Client

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBufferString("simple ")

		io.Copy(w, buffer)
		io.Copy(w, r.Body)
	})

	itAuthenticates := func() {
		BeforeEach(func() {
			authHandler := auth.Handler{
				Handler:   simpleHandler,
				Validator: validator,
			}

			server = httptest.NewServer(authHandler)

			client = &http.Client{
				Transport: &http.Transport{},
			}
		})

		AfterEach(func() {
			server.Close()
		})

		Context("with the correct credentials", func() {
			var request *http.Request
			var response *http.Response

			BeforeEach(func() {
				var err error

				request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("via standard basic auth", func() {
				BeforeEach(func() {
					request.SetBasicAuth(username, password)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("proxies to the handler", func() {
					responseBody, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(responseBody)).To(Equal("simple hello"))
				})
			})
		})

		Context("with incorrect credentials", func() {
			It("returns 401", func() {
				requestBody := bytes.NewBufferString("hello")
				request, err := http.NewRequest("GET", server.URL, requestBody)
				Expect(err).NotTo(HaveOccurred())
				request.SetBasicAuth(username, "wrong")

				response, err := client.Do(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				Expect(response.Header.Get("WWW-Authenticate")).To(Equal(`Basic realm="Restricted"`))

				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(Equal("not authorized"))
			})
		})

		Context("with no credentials", func() {
			It("returns 401", func() {
				requestBody := bytes.NewBufferString("hello")
				request, err := http.NewRequest("GET", server.URL, requestBody)
				Expect(err).NotTo(HaveOccurred())

				response, err := client.Do(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				Expect(response.Header.Get("WWW-Authenticate")).To(Equal(`Basic realm="Restricted"`))

				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(Equal("not authorized"))
			})
		})
	}

	Context("with a username + hashed password validator", func() {
		BeforeEach(func() {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
			Expect(err).NotTo(HaveOccurred())

			validator = auth.BasicAuthHashedValidator{
				Username:       username,
				HashedPassword: string(hashedPassword),
			}
		})

		itAuthenticates()
	})

	Context("with a username + plaintext password validator", func() {
		BeforeEach(func() {
			validator = auth.BasicAuthValidator{
				Username: username,
				Password: password,
			}
		})

		itAuthenticates()
	})
})

var _ = Describe("ExtractUsernameAndPassword", func() {
	Context("When the string starts with 'Basic '", func() {
		Context("When the rest of the string is two non-empty strings separated by a colon, base64-encoded", func() {
			It("returns the username and password", func() {
				username, password, err := auth.ExtractUsernameAndPassword(header("username", "password"))
				Expect(err).NotTo(HaveOccurred())
				Expect(username).To(Equal("username"))
				Expect(password).To(Equal("password"))
			})
		})

		Context("When the rest of the string is has no colon, base64-encoded", func() {
			It("errors", func() {
				_, _, err := auth.ExtractUsernameAndPassword(header("usernamepassword"))
				Expect(err).To(HaveOccurred())
			})
		})

		Context("When the rest of the string is has too many colons, base64-encoded", func() {
			It("errors", func() {
				_, _, err := auth.ExtractUsernameAndPassword(header("too", "many", "things"))
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When the string doesn't start with 'Basic '", func() {
		It("errors", func() {
			credentials := []byte("username:password")
			bustedHeader := "baysick  " + base64.StdEncoding.EncodeToString(credentials)
			_, _, err := auth.ExtractUsernameAndPassword(bustedHeader)
			Expect(err).To(HaveOccurred())
		})
	})
})
