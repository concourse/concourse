package tsa_test

import (
	"context"
	"encoding/json"

	"github.com/concourse/concourse/tsa"
	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("Worker Status Test", func() {
	var (
		workerStatus *tsa.WorkerStatus

		ctx     context.Context
		worker  atc.Worker
		fakeATC *ghttp.Server
		data    []byte
	)

	BeforeEach(func() {
		var err error
		ctx = lagerctx.NewContext(context.Background(), lagertest.NewTestLogger("test"))
		worker = atc.Worker{
			Name: "some-worker",
			Team: "some-team",
		}

		fakeATC = ghttp.NewServer()

		atcEndpoint := rata.NewRequestGenerator(fakeATC.URL(), atc.Routes)

		token := &oauth2.Token{TokenType: "Bearer", AccessToken: "yo"}
		httpClient := oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token))

		workerStatus = &tsa.WorkerStatus{
			ATCEndpoint:      atcEndpoint,
			HTTPClient:       httpClient,
			ContainerHandles: []string{"handle1", "handle2"},
			VolumeHandles:    []string{"handle1", "handle2"},
		}

		expectedBody := []string{"handle1", "handle2"}
		data, err = json.Marshal(expectedBody)
		Expect(err).ShouldNot(HaveOccurred())

	})

	AfterEach(func() {
		fakeATC.Close()
	})

	Context("ResourceType not valid", func() {
		It("Returns an error", func() {
			err := workerStatus.WorkerStatus(ctx, worker, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(tsa.ResourceActionMissing))
		})
	})

	Context("Containers", func() {

		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/containers/report"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
				ghttp.VerifyJSON(`["handle1","handle2"]`),
				ghttp.RespondWith(204, data, nil),
			))
		})

		It("tells the ATC to get destroying containers", func() {
			err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportContainers)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the ATC responds with a 403", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/containers/report"),
					ghttp.RespondWith(403, nil, nil),
				))
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportContainers)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("403")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when the worker name is empty", func() {
			BeforeEach(func() {
				worker.Name = ""
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportContainers)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("empty-worker-name")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the call to ATC fails", func() {
			BeforeEach(func() {
				fakeATC.Close()
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportContainers)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the ATC responds with non 200", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/containers/report"),
					ghttp.RespondWith(500, nil, nil),
				))
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportContainers)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("bad-response (500)")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when handle is empty list", func() {
			BeforeEach(func() {
				workerStatus.ContainerHandles = []string{}
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/containers/report"),
					ghttp.VerifyJSON(`[]`),
					ghttp.RespondWith(204, nil, nil),
				))
			})

			It("does not error", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportContainers)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})
	})

	Context("Volumes", func() {

		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("PUT", "/api/v1/volumes/report"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
				ghttp.VerifyJSON(`["handle1","handle2"]`),
				ghttp.RespondWith(204, data, nil),
			))
		})

		It("tells the ATC to get destroying volumes", func() {
			err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportVolumes)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the ATC responds with a 403", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/volumes/report"),
					ghttp.RespondWith(403, nil, nil),
				))
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportVolumes)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("403")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when the worker name is empty", func() {
			BeforeEach(func() {
				worker.Name = ""
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportVolumes)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("empty-worker-name")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the call to ATC fails", func() {
			BeforeEach(func() {
				fakeATC.Close()
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportVolumes)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the ATC responds with non 200", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/volumes/report"),
					ghttp.RespondWith(500, nil, nil),
				))
			})

			It("errors", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportVolumes)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("bad-response (500)")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when handle is empty list", func() {
			BeforeEach(func() {
				workerStatus.VolumeHandles = []string{}
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/api/v1/volumes/report"),
					ghttp.VerifyJSON(`[]`),
					ghttp.RespondWith(204, nil, nil),
				))
			})

			It("does not error", func() {
				err := workerStatus.WorkerStatus(ctx, worker, tsa.ReportVolumes)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})
	})
})
