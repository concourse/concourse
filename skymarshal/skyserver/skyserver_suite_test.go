package skyserver_test

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/skymarshal/skyserver"
	"github.com/concourse/concourse/skymarshal/token/tokenfakes"

	"github.com/onsi/gomega/ghttp"
	"golang.org/x/oauth2"
)

var (
	fakeTokenMiddleware    *tokenfakes.FakeMiddleware
	fakeTokenParser        *tokenfakes.FakeParser
	skyServer              *httptest.Server
	dexServer              *ghttp.Server
	signingKey             *rsa.PrivateKey
	config                 *skyserver.SkyConfig
	fakeClaimsCacher       *accessorfakes.FakeAccessTokenFetcher
	fakeAccessTokenFactory *dbfakes.FakeAccessTokenFactory
)

func TestSkyServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sky Server Suite")
}

var _ = BeforeEach(func() {
	var err error

	fakeTokenMiddleware = new(tokenfakes.FakeMiddleware)
	fakeTokenParser = new(tokenfakes.FakeParser)
	fakeClaimsCacher = new(accessorfakes.FakeAccessTokenFetcher)
	fakeAccessTokenFactory = new(dbfakes.FakeAccessTokenFactory)

	dexServer = ghttp.NewTLSServer()

	signingKey, err = rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	endpoint := oauth2.Endpoint{
		AuthURL:   dexServer.URL() + "/auth",
		TokenURL:  dexServer.URL() + "/token",
		AuthStyle: oauth2.AuthStyleInHeader,
	}

	oauthConfig := &oauth2.Config{
		Endpoint:     endpoint,
		ClientID:     "dex-client-id",
		ClientSecret: "dex-client-secret",
		Scopes:       []string{"some-scope"},
	}

	config = &skyserver.SkyConfig{
		Logger:             lagertest.NewTestLogger("sky"),
		TokenMiddleware:    fakeTokenMiddleware,
		TokenParser:        fakeTokenParser,
		OAuthConfig:        oauthConfig,
		HTTPClient:         dexServer.HTTPTestServer.Client(),
		ClaimsCacher:       fakeClaimsCacher,
		AccessTokenFactory: fakeAccessTokenFactory,
	}

	server, err := skyserver.NewSkyServer(config)
	Expect(err).NotTo(HaveOccurred())

	skyServer = httptest.NewUnstartedServer(skyserver.NewSkyHandler(server))
})

var _ = AfterEach(func() {
	skyServer.Close()
	dexServer.Close()
})
