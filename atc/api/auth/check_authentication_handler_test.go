package auth_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/auth/authfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuthenticationHandler", func() {

	var (
		fakeAccess   *accessorfakes.FakeAccess
		fakeAccessor *accessorfakes.FakeAccessFactory
		fakeRejector *authfakes.FakeRejector

		server *httptest.Server
		client *http.Client

		err      error
		request  *http.Request
		response *http.Response
	)

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBufferString("simple hello")

		_, err := io.Copy(w, buffer)
		Expect(err).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		fakeAccess = new(accessorfakes.FakeAccess)
		fakeAccessor = new(accessorfakes.FakeAccessFactory)
		fakeRejector = new(authfakes.FakeRejector)

		fakeAccessor.CreateReturns(fakeAccess)

		fakeRejector.UnauthorizedStub = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		}

		client = http.DefaultClient
	})

	JustBeforeEach(func() {
		response, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CheckAuthenticationHandler", func() {

		BeforeEach(func() {
			innerHandler := auth.CheckAuthenticationHandler(
				simpleHandler,
				fakeRejector,
			)

			server = httptest.NewServer(accessor.NewHandler(
				logger,
				"some-action",
				innerHandler,
				fakeAccessor,
				new(accessorfakes.FakeTokenVerifier),
				new(accessorfakes.FakeTeamFetcher),
				new(auditorfakes.FakeAuditor),
				map[string]string{},
			))
		})

		Context("when a request is made", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the user is authenticated ", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)
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

			Context("when the user is not authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("rejects the request", func() {
					responseBody, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(responseBody)).To(Equal("nope\n"))
				})
			})
		})
	})

	Describe("CheckAuthenticationIfProvidedHandler", func() {

		BeforeEach(func() {
			innerHandler := auth.CheckAuthenticationIfProvidedHandler(
				simpleHandler,
				fakeRejector,
			)

			server = httptest.NewServer(accessor.NewHandler(
				logger,
				"some-action",
				innerHandler,
				fakeAccessor,
				new(accessorfakes.FakeTokenVerifier),
				new(accessorfakes.FakeTeamFetcher),
				new(auditorfakes.FakeAuditor),
				map[string]string{},
			))
		})

		Context("when a request is made", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when a token is provided", func() {
				BeforeEach(func() {
					fakeAccess.HasTokenReturns(true)
				})

				Context("when the user is not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})

					It("rejects the request", func() {
						responseBody, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(responseBody)).To(Equal("nope\n"))
					})
				})

				Context("when the user is authenticated ", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
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

			Context("when a token is NOT provided", func() {
				BeforeEach(func() {
					fakeAccess.HasTokenReturns(false)
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
	})
})
