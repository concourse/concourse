package concourse_test

import (
	"net/http"

	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"testing"
)

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Concourse Client Suite")
}

var (
	atcServer  *ghttp.Server
	connection concourse.Connection
	client     concourse.Client
)

var _ = BeforeEach(func() {
	var err error
	atcServer = ghttp.NewServer()

	connection, err = concourse.NewConnection(
		atcServer.URL(),
		&http.Client{},
	)
	Expect(err).NotTo(HaveOccurred())

	client = concourse.NewClient(connection)
})

var _ = AfterEach(func() {
	atcServer.Close()
})
