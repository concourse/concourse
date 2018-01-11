package skymarshal_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal"
	"github.com/concourse/skymarshal/auth/authfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	sink *lager.ReconfigurableSink

	externalURL  = "https://example.com"
	oAuthBaseURL = "https://oauth.example.com"

	dbTeamFactory          *dbfakes.FakeTeamFactory
	oauthFactory           *authfakes.FakeProviderFactory
	fakeCSRFTokenGenerator *authfakes.FakeCSRFTokenGenerator
	fakeAuthTokenGenerator *authfakes.FakeAuthTokenGenerator
	fakeTokenReader        *authfakes.FakeTokenReader
	fakeTokenValidator     *authfakes.FakeTokenValidator
	fakeBasicAuthValidator *authfakes.FakeTokenValidator

	peerAddr string
	drain    chan struct{}
	logger   *lagertest.TestLogger

	server *httptest.Server
	client *http.Client
)

var _ = BeforeEach(func() {
	dbTeamFactory = new(dbfakes.FakeTeamFactory)
	oauthFactory = new(authfakes.FakeProviderFactory)

	dbTeam := new(dbfakes.FakeTeam)
	dbTeam.IDReturns(734)
	dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
	dbTeamFactory.GetByIDReturns(dbTeam)

	fakeCSRFTokenGenerator = new(authfakes.FakeCSRFTokenGenerator)
	fakeAuthTokenGenerator = new(authfakes.FakeAuthTokenGenerator)
	fakeTokenReader = new(authfakes.FakeTokenReader)
	fakeTokenValidator = new(authfakes.FakeTokenValidator)
	fakeBasicAuthValidator = new(authfakes.FakeTokenValidator)

	peerAddr = "127.0.0.1:1234"

	logger = lagertest.NewTestLogger("api")
	logger.RegisterSink(lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG))

	config := &skymarshal.DetailedConfig{
		BaseUrl:            externalURL,
		BaseAuthUrl:        oAuthBaseURL,
		Expiration:         24 * time.Hour,
		IsTLSEnabled:       false,
		TeamFactory:        dbTeamFactory,
		Logger:             logger,
		CSRFTokenGenerator: fakeCSRFTokenGenerator,
		AuthTokenGenerator: fakeAuthTokenGenerator,
		OAuthFactory:       oauthFactory,
		OAuthFactoryV1:     oauthFactory,
		TokenReader:        fakeTokenReader,
		TokenValidator:     fakeTokenValidator,
		BasicAuthValidator: fakeBasicAuthValidator,
	}

	handler, err := skymarshal.NewHandlerWithOptions(config)
	Expect(err).NotTo(HaveOccurred())

	server = httptest.NewServer(handler)

	client = &http.Client{
		Transport: &http.Transport{},
	}
})

var _ = AfterEach(func() {
	server.Close()
})

func TestSkymarshal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Skymarshal Suite")
}
