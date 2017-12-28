package auth_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/auth/authfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckAuthorizationHandler", func() {
	var (
		fakeValidator         *authfakes.FakeValidator
		fakeUserContextReader *authfakes.FakeUserContextReader
		fakeRejector          *authfakes.FakeRejector

		server *httptest.Server
		client *http.Client
	)

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBufferString("simple ")

		io.Copy(w, buffer)
		io.Copy(w, r.Body)
	})

	BeforeEach(func() {
		fakeValidator = new(authfakes.FakeValidator)
		fakeUserContextReader = new(authfakes.FakeUserContextReader)
		fakeRejector = new(authfakes.FakeRejector)

		fakeRejector.UnauthorizedStub = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		}

		fakeRejector.ForbiddenStub = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusForbidden)
		}

		server = httptest.NewServer(
			auth.WrapHandler( // for setting context on the request
				auth.CheckAuthorizationHandler(
					simpleHandler,
					fakeRejector,
				),
				fakeValidator,
				fakeUserContextReader,
			),
		)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Context("when a request is made", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", server.URL+"/teams/some-team/pipelines", bytes.NewBufferString("hello"))
			Expect(err).NotTo(HaveOccurred())
			urlValues := url.Values{":team_name": []string{"some-team"}}
			request.URL.RawQuery = urlValues.Encode()
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the request is authenticated", func() {
			BeforeEach(func() {
				fakeValidator.IsAuthenticatedReturns(true)
			})

			Context("when the bearer token's team matches the request's team", func() {
				BeforeEach(func() {
					fakeUserContextReader.GetTeamReturns("some-team", true, true)
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

			Context("when the bearer token's team is set to something other than the request's team", func() {
				BeforeEach(func() {
					fakeUserContextReader.GetTeamReturns("another-team", true, true)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					responseBody, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(responseBody)).To(Equal("nope\n"))
				})
			})
		})

		Context("when the request is not authenticated", func() {
			BeforeEach(func() {
				fakeValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(Equal("nope\n"))
			})

			Context("when the bearer token is for the requested team", func() {
				BeforeEach(func() {
					fakeUserContextReader.GetTeamReturns("some-team", true, true)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					responseBody, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(responseBody)).To(Equal("nope\n"))
				})
			})
		})
	})
})
