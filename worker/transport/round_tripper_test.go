package transport_test

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker/transport"
	"github.com/concourse/atc/worker/transport/transportfakes"
	"github.com/concourse/retryhttp/retryhttpfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoundTripper #RoundTrip", func() {
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
		roundTripper = transport.NewRoundTripper("some-worker", "some-worker-address", fakeDB, fakeRoundTripper)
		requestUrl, err := url.Parse("http://1.2.3.4/something")
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

	It("sends the request with worker's garden address", func() {
		Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(1))
		actualRequest := fakeRoundTripper.RoundTripArgsForCall(0)
		Expect(actualRequest.URL.Host).To(Equal("some-worker-address"))
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

			savedWorker := db.SavedWorker{
				WorkerInfo: db.WorkerInfo{
					GardenAddr: "some-new-worker-address",
				},
				ExpiresIn: 123,
			}

			fakeDB.GetWorkerReturns(savedWorker, true, nil)
		})

		It("updates cached request host on subsequent call", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some-error"))

			Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(1))
			actualRequest := fakeRoundTripper.RoundTripArgsForCall(0)
			Expect(actualRequest.URL.Host).To(Equal("some-worker-address"))
			Expect(fakeDB.GetWorkerCallCount()).To(Equal(0))

			_, err := roundTripper.RoundTrip(&request)
			Expect(err).To(HaveOccurred())

			Expect(fakeDB.GetWorkerCallCount()).To(Equal(1))
			Expect(fakeRoundTripper.RoundTripCallCount()).To(Equal(2))
			actualRequest = fakeRoundTripper.RoundTripArgsForCall(1)
			Expect(actualRequest.URL.Host).To(Equal("some-new-worker-address"))
		})

		Context("when the lookup of the worker in the db errors", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("some-db-error")
				fakeDB.GetWorkerReturns(db.SavedWorker{}, true, expectedErr)
			})

			It("throws an error", func() {
				_, err := roundTripper.RoundTrip(&request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr.Error()))
			})
		})

		Context("when the worker is not found in the db", func() {
			BeforeEach(func() {
				fakeDB.GetWorkerReturns(db.SavedWorker{}, false, nil)
			})

			It("throws an error", func() {
				_, err := roundTripper.RoundTrip(&request)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(transport.ErrMissingWorker{WorkerName: "some-worker"}))
			})
		})
	})
})
