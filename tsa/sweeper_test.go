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

var _ = Describe("Sweeper", func() {
	var (
		sweeper *tsa.Sweeper

		ctx     context.Context
		worker  atc.Worker
		fakeATC *ghttp.Server
		data    []byte
	)

	BeforeEach(func() {
		ctx = lagerctx.NewContext(context.Background(), lagertest.NewTestLogger("test"))

		worker = atc.Worker{
			Name: "some-worker",
			Team: "some-team",
		}

		fakeATC = ghttp.NewServer()

		atcEndpoint := rata.NewRequestGenerator(fakeATC.URL(), atc.Routes)

		token := &oauth2.Token{TokenType: "Bearer", AccessToken: "yo"}
		httpClient := oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(token))

		sweeper = &tsa.Sweeper{
			ATCEndpoint: atcEndpoint,
			HTTPClient:  httpClient,
		}

		expectedBody := []string{"handle1", "handle2"}

		var err error
		data, err = json.Marshal(expectedBody)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		fakeATC.Close()
	})

	Context("ResourceAction missing", func() {
		It("Returns an error", func() {
			handles, err := sweeper.Sweep(ctx, worker, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(tsa.ResourceActionMissing))
			Expect(handles).To(BeNil())
		})
	})

	Context("Containers", func() {

		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/containers/destroying"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
				ghttp.RespondWith(200, data, nil),
			))
		})

		It("tells the ATC to get destroying containers", func() {
			handles, err := sweeper.Sweep(ctx, worker, tsa.SweepContainers)
			Expect(err).NotTo(HaveOccurred())

			Expect(handles).To(Equal(data))
			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})

		Context("when the ATC responds with a 403", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers/destroying"),
					ghttp.RespondWith(403, nil, nil),
				))
			})

			It("errors", func() {
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepContainers)
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
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepContainers)
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
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepContainers)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the ATC responds with non 200", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/containers/destroying"),
					ghttp.RespondWith(500, nil, nil),
				))
			})

			It("errors", func() {
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepContainers)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("bad-response (500)")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})
	})

	Context("Volumes", func() {

		BeforeEach(func() {
			fakeATC.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying"),
				ghttp.VerifyHeaderKV("Authorization", "Bearer yo"),
				ghttp.RespondWith(200, data, nil),
			))
		})

		It("tells the ATC to get destroying volumes", func() {
			handles, err := sweeper.Sweep(ctx, worker, tsa.SweepVolumes)
			Expect(err).NotTo(HaveOccurred())

			Expect(handles).To(Equal(data))
			Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
		})

		Context("when the ATC responds with a 403", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying"),
					ghttp.RespondWith(403, nil, nil),
				))
			})

			It("errors", func() {
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepVolumes)
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
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepVolumes)
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
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepVolumes)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the ATC responds with non 200", func() {
			BeforeEach(func() {
				fakeATC.Reset()
				fakeATC.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/volumes/destroying"),
					ghttp.RespondWith(500, nil, nil),
				))
			})

			It("errors", func() {
				_, err := sweeper.Sweep(ctx, worker, tsa.SweepVolumes)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("bad-response (500)")))
				Expect(fakeATC.ReceivedRequests()).To(HaveLen(1))
			})
		})
	})
})
