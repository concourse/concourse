package wrappa_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/wrappa"
	"github.com/honeycombio/libhoney-go"
	"github.com/honeycombio/libhoney-go/transmission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HoneycombHandler", func() {
	var (
		server *httptest.Server
		client *http.Client

		request  *http.Request
		response *http.Response
		sender   *transmission.MockSender
	)

	BeforeEach(func() {
		sender = &transmission.MockSender{}
		honeycombClient, err := libhoney.NewClient(
			libhoney.ClientConfig{
				Transmission: sender,
				APIKey:       "bees_are_good",
			})
		Expect(err).ToNot(HaveOccurred())
		defer honeycombClient.Close()

		server = httptest.NewServer(wrappa.HoneycombHandler{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "bees are good!")
			}),
			Client: honeycombClient,
		})

		client = &http.Client{}

		request, err = http.NewRequest("GET", server.URL, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		response, err = client.Do(request)
		Expect(err).ToNot(HaveOccurred())
	})

	It("does not interfere with the response contents", func() {
		body, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(body)).To(Equal("bees are good!\n"))
	})

	It("emits a Honeycomb event that contains information about the request and the response", func() {
		Expect(len(sender.Events())).To(Equal(1))
		Expect(sender.Events()[0].Data["request.content_length"]).To(Equal(int64(0)))
		Expect(sender.Events()[0].Data["request.method"]).To(Equal("GET"))
		Expect(sender.Events()[0].Data["request.header.user_agent"]).To(Equal("Go-http-client/1.1"))
		Expect(sender.Events()[0].Data["request.path"]).To(Equal("/"))
		Expect(sender.Events()[0].Data["request.url"]).To(Equal("/"))
		Expect(sender.Events()[0].Data["meta.type"]).To(Equal("http_request"))
		Expect(sender.Events()[0].Data["response.status_code"]).To(Equal(200))
	})
})
