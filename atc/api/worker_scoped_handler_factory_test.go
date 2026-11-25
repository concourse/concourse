package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/v3/lagertest"

	"github.com/concourse/concourse/v8/atc/api"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/db/dbfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerScopedHandlerFactory", func() {
	var (
		workerFactory *dbfakes.FakeWorkerFactory
		worker        *dbfakes.FakeWorker
		handler       http.Handler
		delegateCalls int
		request       *http.Request
	)

	BeforeEach(func() {
		workerFactory = new(dbfakes.FakeWorkerFactory)
		worker = new(dbfakes.FakeWorker)
		worker.TeamNameReturns("some-team")
		workerFactory.GetWorkerReturns(worker, true, nil)

		delegateCalls = 0
		factory := api.NewWorkerScopedHandlerFactory(lagertest.NewTestLogger("test"), workerFactory)
		handler = factory.HandlerFor(func(scopedWorker db.Worker) http.Handler {
			delegateCalls++
			Expect(scopedWorker).To(BeIdenticalTo(worker))
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r).To(BeIdenticalTo(request))
				w.WriteHeader(http.StatusNoContent)
			})
		})
	})

	serve := func(query string) int {
		request = httptest.NewRequest(http.MethodPost, "/?"+query, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}

	DescribeTable("delegates with the worker from the lookup",
		func(query, team, workerName string) {
			worker.TeamNameReturns(team)
			Expect(serve(query)).To(Equal(http.StatusNoContent))
			Expect(workerFactory.GetWorkerCallCount()).To(Equal(1))
			Expect(workerFactory.GetWorkerArgsForCall(0)).To(Equal(workerName))
			Expect(delegateCalls).To(Equal(1))
		},
		Entry("prefers the route parameter", ":worker_name=route-worker&worker_name=query-worker&team_name=some-team", "some-team", "route-worker"),
		Entry("falls back to the query parameter", "worker_name=query-worker&team_name=some-team", "some-team", "query-worker"),
		Entry("accepts an unassigned worker without a team parameter", "worker_name=query-worker", "", "query-worker"),
	)

	It("returns 400 without looking up a worker when the worker name is missing", func() {
		Expect(serve("team_name=some-team")).To(Equal(http.StatusBadRequest))
		Expect(workerFactory.GetWorkerCallCount()).To(BeZero())
		Expect(delegateCalls).To(BeZero())
	})

	It("returns 500 when the worker lookup fails", func() {
		workerFactory.GetWorkerReturns(nil, false, errors.New("worker lookup failed"))
		Expect(serve("worker_name=some-worker&team_name=some-team")).To(Equal(http.StatusInternalServerError))
		Expect(delegateCalls).To(BeZero())
	})

	It("returns 404 when the worker is not found", func() {
		workerFactory.GetWorkerReturns(nil, false, nil)
		Expect(serve("worker_name=some-worker&team_name=some-team")).To(Equal(http.StatusNotFound))
		Expect(delegateCalls).To(BeZero())
	})

	DescribeTable("returns 403 when the request does not match the worker's team",
		func(query string) {
			Expect(serve(query)).To(Equal(http.StatusForbidden))
			Expect(delegateCalls).To(BeZero())
		},
		Entry("a different team", "worker_name=some-worker&team_name=other-team"),
		Entry("no team", "worker_name=some-worker"),
	)
})
