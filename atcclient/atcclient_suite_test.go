package atcclient_test

import (
	"net/http"

	"github.com/concourse/fly/atcclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"testing"
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ATC Client Suite")
}

var (
	atcServer  *ghttp.Server
	connection atcclient.Connection
	client     atcclient.Client
)

var _ = BeforeEach(func() {
	var err error
	atcServer = ghttp.NewServer()

	connection, err = atcclient.NewConnection(
		atcServer.URL(),
		&http.Client{},
	)
	Expect(err).NotTo(HaveOccurred())

	client = atcclient.NewClient(connection)
})

var _ = AfterEach(func() {
	atcServer.Close()
})
