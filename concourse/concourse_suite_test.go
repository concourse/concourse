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
	atcServer *ghttp.Server
	client    concourse.Client
)

var _ = BeforeEach(func() {
	atcServer = ghttp.NewServer()

	client = concourse.NewClient(
		atcServer.URL(),
		&http.Client{},
	)
})

var _ = AfterEach(func() {
	atcServer.Close()
})
