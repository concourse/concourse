package legacyserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/skymarshal/legacyserver"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	logger *lagertest.TestLogger

	server *httptest.Server
	client *http.Client
)

var _ = BeforeEach(func() {

	logger = lagertest.NewTestLogger("legacyserver")

	handler, err := legacyserver.NewLegacyServer(&legacyserver.LegacyConfig{
		Logger: logger,
	})
	Expect(err).NotTo(HaveOccurred())

	server = httptest.NewServer(handler)

	client = &http.Client{
		Transport: &http.Transport{},
	}
})

var _ = AfterEach(func() {
	server.Close()
})

func TestDexServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Legacy Server Suite")
}
