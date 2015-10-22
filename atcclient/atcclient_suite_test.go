package atcclient_test

import (
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
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
	atcServer *ghttp.Server
	client    atcclient.Client
	handler   atcclient.Handler
)

var _ = BeforeEach(func() {
	var err error
	atcServer = ghttp.NewServer()

	client, err = atcclient.NewClient(
		rc.NewTarget(atcServer.URL(), "", "", "", false),
	)
	Expect(err).NotTo(HaveOccurred())

	handler = atcclient.NewAtcHandler(client)
})

var _ = AfterEach(func() {
	atcServer.Close()
})
