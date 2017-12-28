package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/auth/authfakes"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/tedsuo/rata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CheckWorkerTeamAccessHandler", func() {
	var (
		response      *http.Response
		server        *httptest.Server
		delegate      *workerDelegateHandler
		workerFactory *dbfakes.FakeWorkerFactory
		handler       http.Handler

		authValidator     *authfakes.FakeValidator
		userContextReader *authfakes.FakeUserContextReader
		fakeWorker        *dbfakes.FakeWorker
	)

	BeforeEach(func() {
		workerFactory = new(dbfakes.FakeWorkerFactory)
		authValidator = new(authfakes.FakeValidator)
		userContextReader = new(authfakes.FakeUserContextReader)

		handlerFactory := auth.NewCheckWorkerTeamAccessHandlerFactory(workerFactory)

		delegate = &workerDelegateHandler{}
		checkWorkerTeamAccessHandler := handlerFactory.HandlerFor(delegate, auth.UnauthorizedRejector{})
		handler = auth.WrapHandler(checkWorkerTeamAccessHandler, authValidator, userContextReader)
	})

	JustBeforeEach(func() {
		routes := rata.Routes{}
		for _, route := range atc.Routes {
			if route.Name == atc.RetireWorker {
				routes = append(routes, route)
			}
		}

		router, err := rata.NewRouter(routes, map[string]http.Handler{
			atc.RetireWorker: handler,
		})
		Expect(err).NotTo(HaveOccurred())
		server = httptest.NewServer(router)

		requestGenerator := rata.NewRequestGenerator(server.URL, atc.Routes)
		request, err := requestGenerator.CreateRequest(atc.RetireWorker, rata.Params{
			"worker_name": "some-worker",
		}, nil)
		Expect(err).NotTo(HaveOccurred())

		response, err = new(http.Client).Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	var _ = AfterEach(func() {
		server.Close()
	})

	Context("when not authenticated", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(false)
		})

		It("returns 401", func() {
			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("does not call the scoped handler", func() {
			Expect(delegate.IsCalled).To(BeFalse())
		})
	})

	Context("when authenticated", func() {
		BeforeEach(func() {
			authValidator.IsAuthenticatedReturns(true)
			userContextReader.GetTeamReturns("some-team", false, true)
		})

		Context("when worker exists and belongs to a team", func() {
			BeforeEach(func() {
				fakeWorker = new(dbfakes.FakeWorker)
				fakeWorker.NameReturns("some-worker")
				fakeWorker.TeamNameReturns("some-team")

				workerFactory.GetWorkerReturns(fakeWorker, true, nil)
			})

			Context("when team in auth matches worker team", func() {
				BeforeEach(func() {
					userContextReader.GetTeamReturns("some-team", false, true)
				})

				It("fetches worker by the correct name", func() {
					Expect(workerFactory.GetWorkerArgsForCall(0)).To(Equal("some-worker"))
				})

				It("calls worker delegate", func() {
					Expect(delegate.IsCalled).To(BeTrue())
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when team in auth does not match worker team", func() {
				BeforeEach(func() {
					userContextReader.GetTeamReturns("some-other-team", false, true)
				})

				It("fetches worker by the correct name", func() {
					Expect(workerFactory.GetWorkerArgsForCall(0)).To(Equal("some-worker"))
				})

				It("does not call worker delegate", func() {
					Expect(delegate.IsCalled).To(BeFalse())
				})

				It("returns 403 Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when worker is not owned by a team", func() {
			BeforeEach(func() {
				fakeWorker = new(dbfakes.FakeWorker)
				fakeWorker.NameReturns("some-worker")

				workerFactory.GetWorkerReturns(fakeWorker, true, nil)
			})

			Context("when team in auth is admin", func() {
				BeforeEach(func() {
					userContextReader.GetTeamReturns("admin-team", true, true)
				})

				It("calls worker delegate", func() {
					Expect(delegate.IsCalled).To(BeTrue())
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when team in auth is not admin", func() {
				BeforeEach(func() {
					userContextReader.GetTeamReturns("some-other-team", false, true)
				})

				It("does not call worker delegate", func() {
					Expect(delegate.IsCalled).To(BeFalse())
				})

				It("returns 403 Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when worker does not exist", func() {
			BeforeEach(func() {
				workerFactory.GetWorkerReturns(nil, false, nil)
			})

			It("does not call worker delegate", func() {
				Expect(delegate.IsCalled).To(BeFalse())
			})

			It("returns 404 Not found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when getting worker fails", func() {
			BeforeEach(func() {
				workerFactory.GetWorkerReturns(nil, false, errors.New("disaster"))
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("does not call the scoped handler", func() {
				Expect(delegate.IsCalled).To(BeFalse())
			})
		})
	})
})

type workerDelegateHandler struct {
	IsCalled bool
}

func (handler *workerDelegateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.IsCalled = true
}
