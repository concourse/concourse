package auth_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/auth/authfakes"
	"github.com/concourse/concourse/atc/auditor/auditorfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckAuthorizationHandler", func() {
	var (
		fakeAccessor *accessorfakes.FakeAccessFactory
		fakeaccess   *accessorfakes.FakeAccess
		fakeRejector *authfakes.FakeRejector
		fakePipeline *dbfakes.FakePipeline

		server *httptest.Server
		client *http.Client
	)

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBufferString("simple ")

		io.Copy(w, buffer)
		io.Copy(w, r.Body)
	})

	BeforeEach(func() {
		fakeAccessor = new(accessorfakes.FakeAccessFactory)
		fakeaccess = new(accessorfakes.FakeAccess)
		fakeRejector = new(authfakes.FakeRejector)
		fakePipeline = new(dbfakes.FakePipeline)

		fakeRejector.UnauthorizedStub = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		}

		fakeRejector.ForbiddenStub = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusForbidden)
		}

		innerHandler := auth.CheckAuthorizationHandler(
			simpleHandler,
			fakeRejector,
		)

		server = httptest.NewServer(wrapContext(fakePipeline, accessor.NewHandler(
			logger,
			"some-action",
			innerHandler,
			fakeAccessor,
			new(auditorfakes.FakeAuditor),
			map[string]string{},
		)))

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess, nil)
	})

	Context("when a request is made to a /teams/... endpoint", func() {
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
				fakeaccess.IsAuthenticatedReturns(true)
			})

			Context("when the bearer token's team matches the request's team", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthorizedReturns(true)
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
					fakeaccess.IsAuthorizedReturns(false)
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
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(Equal("nope\n"))
			})
		})
	})

	Context("when a request is made to a /pipelines/... endpoint", func() {
		It("checks the authorization using the team of the pipeline", func() {
			fakeaccess.IsAuthenticatedReturns(true)
			fakePipeline.TeamNameReturns("some-team")

			request, err := http.NewRequest("GET", server.URL+"/pipelines/1", nil)
			Expect(err).NotTo(HaveOccurred())

			_, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeaccess.IsAuthorizedCallCount()).To(Equal(1))
			Expect(fakeaccess.IsAuthorizedArgsForCall(0)).To(Equal("some-team"))
		})
	})
})
