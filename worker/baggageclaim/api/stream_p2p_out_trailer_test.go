package api_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/v8/tracing"
	"github.com/concourse/concourse/v8/worker/baggageclaim"
	"github.com/concourse/concourse/v8/worker/baggageclaim/api"
	bclient "github.com/concourse/concourse/v8/worker/baggageclaim/client"
	"github.com/concourse/concourse/v8/worker/baggageclaim/volume"
	"github.com/concourse/concourse/v8/worker/baggageclaim/volume/volumefakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// These specs drive the stream-p2p-out handler with a fake repository, so the
// byte count trailer is checked over real HTTP without needing to spawn tar.
var _ = Describe("stream-p2p-out byte count trailer", func() {
	// The handler only accepts a stream-in URL with an IP and a port, like the
	// ones GetP2pUrl returns.
	const streamInURL = "http://127.0.0.1:7788/volumes/dest/stream-in"

	var (
		fakeRepo *volumefakes.FakeRepository
		server   *httptest.Server
	)

	BeforeEach(func() {
		fakeRepo = new(volumefakes.FakeRepository)
		handler, err := api.NewHandler(lagertest.NewTestLogger("volume-server"), nil, fakeRepo, regexp.MustCompile("eth0"), 4, 7766)
		Expect(err).NotTo(HaveOccurred())
		server = httptest.NewServer(handler)
	})

	AfterEach(func() {
		server.Close()
	})

	// streamP2pOut makes the request the ATC makes and returns the drained body
	// text with the response, whose trailers are only set after EOF.
	streamP2pOut := func() (string, *http.Response) {
		request, err := http.NewRequest("PUT", server.URL+"/volumes/some-handle/stream-p2p-out?path=.&streamInURL="+streamInURL+"&encoding=gzip", nil)
		Expect(err).NotTo(HaveOccurred())
		response, err := http.DefaultClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(response.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Body.Close()).To(Succeed())
		return strings.TrimSpace(string(body)), response
	}

	It("reports the bytes sent as a trailer after the unchanged ok body", func() {
		fakeRepo.StreamP2pOutReturns(1234, nil)

		body, response := streamP2pOut()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		Expect(body).To(Equal("ok"))
		Expect(response.Trailer.Get(baggageclaim.StreamP2pOutBytesTrailer)).To(Equal("1234"))
		Expect(response.Header.Values(baggageclaim.StreamP2pOutBytesTrailer)).To(BeEmpty(), "the count must travel only as a trailer")
	})

	It("reports the partial byte count alongside an error body", func() {
		fakeRepo.StreamP2pOutReturns(12, errors.New("p2p-stream-in 500: disk full"))

		body, response := streamP2pOut()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		Expect(body).To(Equal("failed to stream p2p out from volume: p2p-stream-in 500: disk full"))
		Expect(response.Trailer.Get(baggageclaim.StreamP2pOutBytesTrailer)).To(Equal("12"))
	})

	It("is recorded by the baggageclaim client on its span", func() {
		spanRecorder := new(tracetest.SpanRecorder)
		tracing.ConfigureTraceProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)))
		defer func() { tracing.Configured = false }()

		fakeRepo.StreamP2pOutReturns(4096, nil)
		fakeRepo.GetVolumeReturns(volume.Volume{Handle: "some-handle", Path: "/some/path"}, true, nil)

		vol, found, err := bclient.New(server.URL, http.DefaultTransport).LookupVolume(context.Background(), "some-handle")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		err = vol.StreamP2pOut(context.Background(), ".", streamInURL, baggageclaim.GzipEncoding)
		Expect(err).NotTo(HaveOccurred())

		var clientSpan sdktrace.ReadOnlySpan
		for _, span := range spanRecorder.Ended() {
			if span.Name() == "volumeClient.streamP2pOut" {
				clientSpan = span
			}
		}
		Expect(clientSpan).NotTo(BeNil())
		Expect(clientSpan.Attributes()).To(ContainElement(attribute.Int64("streamed-bytes", 4096)))
	})
})
