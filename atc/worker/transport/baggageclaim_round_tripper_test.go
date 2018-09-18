package transport_test

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/worker/transport"
	"github.com/concourse/atc/worker/transport/transportfakes"
	"github.com/concourse/retryhttp/retryhttpfakes"

	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BaggageclaimRoundTripper #RoundTrip", func() {
	var (
		request          http.Request
		fakeDB           *transportfakes.FakeTransportDB
		fakeRoundTripper *retryhttpfakes.FakeRoundTripper
		roundTripper     http.RoundTripper
		response         *http.Response
		err              error
	)

	BeforeEach(func() {
		fakeDB = new(transportfakes.FakeTransportDB)
		fakeRoundTripper = new(retryhttpfakes.FakeRoundTripper)
		workerBaggageClaimURL := "http://1.2.3.4:7878"
		roundTripper = transport.NewBaggageclaimRoundTripper("some-worker", &workerBaggageClaimURL, fakeDB, fakeRoundTripper)
		requestUrl, err := url.Parse("/something")
		Expect(err).NotTo(HaveOccurred())

		request = http.Request{
			URL: requestUrl,
		}

		fakeRoundTripper.RoundTripReturns(&http.Response{StatusCode: http.StatusTeapot}, nil)
	})

	JustBeforeEach(func() {
		response, err = roundTripper.RoundTrip(&request)
	})

	It("returns the response", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(response).To(Equal(&http.Response{StatusCode: http.StatusTeapot}))
	})

	It("sends the request with worker's baggageclaim url", func() {
		Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(1))
		actualRequest := fakeRoundTripper.RoundTripArgsForCall(0)
		Expect(actualRequest.URL.Scheme).To(Equal("http"))
		Expect(actualRequest.URL.Host).To(Equal("1.2.3.4:7878"))
		Expect(actualRequest.URL.Path).To(Equal("/something"))
	})

	It("reuses the request cached host on subsequent calls", func() {
		Expect(fakeDB.GetWorkerCallCount()).To(Equal(0))
		_, err := roundTripper.RoundTrip(&request)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeDB.GetWorkerCallCount()).To(Equal(0))
	})

	Context("when inner roundtrip fails", func() {
		BeforeEach(func() {
			fakeRoundTripper.RoundTripReturns(nil, errors.New("some-error"))

			bcURL := "http://5.6.7.8:7878"
			savedWorker := new(dbfakes.FakeWorker)
			savedWorker.BaggageclaimURLReturns(&bcURL)
			savedWorker.ExpiresAtReturns(time.Now().Add(123 * time.Minute))
			savedWorker.StateReturns(db.WorkerStateRunning)

			fakeDB.GetWorkerReturns(savedWorker, true, nil)
		})

		It("updates cached request host on subsequent call", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some-error"))

			Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(1))
			actualRequest := fakeRoundTripper.RoundTripArgsForCall(0)
			Expect(actualRequest.URL.Host).To(Equal("1.2.3.4:7878"))
			Expect(fakeDB.GetWorkerCallCount()).To(Equal(0))

			_, err := roundTripper.RoundTrip(&request)
			Expect(err).To(HaveOccurred())

			Expect(fakeDB.GetWorkerCallCount()).To(Equal(1))
			Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(2))
			actualRequest = fakeRoundTripper.RoundTripArgsForCall(1)
			Expect(actualRequest.URL.Host).To(Equal("5.6.7.8:7878"))
		})

		Context("when the lookup of the worker in the db errors", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("some-db-error")
				fakeDB.GetWorkerReturns(nil, true, expectedErr)
			})

			It("throws an error", func() {
				_, err := roundTripper.RoundTrip(&request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr.Error()))
			})
		})

		Context("when the worker is in the DB and the baggageclaim URL is empty", func() {
			BeforeEach(func() {

				runningWorker := new(dbfakes.FakeWorker)
				runningWorker.StateReturns(db.WorkerStateStalled)
				runningWorker.BaggageclaimURLReturns(nil)

				fakeDB.GetWorkerReturns(runningWorker, true, nil)
			})

			It("throws a descriptive error", func() {
				_, err := roundTripper.RoundTrip(&request)
				Expect(err).To(MatchError("worker 'some-worker' is unreachable (state is 'stalled')"))
			})
		})

		Context("when the worker is not found in the db", func() {
			BeforeEach(func() {
				fakeDB.GetWorkerReturns(nil, false, nil)
			})

			It("throws an error", func() {
				_, err := roundTripper.RoundTrip(&request)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(transport.WorkerMissingError{WorkerName: "some-worker"}))
			})
		})
	})
})
