package worker_test

import (
	"net/http"
	"net/url"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GardenConnectionFactory", func() {
	Describe("WorkerLookupRoundTripper #RoundTrip", func() {
		var (
			request     http.Request
			savedWorker db.SavedWorker
			response    *http.Response
			err         error
		)

		BeforeEach(func() {
			gcfDB := new(fakes.FakeGardenConnectionFactoryDB)
			wlrt := worker.CreateWorkerLookupRoundTripper("some-worker", gcfDB, &fakeRoundTripper{})
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

			response, err = wlrt.RoundTrip(&request)
		})

		It("sends the request with worker's garden address", func() {
			Expect(response.Request.URL.Host).To(Equal(savedWorker.GardenAddr))
			Expect(request.URL.Path).To(Equal("/something"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
