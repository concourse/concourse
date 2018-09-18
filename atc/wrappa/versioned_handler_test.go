package wrappa_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/wrappa"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionedHandler", func() {
	var (
		version string

		server *httptest.Server
		client *http.Client

		request  *http.Request
		response *http.Response
	)

	BeforeEach(func() {
		version = "1.2.3-test"

		server = httptest.NewServer(wrappa.VersionedHandler{
			Version: version,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "sup")
			}),
		})

		client = &http.Client{}

		var err error
		request, err = http.NewRequest("GET", server.URL, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		response, err = client.Do(request)
		Expect(err).ToNot(HaveOccurred())
	})

	It("sets the X-Concourse-Version header", func() {
		Expect(response.Header.Get("X-Concourse-Version")).To(Equal("1.2.3-test"))
	})
})
