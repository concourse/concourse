package worker_test

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
	fakeretryhttp "github.com/concourse/retryhttp/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GardenConnectionFactory", func() {
	Describe("WorkerLookupRoundTripper #RoundTrip", func() {
		var (
			request           http.Request
			gcfDB             *fakes.FakeGardenConnectionFactoryDB
			savedWorker       db.SavedWorker
			innerRoundTripper *fakeretryhttp.FakeRoundTripper
			wlrt              http.RoundTripper
			response          *http.Response
			err               error
		)

		BeforeEach(func() {
			gcfDB = new(fakes.FakeGardenConnectionFactoryDB)
			innerRoundTripper = new(fakeretryhttp.FakeRoundTripper)
			wlrt = worker.CreateWorkerLookupRoundTripper("some-worker", gcfDB, innerRoundTripper)
			requestUrl, err := url.Parse("http://1.2.3.4/something")
			Expect(err).NotTo(HaveOccurred())

			request = http.Request{
				URL: requestUrl,
			}

			savedWorker = db.SavedWorker{
				db.WorkerInfo{
					GardenAddr: "some-garden-addr",
				},
				123,
			}

			gcfDB.GetWorkerReturns(savedWorker, true, nil)

			innerRoundTripper.RoundTripReturns(&http.Response{Request: &request}, nil)
		})

		JustBeforeEach(func() {
			response, err = wlrt.RoundTrip(&request)
		})

		It("sends the request with worker's garden address", func() {
			Expect(response.Request.URL.Host).To(Equal(savedWorker.GardenAddr))
			Expect(request.URL.Path).To(Equal("/something"))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the lookup of the worker in the db errors", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("some-db-error")
				gcfDB.GetWorkerReturns(db.SavedWorker{}, true, expectedErr)
			})

			It("throws an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr.Error()))
			})
		})

		Context("when the lookup of the worker in the db errors", func() {
			BeforeEach(func() {
				gcfDB.GetWorkerReturns(db.SavedWorker{}, false, nil)
			})

			It("throws an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(worker.ErrMissingWorker.Error()))
			})
		})

		It("reuses the request cached host on subsequent calls", func() {
			Expect(gcfDB.GetWorkerCallCount()).To(Equal(1))
			_, err := wlrt.RoundTrip(&request)
			Expect(err).NotTo(HaveOccurred())
			Expect(gcfDB.GetWorkerCallCount()).To(Equal(1))
		})

		Context("when inner rountrip fails", func() {
			BeforeEach(func() {
				innerRoundTripper.RoundTripReturns(nil, errors.New("some-error"))
			})

			It("updates cached request host", func() {
				Expect(gcfDB.GetWorkerCallCount()).To(Equal(1))
				_, err := wlrt.RoundTrip(&request)
				Expect(err).To(HaveOccurred())
				Expect(gcfDB.GetWorkerCallCount()).To(Equal(2))
			})
		})
	})
})
